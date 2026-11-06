// Package chksz plays NetEase Cloud Music's songs through ChKSz
// (api.chksz.com), which resolves a song to a stream without a NetEase
// account: NetEase asks the listener to scan a QR code before it streams
// most songs now, and ChKSz does not.
//
// Asking costs: the API takes 20 asks a minute and 400 a day for a key,
// so a Client keeps what it learned, waits out the minute's asks before
// making another, and stops asking once the API refuses it. Its errors
// are the listener's, in Chinese, as the player shows them: the API's
// own are in English.
package chksz

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"netease-music-mygo/internal/netease"
)

const (
	apiBase = "https://api.chksz.com"
	// perMinute is how many asks the API takes a minute for a key.
	perMinute = 20
	// maxWait is the longest a song's load waits for the API's pace:
	// beyond it the listener is told, and the queue stops.
	maxWait = 3 * time.Second
)

// httpClient asks the API, and follows a redirect only while it stays on
// the same host: the key rides in the query, and a redirect elsewhere
// would hand it to whoever answered.
var httpClient = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && req.URL.Host != via[0].URL.Host {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// How long what a client learned is kept: a stream's URL is signed and
// lapses, while a song that isn't there may come back.
const (
	// streamTTL is how long a stream is kept when its URL says nothing of
	// when it lapses.
	streamTTL = 5 * time.Minute
	// streamLife is the longest a stream is kept, however long its URL
	// says it lasts: NetEase signs them for twenty-odd minutes.
	streamLife = 30 * time.Minute
	// lapse is how long before its URL lapses a stream stops being
	// handed out, so that it isn't given out as it goes.
	lapse = 30 * time.Second
	// missingTTL is how long a song the API has no stream for is kept
	// from being asked for again.
	missingTTL = 2 * time.Minute
	// failedTTL is how long any other failure is kept.
	failedTTL = 30 * time.Second
)

// errMissing is a song the API has no stream for: asking again at once
// would only spend another ask to hear the same.
var errMissing = errors.New("这首歌暂时无法播放")

// BlockedError is a refusal every song would meet: a key the API won't
// take, a quota that is spent, or a pace it asked to wait out. The queue
// stops on it rather than skipping through every song to fail them all.
type BlockedError struct{ msg string }

func (e *BlockedError) Error() string { return e.msg }

// Client resolves songs with one key. It remembers what it resolved and
// what failed, asks no faster than the API allows, and stops asking when
// the API refuses the key, spends its quota or asks to wait.
type Client struct {
	key   string
	api   string
	http  *http.Client
	now   func() time.Time
	sleep func(time.Duration)

	mu      sync.Mutex
	known   map[song]answer
	asks    []time.Time // when the last minute's asks went out
	limit   int         // how many the API takes a minute, as it says
	blocked time.Time   // no ask goes out before this
	why     string      // why it stopped asking
}

// song is a stream a client may have resolved: a song at a level.
type song struct {
	id    int64
	level string
}

// answer is what a song resolved to, and until when that holds.
type answer struct {
	st    *netease.Stream
	err   error
	until time.Time
}

// New returns a client that resolves songs with key.
func New(key string) *Client {
	return &Client{
		key:   key,
		api:   apiBase,
		http:  httpClient,
		now:   time.Now,
		sleep: time.Sleep,
		known: map[song]answer{},
		limit: perMinute,
	}
}

// SongURL returns where a song streams, at level (one of netease.Levels,
// or the closest ChKSz has), with the API key, remembering nothing of
// earlier asks: a Client is what keeps them.
func SongURL(id int64, level, key string) (*netease.Stream, error) {
	return New(key).SongURL(id, level)
}

// SongURL returns where a song streams at level, from what it resolved
// before if that still holds, else asks the API: once, no faster than it
// takes, and not at all while it has said to stop.
func (c *Client) SongURL(id int64, level string) (*netease.Stream, error) {
	if c == nil || c.key == "" {
		return nil, errors.New("没有 ChKSz API Key，无法播放")
	}
	want := song{id: id, level: level}
	if st, err, ok := c.remembered(want); ok {
		return st, err
	}
	if err := c.ready(); err != nil {
		return nil, err
	}
	st, err := c.ask(id, level)
	var blocked *BlockedError
	if err != nil && errors.As(err, &blocked) {
		// The block says how long, not this song: it is no answer to keep.
		return nil, err
	}
	c.remember(want, st, err)
	return st, err
}

// remembered returns what it learned of a song before, while that holds.
func (c *Client) remembered(s song) (*netease.Stream, error, bool) {
	c.mu.Lock()
	a, ok := c.known[s]
	c.mu.Unlock()
	if !ok || c.now().After(a.until) {
		return nil, nil, false
	}
	if a.st == nil {
		return nil, a.err, true
	}
	st := *a.st
	return &st, nil, true
}

// remember keeps what a song resolved to, or why it didn't, for a while.
func (c *Client) remember(s song, st *netease.Stream, err error) {
	now := c.now()
	ttl := failedTTL
	switch {
	case err == nil:
		// The stream holds as long as its URL serves, which is how long
		// the song plays without another ask.
		ttl = keepFor(st, now)
	case errors.Is(err, errMissing):
		ttl = missingTTL
	}
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	if len(c.known) > 512 {
		c.sweep()
	}
	c.known[s] = answer{st: st, err: err, until: now.Add(ttl)}
	c.mu.Unlock()
}

// keepFor returns how long a stream resolved at now may be handed out:
// until the moment its URL stops serving, which NetEase stamps in the
// path, and never longer than streamLife. A URL that says nothing of it
// is kept for streamTTL.
func keepFor(st *netease.Stream, now time.Time) time.Duration {
	end, ok := expiresAt(st.URL, now)
	if !ok {
		return streamTTL
	}
	// A URL that lapses sooner than the app can play it is no use to keep:
	// the ask that follows is a fresh one.
	if ttl := min(end.Sub(now)-lapse, streamLife); ttl > 0 {
		return ttl
	}
	return 0
}

// expiresAt returns when a stream's URL stops serving, as NetEase's say:
// the first part of the path is that moment, to the second, in UTC+8,
// as m801.music.126.net/20261105143004/… stops serving at 14:30:04. A URL
// whose first part is no such moment near now says nothing.
func expiresAt(u string, now time.Time) (time.Time, bool) {
	parsed, err := url.Parse(u)
	if err != nil {
		return time.Time{}, false
	}
	first := strings.TrimPrefix(parsed.Path, "/")
	if i := strings.IndexByte(first, '/'); i >= 0 {
		first = first[:i]
	}
	if len(first) != 14 {
		return time.Time{}, false
	}
	end, err := time.ParseInLocation("20060102150405", first, chkszDay)
	if err != nil {
		return time.Time{}, false
	}
	// A moment far from now is a number that only looks like one.
	if d := end.Sub(now); d > 24*time.Hour || d < -6*time.Hour {
		return time.Time{}, false
	}
	return end, true
}

// sweep forgets what no longer holds. The caller holds the lock.
func (c *Client) sweep() {
	now := c.now()
	for s, a := range c.known {
		if now.After(a.until) {
			delete(c.known, s)
		}
	}
}

// ready returns once a song may be asked for, and counts the ask, or why
// none may go out: the key is refused, the quota is spent, or a minute's
// asks are already spent. It waits out a short one, but no longer than
// maxWait: waiting a minute for a song would look like a hang.
func (c *Client) ready() error {
	for {
		c.mu.Lock()
		now := c.now()
		if now.Before(c.blocked) {
			why := c.why
			c.mu.Unlock()
			return &BlockedError{why}
		}
		// The API counts the asks a minute back.
		for len(c.asks) > 0 && now.Sub(c.asks[0]) >= time.Minute {
			c.asks = c.asks[1:]
		}
		limit := c.limit
		if limit <= 0 {
			limit = perMinute
		}
		if len(c.asks) < limit {
			c.asks = append(c.asks, now)
			c.mu.Unlock()
			return nil
		}
		wait := c.asks[0].Add(time.Minute).Sub(now)
		c.mu.Unlock()
		if wait <= 0 {
			continue
		}
		if wait > maxWait {
			secs := int(wait.Seconds() + 0.5)
			return &BlockedError{"ChKSz 请求太频繁，请 " + strconv.Itoa(secs) + " 秒后再试"}
		}
		c.sleep(wait)
	}
}

// ask resolves a song with the API.
func (c *Client) ask(id int64, level string) (*netease.Stream, error) {
	query := url.Values{
		"id":     {strconv.FormatInt(id, 10)},
		"level":  {levelOf(level)},
		"apikey": {c.key},
	}
	res, err := c.get("/api/163_music", query)
	if errors.Is(err, errUnreachable) {
		// A connection that broke as it opened, which the first ask after
		// the app sits idle may: asked once more, which usually gets
		// through, so that a song is not given up on for it.
		res, err = c.get("/api/163_music", query)
	}
	if err != nil {
		return nil, err
	}
	return streamOf(res, level)
}

// levels maps the levels of the app to the ones ChKSz names: it has no
// 192k, so higher asks for the next one up.
var levels = map[string]string{
	"standard": "standard",
	"higher":   "exhigh",
	"exhigh":   "exhigh",
	"lossless": "lossless",
	"hires":    "hires",
}

// levelOf returns the level to ask ChKSz for a track of level.
func levelOf(level string) string {
	if l, ok := levels[level]; ok {
		return l
	}
	return "exhigh"
}

// streamOf is the stream the API answered with, at level.
func streamOf(res *response, level string) (*netease.Stream, error) {
	u := strings.Replace(res.Data.URL, "http://", "https://", 1)
	if u == "" {
		return nil, errMissing
	}
	s := &netease.Stream{
		URL:    u,
		Size:   res.Data.Size,
		Type:   typeOf(u),
		Level:  level,
		Source: netease.SourceChKSz,
	}
	// It says the level it served, which may be below the one asked for;
	// one the app doesn't know is taken for the one asked for, as the
	// cache names its file by it.
	if slices.Contains(netease.Levels, res.Data.Level) {
		s.Level = res.Data.Level
	}
	return s, nil
}

// typeOf names the format of a stream's file.
func typeOf(u string) string {
	if p, err := url.Parse(u); err == nil && strings.EqualFold(path.Ext(p.Path), ".flac") {
		return "flac"
	}
	return "mp3"
}

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		URL   string `json:"url"`
		Level string `json:"level"`
		Size  int64  `json:"size"`
	} `json:"data"`
}

// errUnreachable is the API not answering at all: a connection that broke
// before it was made. Asking once more gets through, usually.
var errUnreachable = errors.New("连不上 ChKSz，请检查网络")

// unreachable returns why a request did not go through: errUnreachable for
// one the API did not take, and the API being slow for one that ran out of
// time, which asking again would only wait out twice.
func unreachable(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("ChKSz 响应太慢，稍后再试")
	}
	return errUnreachable
}

// get gets endpoint with query, and returns what it answered, or the
// listener's reason why it didn't.
func (c *Client) get(endpoint string, query url.Values) (*response, error) {
	req, err := http.NewRequest("GET", c.api+endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", netease.UserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, unreachable(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var r response
	if len(body) > 0 {
		// A refusal may say nothing at all, or say it in HTML: its
		// status is what tells the listener.
		if err := json.Unmarshal(body, &r); err != nil && res.StatusCode == http.StatusOK {
			return nil, errors.New("ChKSz 返回了无法识别的内容")
		}
	}
	c.noteQuota(res.Header)
	if res.StatusCode == http.StatusOK && r.Code == 200 {
		return &r, nil
	}
	return nil, c.refusal(res, r)
}

// refusal says why ChKSz refused, and keeps the asks the next song would
// make for nothing from going out.
func (c *Client) refusal(res *http.Response, r response) error {
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return c.block(endOfDay(c.now()), "ChKSz 的 API Key 无效，请在设置里换一个")
	case http.StatusPaymentRequired:
		return c.block(endOfDay(c.now()), "ChKSz 今日额度已用完，明天再来")
	case http.StatusForbidden:
		if r.Msg == "" {
			return c.block(endOfDay(c.now()), "ChKSz 拒绝了请求")
		}
		return c.block(endOfDay(c.now()), "ChKSz 拒绝了请求："+r.Msg)
	case http.StatusNotFound:
		return errMissing
	case http.StatusTooManyRequests:
		return c.block(c.now().Add(retryAfter(res.Header)), "ChKSz 请求太频繁，稍后再试")
	case http.StatusServiceUnavailable:
		return errors.New("ChKSz 暂时不可用，稍后再试")
	}
	if r.Msg != "" {
		return errors.New("ChKSz 播放失败：" + r.Msg)
	}
	return errors.New("ChKSz 播放失败（HTTP " + strconv.Itoa(res.StatusCode) + "）")
}

// block stops the asks until then, and returns why, as the listener reads
// it.
func (c *Client) block(until time.Time, why string) error {
	c.mu.Lock()
	if until.After(c.blocked) {
		c.blocked, c.why = until, why
	}
	msg := c.why
	c.mu.Unlock()
	return &BlockedError{msg}
}

// noteQuota reads how many asks the day has left, which every answer
// says, and stops asking when there are none. The rate it says it takes
// is the one paced for, in case a key is given more than the default.
func (c *Client) noteQuota(h http.Header) {
	if n, ok := headerInt(h, "X-RateLimit-Limit"); ok && n > 0 {
		c.mu.Lock()
		c.limit = n
		c.mu.Unlock()
	}
	free, freeOK := headerInt(h, "X-Quota-Free-Remaining")
	paid, paidOK := headerInt(h, "X-Quota-Paid-Remaining")
	if !freeOK && !paidOK {
		return
	}
	if free+paid <= 0 {
		c.block(endOfDay(c.now()), "ChKSz 今日额度已用完，明天再来")
	}
}

// headerInt returns the int a header holds, and whether it held one.
func headerInt(h http.Header, name string) (int, bool) {
	v := strings.TrimSpace(h.Get(name))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

// retryAfter is how long the API asked to wait, a minute if it didn't say.
func retryAfter(h http.Header) time.Duration {
	s := strings.TrimSpace(h.Get("Retry-After"))
	if n, err := strconv.Atoi(s); err == nil {
		if n > 0 {
			return time.Duration(n) * time.Second
		}
		return time.Minute
	}
	if t, err := http.ParseTime(s); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return time.Minute
}

// chkszDay is the zone the API's quota day turns over in, UTC+8.
var chkszDay = time.FixedZone("UTC+8", 8*60*60)

// endOfDay is when the day's quota is full again.
func endOfDay(now time.Time) time.Time {
	t := now.In(chkszDay)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, chkszDay).Add(24 * time.Hour)
}
