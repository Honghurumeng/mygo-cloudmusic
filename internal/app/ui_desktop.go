package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// desktopLyrics shows the song's lyrics in a panel that floats over the
// windows of the other apps, on every Space: the line sung, each word
// lighting up as it is, and under it the translation or the line to come.
// Locked, the pointer goes through it to what is under; unlocked, it can
// be dragged where the listener wants it.
type desktopLyrics struct {
	a   *app
	win *mygo.Window

	w int
	// saveGen counts the places the lyrics were dragged to, to keep the
	// last once the drag settles.
	saveGen int
}

const (
	deskLyricsW = 760
	// deskLyricsH is the window's height while unlocked, with the bar
	// above the lyrics; locked, the lyrics alone need deskCardH.
	deskLyricsH = 136
	deskCardH   = 100
	// deskLyricsEdge is how far above the Dock the lyrics start.
	deskLyricsEdge = 24
	// deskLyricsPadX is the card's padding, on either side.
	deskLyricsPadX = 26
)

func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a, w: deskLyricsW}
	x, y := d.start()
	d.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         "桌面歌词",
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true,
		DisableResize: true,
		X:             x,
		Y:             y,
		Width:         d.w,
		Height:        d.height(),
		Content:       ui.View(d.view),
	})
	// Escape and the like close a panel: this one stays, until the app
	// quits.
	d.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
		}
	})
	d.win.SetIgnoreMouseEvents(a.settings.DesktopLocked)
	if a.settings.DesktopLyrics {
		d.win.ShowInactive()
	}
	mygo.Screen.OnDisplaysChanged(func() { d.place() })
	return d
}

// start returns where the lyrics go: where they were left, or the bottom
// middle of the display with the menu bar.
func (d *desktopLyrics) start() (x, y int) {
	if s := &d.a.settings; s.DesktopPlaced {
		return s.DesktopX, s.DesktopY
	}
	return deskLyricsSpot(mygo.Screen.PrimaryDisplay().WorkArea, d.w, d.height())
}

// deskLyricsSpot is where the lyrics start on the work area of a display:
// in the middle, above the Dock or the taskbar.
func deskLyricsSpot(work mygo.Rectangle, w, h int) (x, y int) {
	return work.X + (work.Width-w)/2, work.Y + work.Height - h - deskLyricsEdge
}

// place puts the lyrics where they were left, kept within the display
// they show on.
func (d *desktopLyrics) place() {
	if d.win == nil {
		return
	}
	x, y := d.start()
	d.win.SetBounds(withinDisplay(mygo.Rectangle{X: x, Y: y, Width: d.w, Height: d.height()}))
}

// withinDisplay moves r as little as it may to lie on its display's work
// area.
func withinDisplay(r mygo.Rectangle) mygo.Rectangle {
	work := mygo.Screen.DisplayMatching(r).WorkArea
	if work.Width <= 0 || work.Height <= 0 {
		return r
	}
	r.X = min(max(r.X, work.X), work.X+work.Width-r.Width)
	r.Y = min(max(r.Y, work.Y), work.Y+work.Height-r.Height)
	return r
}

// setEnabled shows the lyrics, or hides them, as the settings say.
func (d *desktopLyrics) setEnabled(on bool) {
	if d.win == nil {
		return
	}
	if !on {
		d.keep()
		d.win.Hide()
		return
	}
	if !d.win.IsVisible() {
		// The first time they show, the lyrics come unlocked, with the
		// bar that says how to move them, so that the listener can put
		// them where they want before locking them.
		if !d.a.settings.DesktopPlaced {
			d.a.settings.DesktopLocked = false
		}
		d.place()
		d.win.ShowInactive()
	}
}

// invalidate has the lyrics drawn again.
func (d *desktopLyrics) invalidate() {
	if d.win != nil {
		d.win.Invalidate()
	}
}

// applyLock lets the pointer through the lyrics, or not, as the settings
// say, and gives them the room the bar needs while unlocked.
func (d *desktopLyrics) applyLock() {
	if d.win == nil {
		return
	}
	d.win.SetIgnoreMouseEvents(d.a.settings.DesktopLocked)
	d.resize()
	d.win.Invalidate()
}

// height is the window's height: room for the bar while unlocked.
func (d *desktopLyrics) height() int {
	if d.a.settings.DesktopLocked {
		return deskCardH
	}
	return deskLyricsH
}

// resize gives the window the height it needs, its foot where it was, and
// keeps where it now is.
func (d *desktopLyrics) resize() {
	if d.win == nil {
		return
	}
	r := d.win.Bounds()
	h := d.height()
	if r.Height == h {
		return
	}
	r.Y += r.Height - h
	r.Height = h
	d.win.SetBounds(r)
	s := &d.a.settings
	s.DesktopX, s.DesktopY, s.DesktopPlaced = r.X, r.Y, true
}

// keep remembers where the lyrics are.
func (d *desktopLyrics) keep() {
	if d.win == nil || !d.win.IsVisible() {
		return
	}
	r := d.win.Bounds()
	s := &d.a.settings
	s.DesktopX, s.DesktopY, s.DesktopPlaced = r.X, r.Y, true
}

// moved keeps the place the lyrics were dragged to, once the drag has
// settled: a drag moves the window many times a second.
func (d *desktopLyrics) moved() {
	if d.win == nil {
		return
	}
	r := d.win.Bounds()
	s := &d.a.settings
	if s.DesktopPlaced && r.X == s.DesktopX && r.Y == s.DesktopY {
		return
	}
	s.DesktopX, s.DesktopY, s.DesktopPlaced = r.X, r.Y, true
	d.saveGen++
	gen := d.saveGen
	time.AfterFunc(700*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if gen == d.saveGen {
				d.a.settings.save()
			}
		})
	})
}

// desktopShown returns the line the lyrics show at pos, what goes under
// it, and whether the line has begun: before the first, the first shows,
// faint.
func desktopShown(lines []lyrics.Line, pos time.Duration) (cur int, main, sub string, begun bool) {
	if len(lines) == 0 {
		return -1, "", "", false
	}
	cur = lyrics.Index(lines, pos)
	if cur < 0 {
		return cur, lines[0].Text, "", false
	}
	main = lines[cur].Text
	if lines[cur].Translation != "" {
		return cur, main, lines[cur].Translation, true
	}
	if cur+1 < len(lines) {
		sub = lines[cur+1].Text
	}
	return cur, main, sub, true
}

func (d *desktopLyrics) view(c *ui.Context) {
	a := d.a
	c.Root().Background(ui.Transparent)

	tr := a.pl.Current()
	lines, loaded := a.pl.Lyrics()
	pos := a.pl.Position()
	cur, main, sub, begun := desktopShown(lines, pos)

	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.End).Gap(7).Padding(0, 0, 6, 0).Children(func() {
		if !a.settings.DesktopLocked {
			d.bar(c)
		}
		d.card(c, tr, lines, loaded, cur, main, sub, begun, pos)
	})

	// Follow the song: the next line as it comes, each word as it is
	// sung, and, unlocked, where the window was dragged to.
	if a.pl.Playing() {
		next := cur + 1
		if cur < 0 {
			next = 0
		}
		if next < len(lines) {
			c.After(max(16*time.Millisecond, lines[next].At-pos+20*time.Millisecond))
		}
		if begun && len(lines[cur].Words) > 0 && pos < lines[cur].End {
			c.After(50 * time.Millisecond)
		}
	}
	if tr != nil && !loaded {
		// Lyrics come a moment after the song, which a paused player
		// would not otherwise show.
		c.After(200 * time.Millisecond)
	}
	if !a.settings.DesktopLocked {
		d.moved()
		c.After(400 * time.Millisecond)
	}
}

// bar is the handle the lyrics show while unlocked: a hint, and the way
// to lock them again.
func (d *desktopLyrics) bar(c *ui.Context) {
	ui.Row(c).Height(26).Padding(3, 4, 3, 10).Radius(13).
		Background(ui.RGBA(0, 0, 0, 0.55)).
		Border(0.5, ui.RGBA(255, 255, 255, 0.14)).
		AlignItems(ui.Center).Gap(9).Children(func() {
		ui.Text(c, "拖动歌词调整位置").FontSize(11.5).FontWeight(500).TextColor(ui.RGBA(255, 255, 255, 0.62))
		b := ui.ButtonBase(c.Key("lock")).Padding(2, 8).Radius(10).FocusRing(false).Label("锁定桌面歌词")
		b.Children(func() {
			ui.Icon(c, iconLock).Size(12, 12).TextColor(ui.RGBA(255, 255, 255, 0.8))
			ui.Text(c, "锁定").FontSize(11.5).FontWeight(600).TextColor(ui.RGBA(255, 255, 255, 0.85))
		})
		if b.Clicked() {
			d.a.lockDesktopLyrics(true)
		}
	})
}

// card is the lyrics themselves: the line sung, and under it what it says
// or what comes.
func (d *desktopLyrics) card(c *ui.Context, tr *netease.Track, lines []lyrics.Line, loaded bool, cur int, main, sub string, begun bool, pos time.Duration) {
	card := ui.Box(c).Padding(13, deskLyricsPadX).Radius(16).
		Background(ui.RGBA(0, 0, 0, 0.4)).
		Border(0.5, ui.RGBA(255, 255, 255, 0.14))
	if !d.a.settings.DesktopLocked {
		// Anywhere on the lyrics, but not on a button.
		card.DragWindow()
	}
	card.Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(3).Children(func() {
			switch {
			case tr == nil:
				d.plain(c, "未在播放", 20, 0.5)
			case tr.PodcastID != 0:
				d.plain(c, tr.Name, 22, 0.9)
				d.plain(c, tr.ArtistNames()+" · 播客", 15, 0.55)
			case !loaded:
				d.plain(c, "正在加载歌词…", 20, 0.5)
			case len(lines) == 0:
				d.plain(c, "纯音乐，请欣赏", 22, 0.75)
			default:
				if begun {
					d.line(c, &lines[cur], pos)
				} else {
					d.plain(c, main, 30, 0.55)
				}
				if sub != "" {
					alpha := float32(0.62)
					if lines[cur].Translation == "" {
						alpha = 0.42
					}
					d.plain(c, sub, 16, alpha)
				}
			}
		})
	})
}

// plain sets a line of text, shrunk to fit the window.
func (d *desktopLyrics) plain(c *ui.Context, text string, size, alpha float32) {
	ui.Text(c, text).FontSize(d.fit(c, text, size, 600)).FontWeight(600).
		TextColor(ui.RGBA(255, 255, 255, alpha)).SingleLine()
}

// line sets the line sung, each of its words lighting up as it is sung.
func (d *desktopLyrics) line(c *ui.Context, l *lyrics.Line, pos time.Duration) {
	size := d.fit(c, l.Text, 30, 800)
	if len(l.Words) == 0 {
		ui.Text(c, l.Text).FontSize(size).FontWeight(800).LetterSpacing(-0.4).
			TextColor(white).SingleLine()
		return
	}
	ui.RichText(c.Key("line")).FontSize(size).FontWeight(800).LetterSpacing(-0.4).SingleLine().Children(func() {
		for wi, w := range l.Words {
			lit := clamp01(float32((pos - w.Start).Seconds() / max(0.05, (w.End-w.Start).Seconds())))
			ui.Text(c.Key(wi), w.Text).TextColor(ui.RGBA(255, 255, 255, 0.45+0.55*lit))
		}
	})
}

// fit returns the size to set text in, shrunk until it fits the window.
func (d *desktopLyrics) fit(c *ui.Context, text string, size float32, weight int) float32 {
	maxW := float32(d.w - 2*deskLyricsPadX - 8)
	if text == "" || maxW <= 0 {
		return size
	}
	if m, _ := c.MeasureText(0, ui.Span{Text: text, Size: size, Weight: weight}); m > maxW {
		return max(14, size*maxW/m)
	}
	return size
}

// applyDesktop shows or hides the lyrics over the desktop, as the settings
// say, and lets the pointer through when they are locked.
func (a *app) applyDesktop() {
	if a.desktop == nil {
		return
	}
	a.desktop.setEnabled(a.settings.DesktopLyrics)
	a.desktop.applyLock()
}

// toggleDesktopLyrics turns the lyrics over the desktop on or off.
func (a *app) toggleDesktopLyrics() {
	a.settings.DesktopLyrics = !a.settings.DesktopLyrics
	a.applyDesktop()
	a.settings.save()
	a.redraw()
}

// lockDesktopLyrics lets the pointer through the lyrics, or has them
// dragged.
func (a *app) lockDesktopLyrics(locked bool) {
	a.settings.DesktopLocked = locked
	a.applyDesktop()
	a.settings.save()
	a.redraw()
}
