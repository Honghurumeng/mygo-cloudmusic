package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The key the settings hold is what a client is made with, unless the
// listener turned the source off to play only as NetEase streams.
func TestNewChKSz(t *testing.T) {
	if newChKSz(settings{NetEaseOnly: true, ChKSzKey: "chksz_test"}) != nil {
		t.Fatal("a client is made with the source turned off")
	}
	if newChKSz(settings{ChKSzKey: "chksz_test"}) == nil {
		t.Fatal("no client is made with a key")
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

// A key found beside the binary or in the directory the app runs from is
// kept in its data directory, where it is found however the app starts
// next time: otherwise a build started from another directory would find
// no key at all.
func TestKeepKey(t *testing.T) {
	home, from := t.TempDir(), t.TempDir()
	mygo.App.SetPath(mygo.PathUserData, home)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, "") })

	keepKey(from, "chksz_kept")
	path := filepath.Join(home, keyName)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != "chksz_kept" {
		t.Fatalf("kept %q, want the key found", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the key file is not private: %v %v", info, err)
	}

	// One already kept is the listener's: it stays.
	keepKey(from, "chksz_other")
	if b, err := os.ReadFile(path); err != nil || strings.TrimSpace(string(b)) != "chksz_kept" {
		t.Fatalf("a kept key was written over: %q %v", b, err)
	}
}

// Without a session, NetEase streams nothing: a song it won't play is told
// to be about the missing key rather than VIP, which is not why it did not
// play.
func TestNoSourceWhy(t *testing.T) {
	if err := noSourceWhy(netease.ErrUnavailable, false); err != errNoSource {
		t.Fatalf("%v, want the missing key's reason", err)
	}
	if err := noSourceWhy(netease.ErrUnavailable, true); err != netease.ErrUnavailable {
		t.Fatalf("signed in: %v, want NetEase's own reason", err)
	}
	down := errors.New("连不上网易云")
	if err := noSourceWhy(down, false); err != down {
		t.Fatalf("%v, want the network's reason", err)
	}
}
