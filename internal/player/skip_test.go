package player

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

// picks plays songs from a server that serves the test MP3, but song 2,
// which NetEase has nothing to stream for.
func picks(t *testing.T) (*Player, chan func()) {
	t.Helper()
	b, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	t.Cleanup(srv.Close)
	lyrics := lyricsOf
	lyricsOf = func(int64) (string, string, string, error) { return "", "", "", nil }
	t.Cleanup(func() { lyricsOf = lyrics })

	ch := make(chan func(), 64)
	p := New(func(fn func()) { ch <- fn })
	p.SetVolume(0)
	p.Resolve = func(id int64, level string) (*netease.Stream, error) {
		if id == 2 {
			return nil, netease.ErrUnavailable
		}
		return &netease.Stream{URL: srv.URL, Size: int64(len(b)), Type: "mp3", Level: "exhigh"}, nil
	}
	t.Cleanup(func() {
		if p.out != nil {
			p.out.Close()
		}
	})
	return p, ch
}

// settle runs what the player posts until nothing loads, as the main
// thread would.
func settle(t *testing.T, p *Player, ch chan func()) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for p.Loading() || len(ch) > 0 {
		select {
		case fn := <-ch:
			fn()
		case <-timeout:
			t.Fatal("the queue did not settle")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// queue is three songs, the second of which cannot play.
func queue() []netease.Track {
	return []netease.Track{
		{ID: 1, Name: "一", Duration: 600000},
		{ID: 2, Name: "二", Duration: 600000},
		{ID: 3, Name: "三", Duration: 600000},
	}
}

// A song the listener picked that cannot play stays current, with the
// reason, rather than the queue going on to a song further down: a click
// played a song the listener did not ask to hear.
func TestPickedSongStaysWhenItCannotPlay(t *testing.T) {
	p, ch := picks(t)
	p.PlaySong(queue(), 1)
	settle(t, p, ch)

	if cur := p.Current(); cur == nil || cur.ID != 2 {
		t.Fatalf("playing %v, want the song picked, 二", cur)
	}
	if p.Playing() {
		t.Error("it says it plays, with nothing playing")
	}
	if p.Err() == "" {
		t.Error("it says nothing of why the song does not play")
	}
}

// The queue still goes on past a song that cannot play as it reaches one,
// which the listener did not pick.
func TestQueueGoesOnPastASongThatCannotPlay(t *testing.T) {
	p, ch := picks(t)
	p.PlaySong(queue(), 0)
	settle(t, p, ch)
	if cur := p.Current(); cur == nil || cur.ID != 1 {
		t.Fatalf("playing %v, want the song picked, 一", cur)
	}

	p.Next()
	settle(t, p, ch)

	if cur := p.Current(); cur == nil || cur.ID != 3 {
		t.Fatalf("playing %v, want 三, past the song that cannot play", cur)
	}
	if !p.Playing() {
		t.Error("it does not play the song it went on to")
	}
	if p.Err() != "" {
		t.Errorf("it says %q, with a song playing", p.Err())
	}
}
