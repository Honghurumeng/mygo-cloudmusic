// Package chksz plays NetEase Cloud Music's songs through ChKSz
// (api.chksz.com), which resolves a song to a stream without a NetEase
// account: NetEase asks the listener to scan a QR code before it streams
// most songs now, and ChKSz does not.
//
// Its errors are the listener's, in Chinese, as the player shows them:
// the API's own are in English.
package chksz

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"netease-music-mygo/internal/netease"
)

const apiBase = "https://api.chksz.com"

var httpClient = &http.Client{Timeout: 20 * time.Second}

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

// SongURL returns where a song streams, at level (one of netease.Levels,
// or the closest ChKSz has), with the API key.
func SongURL(id int64, level, key string) (*netease.Stream, error) {
	if key == "" {
		return nil, errors.New("没有 ChKSz API Key，无法播放")
	}
	res, err := call("/api/163_music", url.Values{
		"id":     {strconv.FormatInt(id, 10)},
		"level":  {levelOf(level)},
		"apikey": {key},
	})
	if err != nil {
		return nil, err
	}
	u := strings.Replace(res.Data.URL, "http://", "https://", 1)
	if u == "" {
		return nil, errors.New("这首歌暂时无法播放")
	}
	s := &netease.Stream{
		URL:   u,
		Size:  res.Data.Size,
		Type:  typeOf(u),
		Level: level,
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

// call gets endpoint with query, and returns what it answered, or the
// listener's reason why it didn't.
func call(endpoint string, query url.Values) (*response, error) {
	req, err := http.NewRequest("GET", apiBase+endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", netease.UserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.New("连不上 ChKSz，请检查网络")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, errors.New("ChKSz 返回了无法识别的内容")
	}
	if res.StatusCode == http.StatusOK && r.Code == 200 {
		return &r, nil
	}
	return nil, statusError(res.StatusCode, r)
}

// statusError says why ChKSz refused, as the listener reads it.
func statusError(status int, r response) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.New("ChKSz 的 API Key 无效，请在设置里换一个")
	case http.StatusPaymentRequired:
		return errors.New("ChKSz 今日额度已用完，明天再来")
	case http.StatusForbidden:
		if r.Msg == "" {
			return errors.New("ChKSz 拒绝了请求")
		}
		return errors.New("ChKSz 拒绝了请求：" + r.Msg)
	case http.StatusNotFound:
		return errors.New("这首歌暂时无法播放")
	case http.StatusTooManyRequests:
		return errors.New("ChKSz 请求太频繁，稍后再试")
	case http.StatusServiceUnavailable:
		return errors.New("ChKSz 暂时不可用，稍后再试")
	}
	if r.Msg != "" {
		return errors.New("ChKSz 播放失败：" + r.Msg)
	}
	return errors.New("ChKSz 播放失败（HTTP " + strconv.Itoa(status) + "）")
}
