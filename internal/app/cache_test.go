package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/player"
)

// The settings list the songs the cache keeps, dropping one takes it out of
// the list and its files out of the cache, and back returns to the settings.
func TestCachedSongsList(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	dir := t.TempDir()
	for i, name := range []string{"宝船", "夜曲"} {
		id := int64(i + 1)
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d-exhigh.mp3", id)), make([]byte, 100), 0o600)
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d-exhigh.json", id)),
			[]byte(fmt.Sprintf(`{"size":100,"type":"mp3","level":"exhigh","name":%q,"artists":"歌手","duration":200000}`, name)), 0o600)
	}
	a.pl.Cache = player.NewCache(dir, 1<<30)
	a.cacheUsed = 200
	a.cacheSongs = a.pl.Cache.Songs()
	a.cacheOpen = true

	tt := ui.NewTester(a.settingsView, 560, 720)
	tt.Frame()
	for _, s := range []string{"宝船", "夜曲"} {
		if !tt.HasText(s) {
			t.Fatalf("missing %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click("删除 宝船"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("宝船") {
		t.Errorf("still listed: %q", tt.Texts())
	}
	for start := time.Now(); time.Since(start) < time.Second; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(dir, "1-exhigh.mp3")); err != nil {
			break
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "1-exhigh.mp3")); !os.IsNotExist(err) {
		t.Error("its audio stays in the cache")
	}
	if n := a.cacheUsed; n != 100 {
		t.Errorf("takes %d after dropping one of 200", n)
	}

	if err := tt.Click("返回设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.cacheOpen || !tt.HasText("缓存明细") {
		t.Errorf("back did not return to the settings: %q", tt.Texts())
	}
}
