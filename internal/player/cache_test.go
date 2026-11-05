package player

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

// The cache lists what it keeps with what each song is, the one played last
// first, and dropping one takes its audio and what is kept beside it.
func TestCacheSongs(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1<<30)
	store := func(id int64, name string, n int) {
		st := &netease.Stream{Level: "exhigh", Type: "mp3"}
		f := c.part(st)
		f.Write(make([]byte, n))
		f.Close()
		tr := netease.Track{
			ID: id, Name: name, Duration: 200000,
			Artists: []netease.Artist{{Name: "歌手"}},
			Album:   netease.Album{Name: "专辑", PicURL: "http://x/cover.jpg"},
		}
		if _, ok := c.keep(tr, st, f.Name(), int64(n)); !ok {
			t.Fatalf("did not keep %q", name)
		}
	}
	store(1, "宝船", 100)
	store(2, "夜曲", 200)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "2-exhigh.mp3"), old, old); err != nil {
		t.Fatal(err)
	}

	songs := c.Songs()
	if len(songs) != 2 {
		t.Fatalf("listed %d songs", len(songs))
	}
	if got := songs[0]; got.ID != 1 || got.Name != "宝船" || got.Artists != "歌手" ||
		got.Album != "专辑" || got.Cover != "http://x/cover.jpg" || got.Size != 100 {
		t.Errorf("first: %+v", got)
	}
	if songs[0].Duration != 200*time.Second {
		t.Errorf("duration %v", songs[0].Duration)
	}
	if songs[1].ID != 2 {
		t.Errorf("second: %+v", songs[1])
	}

	c.Remove(1, "exhigh")
	for _, name := range []string{"1-exhigh.mp3", "1-exhigh.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s still kept", name)
		}
	}
	if got := c.Songs(); len(got) != 1 || got[0].ID != 2 {
		t.Errorf("after removing: %+v", got)
	}
}

// A song kept before the cache kept what a song is lists without it, and
// takes it once NetEase says, while its stream stays as it was.
func TestCacheFillsOlderSongs(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir, 1<<30)
	os.WriteFile(filepath.Join(dir, "7-exhigh.mp3"), make([]byte, 50), 0o600)
	os.WriteFile(filepath.Join(dir, "7-exhigh.json"),
		[]byte(`{"size":50,"type":"mp3","level":"exhigh","gain":-3}`), 0o600)

	songs := c.Songs()
	if len(songs) != 1 || songs[0].Name != "" {
		t.Fatalf("listed %+v", songs)
	}
	// It still plays without the network, as it did before the cache kept
	// what a song is.
	if _, st, ok := c.lookup(7, "exhigh"); !ok || st.Level != "exhigh" || st.Size != 50 {
		t.Fatalf("playback: %v %+v", ok, st)
	}
	tr := netease.Track{ID: 7, Name: "宝船", Duration: 200000, Artists: []netease.Artist{{Name: "DOUDOU"}}}
	if !c.SetInfo(7, "exhigh", tr) {
		t.Fatal("did not write what the song is")
	}
	if got := c.Songs()[0]; got.Name != "宝船" || got.Artists != "DOUDOU" {
		t.Errorf("still %+v", got)
	}
	if _, st, ok := c.lookup(7, "exhigh"); !ok || st.Level != "exhigh" || st.Size != 50 || st.Gain != -3 {
		t.Errorf("stream after writing: %v %+v", ok, st)
	}
}
