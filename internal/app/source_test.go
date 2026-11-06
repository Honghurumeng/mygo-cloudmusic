package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// The marks: NetEase's own, and for a service without one, the first
// letters of its name on a square of its own.
func TestMarksOfSources(t *testing.T) {
	if m := markOf(netease.SourceNetEase); m.icon == nil || m.text != "" || m.name != "网易云音乐" {
		t.Errorf("NetEase's mark: icon %v, text %q, name %q", m.icon != nil, m.text, m.name)
	}
	// A stream cached before sources were kept, or one not yet resolved,
	// is NetEase's.
	if m := markOf(""); m.icon != iconNetease || m.name != "网易云音乐" {
		t.Errorf("unknown source: name %q", m.name)
	}
	for _, c := range []struct {
		source string
		text   string
	}{
		{netease.SourceChKSz, "CH"},
		{"qq", "QQ"},
		{"酷狗", "酷"},
		{"  ", "?"},
	} {
		m := markOf(c.source)
		if m.icon != nil || m.text != c.text {
			t.Errorf("%q shows %q, want %q", c.source, m.text, c.text)
		}
		if m.color == (ui.Color{}) {
			t.Errorf("%q has no color of its own", c.source)
		}
	}
	// Two services do not look alike.
	chksz, qq := markOf("chksz").color, markOf("qq").color
	if chksz == qq {
		t.Error("chksz and qq share a color")
	}
}

// The transport has the one button of the play mode at its left, and
// shows where the song comes from at its right.
func TestTransportShowsTheSource(t *testing.T) {
	for _, c := range []struct {
		source string
		label  string
	}{
		{netease.SourceNetEase, "音源：网易云音乐"},
		{netease.SourceChKSz, "音源：ChKSz"},
	} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.pl.Cue([]netease.Track{{ID: 3, Name: "宝船", Duration: 239000}})
		a.pl.SetStream(&netease.Stream{Level: "lossless", Type: "flac", Source: c.source})
		tt := ui.NewTester(a.view, 1200, 800)
		tt.Advance(time.Second)
		tt.Frame()

		at := func(label string) (ui.Rect, bool) { return tt.Find(label) }
		// A player starts with the list looping, which is the first of
		// the one button's three.
		mode, ok := at("播放模式：列表循环")
		if !ok {
			t.Fatalf("%s: no mode button", c.source)
		}
		prev, ok := at("上一首")
		if !ok {
			t.Fatalf("%s: no previous button", c.source)
		}
		next, ok := at("下一首")
		if !ok {
			t.Fatalf("%s: no next button", c.source)
		}
		mark, ok := at(c.label)
		if !ok {
			t.Fatalf("%s: no mark of the source", c.source)
		}
		// The mode, then the song's own controls, then the mark.
		if !(mode.X < prev.X && next.X < mark.X) {
			t.Errorf("%s: mode %.0f, previous %.0f, next %.0f, mark %.0f are out of order",
				c.source, mode.X, prev.X, next.X, mark.X)
		}
		if mark.X+mark.W > 1200 {
			t.Errorf("%s: the mark runs off the window", c.source)
		}
	}
}

// The three modes share one button: list looping, this song looping, the
// list shuffled, and round again.
func TestOneButtonCyclesModes(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 3, Name: "宝船", Duration: 239000}})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()

	steps := []struct {
		label string
		mode  player.PlayMode
	}{
		{"播放模式：列表循环", player.ModeRepeatAll},
		{"播放模式：单曲循环", player.ModeRepeatOne},
		{"播放模式：随机播放", player.ModeShuffle},
		{"播放模式：列表循环", player.ModeRepeatAll},
	}
	for i, s := range steps {
		if got := a.pl.Mode(); got != s.mode {
			t.Fatalf("step %d: mode %v, want %v", i, got, s.mode)
		}
		if _, ok := tt.Find(s.label); !ok {
			t.Fatalf("step %d: no button named %q", i, s.label)
		}
		if i == len(steps)-1 {
			break
		}
		if err := tt.Click(s.label); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		tt.Frame()
	}
	// The two buttons it was made of are gone: only the one is there.
	for _, old := range []string{"随机播放", "循环"} {
		if _, ok := tt.Find(old); ok {
			t.Errorf("the old button %q is still there", old)
		}
	}
}

// The mark takes no clicks. The pointer resting on it brings a tip that
// names the source in words, for a square of letters that may not say it.
func TestSourceMarkNamesItself(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 3, Name: "宝船", Duration: 239000}})
	a.pl.SetStream(&netease.Stream{Level: "lossless", Type: "flac", Source: netease.SourceChKSz})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	tips := func() int {
		n := 0
		for _, s := range tt.Texts() {
			if s == "音源：ChKSz" {
				n++
			}
		}
		return n
	}
	if got := tips(); got != 1 {
		t.Fatalf("the mark says it %d times before the pointer rests on it, want 1", got)
	}
	r, ok := tt.Find("音源：ChKSz")
	if !ok {
		t.Fatal("no mark")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(2 * time.Second)
	tt.Frame()
	if got := tips(); got != 2 {
		t.Errorf("the tip did not come: named %d times, want 2", got)
	}
}

// The mark stands before the stream says where the song came from: it
// shows NetEase's, the one tried first, rather than nothing at all.
func TestSourceMarkBeforeTheStream(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 3, Name: "宝船", Duration: 239000}})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("音源：网易云音乐"); !ok {
		t.Fatal("no mark before the stream")
	}
	// Resolved, it says what served it.
	a.pl.SetStream(&netease.Stream{Level: "lossless", Type: "flac", Source: netease.SourceChKSz})
	tt.Frame()
	if _, ok := tt.Find("音源：ChKSz"); !ok {
		t.Fatal("the mark did not follow the stream")
	}
}
