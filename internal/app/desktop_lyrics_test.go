package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// The lyrics over the desktop show the line being sung, with its
// translation under it, and as it ends the line after.
func TestDesktopLyricsFollowsTheSong(t *testing.T) {
	first := lyrics.Line{
		At:          10 * time.Second,
		End:         14 * time.Second,
		Text:        "Hello",
		Translation: "你好",
		Words:       lyrics.SpreadWords("Hello", 10*time.Second, 14*time.Second),
	}
	second := lyrics.Line{At: 15 * time.Second, End: 19 * time.Second, Text: "下一句",
		Words: lyrics.SpreadWords("下一句", 15*time.Second, 19*time.Second)}
	a := newTestApp()
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	a.pl.SetLyrics([]lyrics.Line{first, second})
	a.pl.Hold(11 * time.Second)

	tt := ui.NewTester((&desktopLyrics{a: a, w: deskLyricsW}).view, deskLyricsW, deskLyricsH)
	tt.Frame()
	if !tt.HasText("Hello") {
		t.Errorf("the line sung is not shown: %q", tt.Texts())
	}
	if !tt.HasText("你好") {
		t.Errorf("its translation is not shown: %q", tt.Texts())
	}

	// The line after it, once it is sung.
	a.pl.Hold(15 * time.Second)
	tt.Frame()
	if !tt.HasText("下一句") {
		t.Errorf("the line sung is not shown: %q", tt.Texts())
	}
	if tt.HasText("你好") {
		t.Error("the translation of the line before still shows")
	}
}

// Before the first line its text shows, faint, and nothing under it.
func TestDesktopLyricsBeforeTheFirstLine(t *testing.T) {
	lines := []lyrics.Line{{At: 10 * time.Second, End: 14 * time.Second, Text: "第一句"}}
	cur, main, sub, begun := desktopShown(lines, 2*time.Second)
	if cur != -1 || main != "第一句" || begun {
		t.Fatalf("showed %d %q, begun %v", cur, main, begun)
	}
	if sub != "" {
		t.Errorf("under it: %q", sub)
	}
}

// Under the line sung goes its translation; without one, the line after.
func TestDesktopLyricsUnderTheLine(t *testing.T) {
	lines := []lyrics.Line{
		{At: time.Second, End: 4 * time.Second, Text: "一"},
		{At: 5 * time.Second, End: 8 * time.Second, Text: "二", Translation: "two"},
		{At: 9 * time.Second, End: 12 * time.Second, Text: "三"},
	}
	if _, _, sub, _ := desktopShown(lines, 2*time.Second); sub != "二" {
		t.Errorf("under 一: %q, want the line after", sub)
	}
	if _, _, sub, _ := desktopShown(lines, 6*time.Second); sub != "two" {
		t.Errorf("under 二: %q, want its translation", sub)
	}
	if _, _, sub, _ := desktopShown(lines, 10*time.Second); sub != "" {
		t.Errorf("under the last line: %q", sub)
	}
}

// With nothing to sing, the lyrics say what plays: a song without lyrics,
// an episode, or nothing at all.
func TestDesktopLyricsWithoutLyrics(t *testing.T) {
	a := newTestApp()
	d := &desktopLyrics{a: a, w: deskLyricsW}

	tt := ui.NewTester(d.view, deskLyricsW, deskLyricsH)
	tt.Frame()
	if !tt.HasText("未在播放") {
		t.Errorf("without a song: %q", tt.Texts())
	}

	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	a.pl.Hold(time.Second)
	tt.Frame()
	if !tt.HasText("正在加载歌词…") {
		t.Errorf("while lyrics load: %q", tt.Texts())
	}

	a.pl.SetLyrics(nil)
	tt.Frame()
	if !tt.HasText("纯音乐，请欣赏") {
		t.Errorf("a song without lyrics: %q", tt.Texts())
	}
}

// An episode has no lyrics to sing: it shows what it is.
func TestDesktopLyricsForAnEpisode(t *testing.T) {
	a := newTestApp()
	a.pl.Cue([]netease.Track{{
		ID: 8, Name: "第一期", PodcastID: 9,
		Artists:  []netease.Artist{{Name: "某播客"}},
		Duration: 60000,
	}})
	a.pl.SetLyrics(nil)
	d := &desktopLyrics{a: a, w: deskLyricsW}
	tt := ui.NewTester(d.view, deskLyricsW, deskLyricsH)
	tt.Frame()
	if !tt.HasText("第一期") || !tt.HasText("某播客 · 播客") {
		t.Errorf("an episode: %q", tt.Texts())
	}
}

// A line too long for the window is set smaller, until it fits.
func TestDesktopLyricsFitsALongLine(t *testing.T) {
	long := strings.Repeat("很长的一句歌词", 6)
	a := newTestApp()
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	a.pl.SetLyrics([]lyrics.Line{{At: time.Second, End: 30 * time.Second, Text: long}})
	a.pl.Hold(10 * time.Second)
	d := &desktopLyrics{a: a, w: deskLyricsW}
	tt := ui.NewTester(d.view, deskLyricsW, deskLyricsH)
	tt.Frame()
	if !tt.HasText(long) {
		t.Fatalf("the line is not shown: %q", tt.Texts())
	}
	if _, ok := tt.Find(long); !ok {
		t.Fatal("no line laid out")
	}
	if r, _ := tt.Find(long); r.W > deskLyricsW {
		t.Errorf("the line is %v wide, past the window's %d", r.W, deskLyricsW)
	}
}

// The lyrics start at the bottom middle of the work area, above the Dock.
func TestDesktopLyricsSpot(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 850}
	x, y := deskLyricsSpot(work, deskLyricsW, deskLyricsH)
	if x != (1440-deskLyricsW)/2 {
		t.Errorf("x = %d, want it centered", x)
	}
	if want := 25 + 850 - deskLyricsH - deskLyricsEdge; y != want {
		t.Errorf("y = %d, want %d", y, want)
	}
}

// Unlocking the lyrics is kept: the default is locked, so a setting left
// out of the file would lock them again at the next launch.
func TestDesktopLyricsKeepTheirLock(t *testing.T) {
	s := defaultSettings()
	s.DesktopLyrics = true
	s.DesktopLocked = false
	s.DesktopX, s.DesktopY = 10, 20
	s.path = filepath.Join(t.TempDir(), "settings.json")
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	got := defaultSettings()
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.DesktopLocked {
		t.Errorf("unlocking was lost: %s", b)
	}
	if got.DesktopX != 10 || got.DesktopY != 20 {
		t.Errorf("the place was lost: %d,%d", got.DesktopX, got.DesktopY)
	}
}

// Unlocked, the window makes room for the bar above the lyrics; locked,
// it is as tall as the lyrics. Either way its foot stays where it was.
func TestDesktopLyricsRoomForTheBar(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	d := &desktopLyrics{a: a}
	if got := d.height(); got != deskCardH {
		t.Errorf("locked, the window is %d tall, want %d", got, deskCardH)
	}
	a.settings.DesktopLocked = false
	if got := d.height(); got != deskLyricsH {
		t.Errorf("unlocked, the window is %d tall, want %d", got, deskLyricsH)
	}

	work := mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 850}
	_, locked := deskLyricsSpot(work, deskLyricsW, deskCardH)
	_, unlocked := deskLyricsSpot(work, deskLyricsW, deskLyricsH)
	if locked+deskCardH != unlocked+deskLyricsH {
		t.Errorf("the foot moved from %d to %d", locked+deskCardH, unlocked+deskLyricsH)
	}
}

// The player bar's first button turns the lyrics over the desktop on and
// off.
func TestDesktopLyricsToggledOnTheBar(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()

	if err := tt.Click("打开桌面歌词"); err != nil {
		t.Fatal(err)
	}
	if !a.settings.DesktopLyrics {
		t.Fatal("the button did not turn them on")
	}
	tt.Frame()
	if err := tt.Click("关闭桌面歌词"); err != nil {
		t.Fatal(err)
	}
	if a.settings.DesktopLyrics {
		t.Error("still on after the second click")
	}
}
