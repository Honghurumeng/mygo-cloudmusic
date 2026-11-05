package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// A song NetEase keeps for subscribers plays through ChKSz, and is dimmed
// only when there is no key to play it with.
func TestCanPlayThroughChKSz(t *testing.T) {
	// As NetEase marks a VIP song for someone not signed in.
	vip := netease.Track{ID: 1, Privilege: &netease.Privilege{St: -100, Pl: 0}}
	free := netease.Track{ID: 2, Privilege: &netease.Privilege{St: 0, Pl: 320000}}

	a := newTestApp()
	if a.canPlay(&vip) {
		t.Fatal("a VIP song plays with no key")
	}
	if !a.canPlay(&free) {
		t.Fatal("a free song does not play")
	}

	a.chksz = "chksz_test"
	if !a.canPlay(&vip) {
		t.Fatal("a VIP song does not play through ChKSz")
	}
}

// The key typed into the settings is the one the app plays with.
func TestChKSzKeyTyped(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.settingsView, 560, 1500)
	tt.Frame()
	if err := tt.Click("ChKSz API Key"); err != nil {
		t.Fatal(err)
	}
	tt.Type("chksz_abc")
	tt.Frame()
	if a.settings.ChKSzKey != "chksz_abc" {
		t.Fatalf("key %q, want the one typed", a.settings.ChKSzKey)
	}
}
