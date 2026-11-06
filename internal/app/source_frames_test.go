package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

// With NM_FRAMES set, saves every place the transport shows — the player
// bar, Now Playing, the wall of what plays next and the notch — with the
// marks of the sources, for looking at them.
func TestSourceFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	scenes := []struct {
		name string
		src  string
		kind string // what shows the transport
		mode player.PlayMode
		dark bool
		w, h int
	}{
		{"bar-netease", netease.SourceNetEase, "bar", player.ModeRepeatAll, false, 1200, 800},
		{"bar-chksz", netease.SourceChKSz, "bar", player.ModeRepeatAll, false, 1200, 800},
		{"bar-chksz-dark", netease.SourceChKSz, "bar", player.ModeRepeatAll, true, 1200, 800},
		{"bar-small", netease.SourceNetEase, "bar", player.ModeRepeatAll, false, 880, 580},
		{"bar-single", netease.SourceNetEase, "bar", player.ModeRepeatOne, false, 1200, 800},
		{"bar-shuffle", netease.SourceNetEase, "bar", player.ModeShuffle, false, 1200, 800},
		{"np-netease", netease.SourceNetEase, "lyrics", player.ModeRepeatAll, false, 1200, 800},
		{"np-chksz", netease.SourceChKSz, "lyrics", player.ModeRepeatAll, false, 1200, 800},
		{"np-small", netease.SourceChKSz, "stage", player.ModeRepeatAll, false, 880, 580},
		{"np-stage", netease.SourceChKSz, "stage", player.ModeRepeatOne, false, 1200, 800},
		{"wall", netease.SourceChKSz, "wall", player.ModeShuffle, false, 1200, 800},
		{"notch", netease.SourceNetEase, "notch", player.ModeRepeatAll, false, notchOpenW + 60, 240},
	}
	save := func(name string, img image.Image) {
		f, _ := os.Create(filepath.Join(dir, "src-"+name+".png"))
		png.Encode(f, img)
		f.Close()
	}
	// A part of the frame, blown up, to look at a small glyph.
	zoom := func(name string, img image.Image, r ui.Rect) {
		z := 8
		w, h := int(r.W), int(r.H)
		out := image.NewRGBA(image.Rect(0, 0, w*z, h*z))
		for y := 0; y < h*z; y++ {
			for x := 0; x < w*z; x++ {
				out.Set(x, y, img.At(int(r.X)+x/z, int(r.Y)+y/z))
			}
		}
		save(name, out)
	}
	for _, s := range scenes {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.path = ""
		a.pl.Cue([]netease.Track{{ID: 3, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Album: netease.Album{Name: "宝船"}, Duration: 239000}})
		a.pl.SetLyrics(nil)
		a.pl.SetStream(&netease.Stream{Level: "lossless", Type: "flac", Source: s.src})
		a.pl.SetMode(s.mode)

		view := a.view
		switch s.kind {
		case "lyrics", "stage", "wall":
			a.nowPlaying = true
			a.npPane = map[string]int{"lyrics": paneLyrics, "stage": paneStage, "wall": paneWall}[s.kind]
		case "notch":
			n := &notch{a: a, real: true, w0: 200, h0: 32, winW: notchOpenW + 60, winH: 240, open: true}
			a.notch = n
			view = n.view
		}

		tt := ui.NewTester(view, s.w, s.h)
		tt.SetDark(s.dark)
		tt.Advance(2 * time.Second)
		tt.Frame()
		img := tt.Image()
		save(s.name, img)

		label := "音源：网易云音乐"
		if s.src == netease.SourceChKSz {
			label = "音源：ChKSz"
		}
		// The mark itself, blown up, to look at its glyph.
		if r, ok := tt.Find(label); ok {
			zoom(s.name+"-mark", img, r)
		}
		// The one button of the play mode, blown up, for its three icons.
		_, mname := modeMark(s.mode)
		if r, ok := tt.Find("播放模式：" + mname); ok {
			zoom(s.name+"-mode", img, r)
		}
		// The foot of the window, where the bar is, larger than it shows.
		if s.kind == "bar" {
			b := img.Bounds()
			band := img.SubImage(image.Rect(b.Min.X, b.Max.Y-140, b.Max.X, b.Max.Y))
			save(s.name+"-crop", band)
		}
	}
}
