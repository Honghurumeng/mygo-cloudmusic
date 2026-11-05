package app

import (
	"errors"
	"os"
	"testing"

	"netease-music-mygo/internal/chksz"
	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// NetEase goes first, signed in or not: the songs it streams to nobody in
// particular play as it gives them, and spend no ChKSz ask, which is why
// they play with a key the API would refuse. What it keeps for
// subscribers falls to ChKSz, and fails when the key is refused.
//
// The sample is the top of a chart, which holds both kinds on most days; a
// day that doesn't says so rather than passing quietly.
func TestSongSourcesLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.chksz = chksz.New("chksz_refused_for_sure")

	pl, err := netease.PlaylistDetail(3778678)
	if err != nil {
		t.Fatal(err)
	}
	free, kept := 0, 0
	for i := 0; i < len(pl.Tracks) && i < 20; i++ {
		tr := &pl.Tracks[i]
		_, err := a.streamURL(tr.ID, "exhigh")
		var stop *player.StopError
		switch {
		case err == nil:
			free++
			t.Logf("%q played without a ChKSz ask", tr.Name)
		case errors.As(err, &stop):
			kept++
			t.Logf("%q was left to ChKSz: %v", tr.Name, err)
		}
	}
	if free == 0 || kept == 0 {
		t.Skipf("the sample held %d free and %d VIP songs: nothing to compare today", free, kept)
	}
	t.Logf("%d songs played by NetEase, %d left to ChKSz", free, kept)
}
