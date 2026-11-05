package chksz

import (
	"os"
	"testing"

	"netease-music-mygo/internal/netease"
)

// Where a song of the doc's example streams at each level, through a key
// in CHKSZ_API_KEY.
func TestSongURLLive(t *testing.T) {
	key := os.Getenv("CHKSZ_API_KEY")
	if os.Getenv("LIVE") == "" || key == "" {
		t.Skip()
	}
	for _, level := range netease.Levels {
		s, err := SongURL(2034742057, level, key)
		if err != nil {
			t.Fatalf("%s: %v", level, err)
		}
		t.Logf("%s: %s, %d bytes, level %s", level, s.Type, s.Size, s.Level)
	}
}

// A song that isn't there says so, in the listener's words.
func TestSongURLMissingLive(t *testing.T) {
	key := os.Getenv("CHKSZ_API_KEY")
	if os.Getenv("LIVE") == "" || key == "" {
		t.Skip()
	}
	if _, err := SongURL(999999999999, "exhigh", key); err == nil {
		t.Fatal("a song that isn't there played")
	} else {
		t.Log(err)
	}
}
