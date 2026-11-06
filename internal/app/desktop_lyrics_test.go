package app

import (
	"encoding/json"
	"math"
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

	d := &desktopLyrics{a: a}
	tt := ui.NewTester(d.view, d.width(), d.height())
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
	d := &desktopLyrics{a: a}

	tt := ui.NewTester(d.view, d.width(), d.height())
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
	d := &desktopLyrics{a: a}
	tt := ui.NewTester(d.view, d.width(), d.height())
	tt.Frame()
	if !tt.HasText("第一期") || !tt.HasText("某播客 · 播客") {
		t.Errorf("an episode: %q", tt.Texts())
	}
}

// A line wider than the card is kept whole and moves along as it is sung,
// where one that fits stands still.
func TestDesktopLyricsScrollsALongLine(t *testing.T) {
	moves := func(text string, width int) bool {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.DesktopW = width
		l := lyrics.Line{At: time.Second, End: 31 * time.Second, Text: text}
		d := &desktopLyrics{a: a}
		got := false
		tt := ui.NewTester(func(c *ui.Context) {
			got = d.lyric(c, white, deskTypeSize(deskHDefault), &l, 2*time.Second, deskWordDim)
		}, d.width(), d.height())
		tt.Frame()
		return got
	}
	if moves("一句歌词", deskWDefault) {
		t.Error("a line that fits moves")
	}
	if !moves(strings.Repeat("很长的一句歌词", 8), deskWDefault) {
		t.Error("a line too long for the card stands still")
	}
	// The room the card leaves the line is its width, less the padding.
	if moves("一句完整但不算太长的歌词", deskWMax) {
		t.Error("a line that fits a wide card moves")
	}
	if !moves("一句完整但不算太长的歌词", deskWMin) {
		t.Error("the same line fits a card half its width")
	}
}

// A line too long for the card has moved all of what it hides by the time
// it is sung: none at its start, all of it at its end.
func TestDesktopLyricsScrollProgress(t *testing.T) {
	l := lyrics.Line{At: 10 * time.Second, End: 20 * time.Second}
	for _, c := range []struct {
		pos  time.Duration
		want float32
	}{
		{5 * time.Second, 0},
		{10 * time.Second, 0},
		{15 * time.Second, 0.5},
		{20 * time.Second, 1},
		{30 * time.Second, 1},
	} {
		if got := deskProgress(&l, c.pos); math.Abs(float64(got-c.want)) > 1e-6 {
			t.Errorf("at %v: %v, want %v", c.pos, got, c.want)
		}
	}
}

// The lyrics start at the bottom middle of the work area, above the Dock.
func TestDesktopLyricsSpot(t *testing.T) {
	h := deskHDefault
	work := mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 850}
	x, y := deskLyricsSpot(work, deskWDefault, h)
	if x != (1440-deskWDefault)/2 {
		t.Errorf("x = %d, want it centered", x)
	}
	if want := 25 + 850 - h - deskLyricsEdge; y != want {
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

// The card is as big as the listener set, and the height they set decides
// the size of the text: at the least and at the most, the line and the
// lock in the corner stay whole inside it.
func TestDesktopLyricsCard(t *testing.T) {
	if deskTypeSize(deskHMax) <= deskTypeSize(deskHMin) {
		t.Errorf("the text is %v at the least height and %v at the most",
			deskTypeSize(deskHMin), deskTypeSize(deskHMax))
	}
	measure := func(h int) (line, lock ui.Rect, winW, winH int) {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.DesktopH = h
		a.settings.DesktopW = deskWMin
		a.settings.DesktopLocked = false
		a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
		a.pl.SetLyrics([]lyrics.Line{{At: time.Second, End: 30 * time.Second, Text: "一句歌词"}})
		a.pl.Hold(10 * time.Second)
		d := &desktopLyrics{a: a}
		winW, winH = d.width(), d.height()
		tt := ui.NewTester(d.view, winW, winH)
		tt.Frame()
		var ok bool
		if line, ok = tt.Find("一句歌词"); !ok {
			t.Fatal("the line is not shown")
		}
		if lock, ok = tt.Find("锁定桌面歌词"); !ok {
			t.Fatal("the lock is not in the card")
		}
		return line, lock, winW, winH
	}
	small, smallLock, _, smallWin := measure(deskHMin)
	big, bigLock, _, bigWin := measure(deskHMax)
	if big.H <= small.H {
		t.Errorf("the line is %v tall at %d, %v at %d", small.H, deskHMin, big.H, deskHMax)
	}
	if bigWin <= smallWin {
		t.Errorf("the card is %d tall at %d, %d at %d", smallWin, deskHMin, bigWin, deskHMax)
	}
	// The lock sits 3 points under the card's top: it, and the line,
	// show whole only if the card fits the window.
	for _, m := range []struct {
		h          int
		line, lock ui.Rect
		win        int
	}{{deskHMin, small, smallLock, smallWin}, {deskHMax, big, bigLock, bigWin}} {
		if m.lock.Y < 3 || m.lock.Y+m.lock.H > float32(m.win) {
			t.Errorf("at %d the lock is at %v in a card %d tall", m.h, m.lock, m.win)
		}
		if m.line.Y < 0 || m.line.Y+m.line.H > float32(m.win) {
			t.Errorf("at %d the line is at %v in a card %d tall", m.h, m.line, m.win)
		}
	}
}

// A line too long for the card runs from its head, flush with the card's
// padding, to its tail, which is too. It stands still while its first third
// is sung, then moves to its tail, which shows whole for the last third.
func TestDesktopLyricsScrollKeepsItsEnds(t *testing.T) {
	const room = 708
	width := float32(900)

	if shift, moving := deskScroll(float32(room-40), room, 0.5); moving || shift != 0 {
		t.Errorf("a line that fits: %v, moving %v", shift, moving)
	}

	shift, moving := deskScroll(width, room, 0)
	if !moving {
		t.Fatal("a line too long for the card stands still")
	}
	if shift != 0 {
		t.Errorf("at its head the line is at %v, want the card's edge", shift)
	}
	// At its tail the line has gone as far as it may: its last word ends
	// at the card's other edge, with nothing left blank.
	if shift, _ = deskScroll(width, room, 1); shift != room-width {
		t.Errorf("at its tail the line is at %v, want %v", shift, room-width)
	}

	// Sung less than half way, it has not moved at all; come to its tail,
	// it rests there until the line is over.
	for _, c := range []struct {
		p    float32
		want float32
	}{
		{0, 0},
		{0.25, 0},
		{deskHeadHold, 0},
		{1 - deskTailHold, 1},
		{1, 1},
	} {
		if got := deskHeld(c.p); math.Abs(float64(got-c.want)) > 1e-6 {
			t.Errorf("%v sung: moved %v, want %v", c.p, got, c.want)
		}
	}
	half := float32((deskHeadHold + 1 - deskTailHold) / 2)
	if got := deskHeld(half); math.Abs(float64(got-0.5)) > 1e-6 {
		t.Errorf("half way through the move: %v, want it half way", got)
	}
}

// The width and height the listener sets are those of the card, kept
// within what the settings offer.
func TestDesktopLyricsSize(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.DesktopW, a.settings.DesktopH = 500, 120
	d := &desktopLyrics{a: a}
	if d.width() != 500 || d.height() != 120 {
		t.Errorf("the card is %dx%d, want 500x120", d.width(), d.height())
	}
	if deskWidth(0) != deskWDefault || deskHeight(0) != deskHDefault {
		t.Errorf("none set: %dx%d", deskWidth(0), deskHeight(0))
	}
	if deskWidth(50) != deskWMin || deskHeight(5000) != deskHMax {
		t.Errorf("out of bounds: %dx%d", deskWidth(50), deskHeight(5000))
	}
}

// The lock in the card's corner unlocks the lyrics, and locks them again:
// it is the way back for a window the pointer goes through.
func TestDesktopLyricsLockInTheCard(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.DesktopLocked = true
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	a.pl.SetLyrics([]lyrics.Line{{At: time.Second, End: 30 * time.Second, Text: "一句歌词"}})
	a.pl.Hold(10 * time.Second)
	d := &desktopLyrics{a: a}
	tt := ui.NewTester(d.view, d.width(), d.height())
	tt.Frame()

	// Locked, the lock shows as the pointer comes to it.
	d.near = true
	tt.Frame()
	if err := tt.Click("解锁桌面歌词"); err != nil {
		t.Fatal(err)
	}
	if a.settings.DesktopLocked {
		t.Fatal("the lock did not unlock the lyrics")
	}
	tt.Frame()
	if err := tt.Click("锁定桌面歌词"); err != nil {
		t.Fatal(err)
	}
	if !a.settings.DesktopLocked {
		t.Error("the lock did not lock them again")
	}
}

// Locked, the lyrics take the pointer as it comes to the lock, or within
// reach of it, and not from the rest of the card.
func TestDesktopLyricsNearTheLock(t *testing.T) {
	lock := ui.Rect{X: 700, Y: 3, W: deskLockSize, H: deskLockSize}
	mid := lock.Y + lock.H/2
	cases := []struct {
		name string
		x, y float32
		want bool
	}{
		{"on the lock", lock.X + lock.W/2, mid, true},
		{"at the reach", lock.X - deskLockReach, mid, true},
		{"past the reach", lock.X - deskLockReach - 1, mid, false},
		{"under it", lock.X + lock.W/2, lock.Y + lock.H + deskLockReach + 1, false},
		{"beside it", lock.X + lock.W + deskLockReach, mid, true},
		{"the far end", 20, mid, false},
	}
	for _, c := range cases {
		if got := deskReached(lock, c.x, c.y); got != c.want {
			t.Errorf("%s at (%v, %v): %v", c.name, c.x, c.y, got)
		}
	}
}

// The lyrics take the color the listener chose, of those offered, and
// white for one the settings no longer offer.
func TestDesktopLyricsColor(t *testing.T) {
	for _, c := range deskColors {
		if got := deskInk(c.hex); got != ui.Hex(c.hex) {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if got := deskInk(""); got != white {
		t.Errorf("no color: %v, want white", got)
	}
	if got := deskInk("#123456"); got != white {
		t.Errorf("a color not offered: %v, want white", got)
	}
}

// A dot of the colors offered sets the one the lyrics take.
func TestDesktopLyricsColorsPick(t *testing.T) {
	chosen := ""
	tt := ui.NewTester(func(c *ui.Context) {
		lyricColors(c, &chosen, false)
	}, 320, 40)
	tt.Frame()
	if err := tt.Click("歌词颜色：蓝色"); err != nil {
		t.Fatal(err)
	}
	if chosen != "#8ab4ff" {
		t.Errorf("chose %q, want 蓝色", chosen)
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
