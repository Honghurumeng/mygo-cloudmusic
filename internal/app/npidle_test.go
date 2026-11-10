package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// openNowPlaying shows Now Playing on a song, at rest, with the pointer out
// of the window.
func openNowPlaying(t *testing.T, a *app) *ui.Tester {
	t.Helper()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 1, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(nil)
	tt := ui.NewTester(a.view, 1200, 800)
	a.nowPlaying = true
	tt.Frame()
	tt.Advance(npSettle + time.Second)
	tt.Frame()
	return tt
}

// A pointer that stays still in Now Playing for two seconds takes the chrome
// and itself away, and a move brings both back.
func TestNowPlayingStillsItsChromeAndThePointer(t *testing.T) {
	a := newTestApp()
	tt := openNowPlaying(t, a)
	var hidden []bool
	hidePointer = func(h bool) { hidden = append(hidden, h) }
	defer func() { hidePointer = nativeHidePointer }()

	tt.Move(600, 400)
	tt.Frame()
	if a.pointerHidden {
		t.Fatal("the pointer is hidden while it moves")
	}
	tt.Advance(npIdle + 100*time.Millisecond)
	tt.Frame()
	if !a.pointerHidden {
		t.Fatal("the pointer is still there after standing still for two seconds")
	}
	if len(hidden) == 0 || !hidden[len(hidden)-1] {
		t.Errorf("the pointer was not hidden, only %v", hidden)
	}
	tt.Move(600, 430)
	tt.Frame()
	if a.pointerHidden {
		t.Error("the pointer stayed hidden after it moved")
	}
}
