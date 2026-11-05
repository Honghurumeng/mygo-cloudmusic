package chksz

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// clock is a time that moves only when a test says so, and a sleep that
// moves it: the client's waits cost nothing to test.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) sleep(d time.Duration)   { c.t = c.t.Add(d) }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
func newClock() *clock                   { return &clock{t: time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC)} }

// blocked reports whether err stops the queue rather than failing a song.
func blocked(err error) bool {
	var b *BlockedError
	return errors.As(err, &b)
}

// testClient is a client that asks srv, with a clock a test moves.
func testClient(t *testing.T, srv *httptest.Server, c *clock) *Client {
	t.Helper()
	client := New("chksz_test")
	client.api, client.http = srv.URL, srv.Client()
	client.now, client.sleep = c.now, c.sleep
	return client
}

// anAnswer is what the API says of a song that streams.
func anAnswer(w http.ResponseWriter) {
	anAnswerOf(w, "https://m801.music.126.net/2026/a/b.flac?vuutv=x")
}

// anAnswerOf is what the API says of a song that streams from u.
func anAnswerOf(w http.ResponseWriter, u string) {
	json.NewEncoder(w).Encode(map[string]any{
		"code": 200,
		"data": map[string]any{
			"url":   u,
			"level": "lossless",
			"size":  12345,
		},
	})
}

// A song asked for twice is resolved once: the second time is free.
func TestClientRemembersASong(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		anAnswer(w)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	for i := 0; i < 3; i++ {
		st, err := c.SongURL(1, "lossless")
		if err != nil {
			t.Fatalf("ask %d: %v", i, err)
		}
		if st.Type != "flac" || st.Level != "lossless" || st.Size != 12345 {
			t.Fatalf("ask %d: stream %+v", i, st)
		}
	}
	if asks != 1 {
		t.Fatalf("%d asks for one song, want 1", asks)
	}
}

// A stream is kept until the moment its URL stops serving, as the path
// of the URL says: playing the song again before then costs no ask, and
// after it costs one.
func TestStreamIsKeptUntilItsURLapses(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		// NetEase's URLs serve for twenty-odd minutes, and carry the
		// moment they stop in their path.
		end := clock.t.In(chkszDay).Add(20 * time.Minute).Format("20060102150405")
		anAnswerOf(w, "https://m801.music.126.net/"+end+"/x/y.mp3?vuutv=z")
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	if _, err := c.SongURL(1, "exhigh"); err != nil {
		t.Fatal(err)
	}
	clock.advance(19 * time.Minute)
	if _, err := c.SongURL(1, "exhigh"); err != nil {
		t.Fatal(err)
	}
	if asks != 1 {
		t.Fatalf("%d asks while the URL served, want 1", asks)
	}

	// It lapsed: the next play is a new ask, not an address that won't
	// serve.
	clock.advance(2 * time.Minute)
	if _, err := c.SongURL(1, "exhigh"); err != nil {
		t.Fatal(err)
	}
	if asks != 2 {
		t.Fatalf("%d asks after the URL lapsed, want 2", asks)
	}
}

// The moment a URL stops serving is read off its path, in UTC+8; a number
// that only looks like one is taken to say nothing.
func TestExpiresAt(t *testing.T) {
	now := time.Date(2026, 2, 3, 9, 0, 0, 0, time.UTC) // 17:00 in UTC+8
	at := func(h, m int) time.Time {
		return time.Date(2026, 2, 3, h, m, 0, 0, time.UTC)
	}
	cases := []struct {
		url  string
		want time.Time
		ok   bool
	}{
		{"https://m801.music.126.net/20260203172000/x/y.flac", at(9, 20), true},
		{"https://m801.music.126.net/20260203200000/x/y.mp3?vuutv=z", at(12, 0), true},
		{"https://m801.music.126.net/2026/x/y.flac", time.Time{}, false},
		{"https://m801.music.126.net/x/20260203172000/y.flac", time.Time{}, false},
		{"https://m801.music.126.net/20200203172000/x/y.flac", time.Time{}, false},
		{"", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := expiresAt(c.url, now)
		if ok != c.ok || (ok && !got.Equal(c.want)) {
			t.Errorf("%s: %v %v, want %v %v", c.url, got, ok, c.want, c.ok)
		}
	}
}

// A song that isn't there is not asked for again at once: hearing the
// same answer costs another ask.
func TestClientRemembersAMissingSong(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	for i := 0; i < 2; i++ {
		if _, err := c.SongURL(2, "exhigh"); err == nil {
			t.Fatal("a song that isn't there played")
		}
	}
	if asks != 1 {
		t.Fatalf("%d asks for a missing song, want 1", asks)
	}
}

// A minute takes twenty asks, not one more: the twenty-first waits for
// the first to be a minute old, and one further off than a song may wait
// stops the queue instead.
func TestClientAsksNoFasterThanTheAPITakes(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		anAnswer(w)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	for i := 0; i < perMinute; i++ {
		if _, err := c.SongURL(int64(i+1), "exhigh"); err != nil {
			t.Fatalf("ask %d: %v", i, err)
		}
	}
	if asks != perMinute {
		t.Fatalf("%d asks, want %d", asks, perMinute)
	}

	// A burst: the minute is spent, and the next song would wait it out.
	if _, err := c.SongURL(99, "exhigh"); err == nil {
		t.Fatal("the twenty-first ask went out at once")
	} else if !blocked(err) {
		t.Fatalf("err %v, want a blocked one", err)
	}
	if asks != perMinute {
		t.Fatalf("%d asks, want no more than %d", asks, perMinute)
	}

	// A wait of a couple of seconds is waited out, then the ask goes.
	clock.advance(58 * time.Second)
	if _, err := c.SongURL(100, "exhigh"); err != nil {
		t.Fatalf("the ask after the wait: %v", err)
	}
	if asks != perMinute+1 {
		t.Fatalf("%d asks, want %d", asks, perMinute+1)
	}
}

// The API saying the pace is too fast stops the asks for as long as it
// says, and no longer.
func TestClientWaitsOutTooManyRequests(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		if asks == 1 {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		anAnswer(w)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	if _, err := c.SongURL(1, "exhigh"); err == nil {
		t.Fatal("a rate limit played a song")
	}
	if _, err := c.SongURL(2, "exhigh"); err == nil {
		t.Fatal("an ask went out while the API asked to wait")
	} else if !blocked(err) {
		t.Fatalf("err %v, want a blocked one", err)
	}
	if asks != 1 {
		t.Fatalf("%d asks, want 1", asks)
	}

	clock.advance(31 * time.Second)
	if _, err := c.SongURL(3, "exhigh"); err != nil {
		t.Fatalf("the ask after the wait: %v", err)
	}
	if asks != 2 {
		t.Fatalf("%d asks, want 2", asks)
	}
}

// The quota headers say when the day's asks are gone: no more go out,
// and the listener is told why.
func TestClientStopsWhenTheQuotaIsSpent(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		w.Header().Set("X-Quota-Free-Remaining", "0")
		w.Header().Set("X-Quota-Paid-Remaining", "0")
		anAnswer(w)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	if _, err := c.SongURL(1, "exhigh"); err != nil {
		t.Fatalf("the ask that spent the quota: %v", err)
	}
	if _, err := c.SongURL(2, "exhigh"); err == nil {
		t.Fatal("an ask went out with the quota spent")
	} else if !blocked(err) {
		t.Fatalf("err %v, want a blocked one", err)
	}
	if asks != 1 {
		t.Fatalf("%d asks, want 1", asks)
	}

	// A new day fills it again.
	clock.advance(24 * time.Hour)
	if _, err := c.SongURL(3, "exhigh"); err != nil {
		t.Fatalf("the ask of the next day: %v", err)
	}
}

// A key the API won't take stops the asks rather than ask again for every
// song.
func TestClientStopsOnARefusedKey(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	if _, err := c.SongURL(1, "exhigh"); err == nil {
		t.Fatal("a refused key played a song")
	}
	if _, err := c.SongURL(2, "exhigh"); err == nil {
		t.Fatal("an ask went out with the key refused")
	}
	if asks != 1 {
		t.Fatalf("%d asks, want 1", asks)
	}
}

// The key is in the query: a redirect off the API's host is not followed,
// so it goes nowhere else.
func TestRedirectStaysOnHost(t *testing.T) {
	from := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.chksz.com"}}
	same := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.chksz.com"}}
	elsewhere := &http.Request{URL: &url.URL{Scheme: "https", Host: "music.163.com"}}
	if err := httpClient.CheckRedirect(same, []*http.Request{from}); err != nil {
		t.Fatalf("a redirect on the same host: %v", err)
	}
	if err := httpClient.CheckRedirect(elsewhere, []*http.Request{from}); err != http.ErrUseLastResponse {
		t.Fatalf("a redirect elsewhere: %v, want it left unfollowed", err)
	}
}

// A connection that breaks as a song is asked for is asked again: the
// first ask after the app sits idle may lose it, and a song given up on
// for that is one the queue played another in place of.
func TestClientAsksAgainWhenTheConnectionBreaks(t *testing.T) {
	clock := newClock()
	var asks int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks++
		if asks == 1 {
			panic(http.ErrAbortHandler)
		}
		anAnswer(w)
	}))
	defer srv.Close()
	c := testClient(t, srv, clock)

	st, err := c.SongURL(1, "lossless")
	if err != nil {
		t.Fatalf("asked once more: %v", err)
	}
	if asks != 2 {
		t.Fatalf("%d asks, want the song asked for again", asks)
	}
	if st.URL == "" {
		t.Errorf("resolved %+v", st)
	}
}
