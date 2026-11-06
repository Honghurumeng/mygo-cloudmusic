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
// The lock in their corner lets the pointer through to what is under them;
// unlocked, they can be dragged where the listener wants.
type desktopLyrics struct {
	a   *app
	win *mygo.Window

	// near says the pointer has come to the lock, which takes the pointer
	// back to a window that otherwise lets it through.
	near bool
	// armed says the pointer left the lock since the lyrics were locked:
	// the click that locked them must not bring the lock back at once.
	armed bool
	// lockRect is where the lock was drawn, in the window, as the last
	// frame left it: the place the pointer has to come to.
	lockRect ui.Rect
	// saveGen counts the places the lyrics were dragged to, to keep the
	// last once the drag settles.
	saveGen int
	// dragging says the pointer has the lyrics, which follow it; dragFrom
	// is where they stood when it took them, and dragAt where it was.
	dragging bool
	dragFrom mygo.Rectangle
	dragAt   mygo.Point
}

const (
	// deskWDefault and deskHDefault are the size the lyrics start at, and
	// deskWMin..deskHMax bound what the listener may set: the card is as
	// big as they say, and its height decides the size of the text.
	deskWDefault = 760
	deskHDefault = 84
	deskWMin     = 320
	deskWMax     = 1600
	deskHMin     = 56
	deskHMax     = 220
	// deskAlphaDefault is how much of the card shows at first, of 0 to 100.
	deskAlphaDefault = 40
	// deskLyricsEdge is how far above the Dock the lyrics start.
	deskLyricsEdge = 24
	// deskLineHeight is how tall a line stands against its size, deskSubScale
	// how big the line under the line sung is against it, and deskGapScale
	// the room between them against the card's height.
	deskLineHeight = 1.18
	deskSubScale   = 0.53
	deskGapScale   = 0.032
	// deskLockSize is the lock in the card's corner, and deskLockReach how
	// near the pointer may come to it, locked, for the lyrics to take the
	// pointer: a near miss still finds the button.
	deskLockSize  = 22
	deskLockReach = 18
	// deskLockGap is how often the locked lyrics look for the pointer: it
	// comes quicker to one it has, and leaves to one it has not.
	deskLockGap     = 150 * time.Millisecond
	deskLockNearGap = 60 * time.Millisecond
	// deskScrollGap is how often a line too long for the card is drawn
	// again as it moves along.
	deskScrollGap = 16 * time.Millisecond
	// deskDragGap is how often the lyrics look where the pointer has taken
	// them: they follow it as it goes, the window's own moves telling the
	// app nothing of a pointer that stays still against it.
	deskDragGap = 16 * time.Millisecond
	// deskHeadHold and deskTailHold are the shares of a moving line that it
	// stands still at its head, and at its tail: the line is sung a third
	// of the way before it starts to go, and it comes to its end a third
	// before the line is over.
	deskHeadHold = 0.3
	deskTailHold = 0.3
	// deskWordDim is how faint the words of the line sung are before they
	// are, and deskFirstDim how the first line shows before it starts.
	deskWordDim  = 0.45
	deskFirstDim = 0.55
)

func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a}
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
		Width:         d.width(),
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
	// Without its hidden title bar, the window may lie at the very foot of
	// the screen, where the listener may put it, and over the Dock there,
	// which would hide it whole.
	makeBorderless(d.win.NativeHandle())
	raiseAboveDock(d.win.NativeHandle())
	if a.settings.DesktopLyrics {
		d.win.ShowInactive()
		// A window opens where it was asked to, or above the Dock if the
		// system keeps it off the Dock's strip: the lyrics are put where
		// the listener left them, the foot of the screen too.
		d.place()
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
	return deskLyricsSpot(mygo.Screen.PrimaryDisplay().WorkArea, d.width(), d.height())
}

// deskLyricsSpot is where the lyrics start on the work area of a display:
// in the middle, above the Dock or the taskbar.
func deskLyricsSpot(work mygo.Rectangle, w, h int) (x, y int) {
	return work.X + (work.Width-w)/2, work.Y + work.Height - h - deskLyricsEdge
}

// deskWidth returns the width the lyrics take, kept within what they may.
func deskWidth(pt int) int {
	if pt <= 0 {
		return deskWDefault
	}
	return min(max(pt, deskWMin), deskWMax)
}

// deskHeight returns the height the lyrics take, kept within what they may:
// it is the height of the card, and what the text is set to fit.
func deskHeight(pt int) int {
	if pt <= 0 {
		return deskHDefault
	}
	return min(max(pt, deskHMin), deskHMax)
}

// deskAlpha returns how much of the card shows, from 0 to 100. None of it
// is a choice of its own: only the settings that were never set get the
// default.
func deskAlpha(pt int) int {
	if pt < 0 {
		return deskAlphaDefault
	}
	return min(pt, 100)
}

// deskPadX is the card's padding on either side: room for the lock in its
// corner, and more on a wider card.
func deskPadX(w int) float32 {
	return float32(min(max(int(float32(w)*0.034+0.5), 26), 56))
}

// deskPadY is the card's padding above and below the lines, a share of its
// height.
func deskPadY(h int) float32 {
	return float32(min(max(int(float32(h)*0.14+0.5), 8), 30))
}

// deskGap is the room between the line sung and the one under it.
func deskGap(h int) float32 {
	return float32(min(max(int(float32(h)*deskGapScale+0.5), 2), 6))
}

// deskTypeSize is the size the lyrics are set in, which the height of the
// card decides: what its padding and the gap leave goes to the two lines,
// the one under at deskSubScale of the one sung.
func deskTypeSize(h int) float32 {
	room := float32(h) - 2*deskPadY(h) - deskGap(h) - 4
	if room < 10 {
		room = 10
	}
	return room / (deskLineHeight * (1 + deskSubScale))
}

// deskProgress is how far the line has been sung, from 0 to 1: the share of
// the line too long for the card that has come into view.
func deskProgress(l *lyrics.Line, pos time.Duration) float32 {
	span := l.End - l.At
	if span <= 0 {
		return 0
	}
	return clamp01(float32((pos - l.At).Seconds() / span.Seconds()))
}

// deskHeld spreads the part of the line sung over the part of it that
// moves: the line stands still while its first third is sung, then goes to
// its tail, which shows whole while the last third of it is still to sing.
func deskHeld(p float32) float32 {
	return clamp01((p - deskHeadHold) / (1 - deskHeadHold - deskTailHold))
}

// deskScroll returns where a line of the width given stands in a card that
// leaves it the room given, once p of it has been sung: from its head,
// which is flush with the card's padding, to its tail, which is too. A line
// that fits stands where it is, and does not move.
func deskScroll(width, room float32, p float32) (shift float32, moving bool) {
	if width <= room-1 {
		return 0, false
	}
	return -(width - room) * deskHeld(p), true
}

// deskColors are the colors the lyrics over the desktop may take, named as
// the settings offer them: light ones, over the card's dark glass.
var deskColors = []struct{ name, hex string }{
	{"白色", "#ffffff"}, {"暖白", "#ffe8c7"}, {"粉色", "#ff9ec4"}, {"红色", "#ff6b6b"}, {"橙色", "#ffb26b"},
	{"黄色", "#ffd166"}, {"绿色", "#7ee787"}, {"青色", "#67e8f9"}, {"蓝色", "#8ab4ff"}, {"紫色", "#c9a7ff"},
}

// deskInk returns the color of the lyrics: the one the listener chose of
// those offered, or white for none.
func deskInk(hex string) ui.Color {
	for _, c := range deskColors {
		if c.hex == hex {
			return ui.Hex(c.hex)
		}
	}
	return white
}

// lyricColors offers the colors the lyrics may take, the one chosen
// ringed, and reports whether the user chose another.
func lyricColors(c *ui.Context, chosen *string, disabled bool) bool {
	before := *chosen
	ring := c.Theme().Text.Alpha(0.8)
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, col := range deskColors {
			on := *chosen == col.hex || (*chosen == "" && col.hex == "#ffffff")
			b := ui.ButtonBase(c.Key(col.hex)).Size(24, 24).Radius(999).Disabled(disabled).
				Label("歌词颜色：" + col.name).Tooltip(col.name)
			bg := ui.Transparent
			if b.Hovered() && !disabled {
				bg = ui.RGBA(128, 128, 128, 0.16)
			}
			b.Background(bg)
			b.Draw(func(p *ui.Painter, r ui.Rect) {
				cx, cy := r.X+r.W/2, r.Y+r.H/2
				dot := ui.Rect{X: cx - 7, Y: cy - 7, W: 14, H: 14}
				p.Fill(dot, ui.Hex(col.hex), 7)
				p.Stroke(dot, ui.RGBA(0, 0, 0, 0.14), 7, 0.5)
				if on {
					p.Stroke(ui.Rect{X: cx - 10.5, Y: cy - 10.5, W: 21, H: 21}, ring, 10.5, 1.5)
				}
			})
			if b.Clicked() && !disabled {
				*chosen = col.hex
			}
		}
	})
	return *chosen != before
}

// place puts the lyrics where they were left, kept within the display
// they show on.
func (d *desktopLyrics) place() {
	if d.win == nil {
		return
	}
	x, y := d.start()
	r := withinDisplay(mygo.Rectangle{X: x, Y: y, Width: d.width(), Height: d.height()})
	d.move(r)
}

// move puts the lyrics at r: the size through MyGo, and then the place
// through the window itself, which AppKit keeps above the Dock's strip
// otherwise, however near the foot of the screen they are put.
func (d *desktopLyrics) move(r mygo.Rectangle) {
	if d.win == nil {
		return
	}
	d.win.SetBounds(r)
	moveWindowTo(d.win.NativeHandle(), r.X, r.Y, r.Width, r.Height)
}

// withinDisplay moves r as little as it may to lie on its display, the
// Dock and the menu bar too: the lyrics may sit at the very foot of the
// screen, where a window the listener put them has to stay.
func withinDisplay(r mygo.Rectangle) mygo.Rectangle {
	b := mygo.Screen.DisplayMatching(r).Bounds
	if b.Width <= 0 || b.Height <= 0 {
		return r
	}
	return deskFit(r, b)
}

// deskFit moves r as little as it may to lie on area.
func deskFit(r, area mygo.Rectangle) mygo.Rectangle {
	r.X = min(max(r.X, area.X), area.X+area.Width-r.Width)
	r.Y = min(max(r.Y, area.Y), area.Y+area.Height-r.Height)
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
		// The first time they show, the lyrics come unlocked, so that the
		// listener can put them where they want before locking them.
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
// say, and gives them the room their size needs.
func (d *desktopLyrics) applyLock() {
	if d.win == nil {
		return
	}
	d.near, d.armed = false, false
	d.win.SetIgnoreMouseEvents(d.a.settings.DesktopLocked)
	d.resize()
	d.win.Invalidate()
}

// width is the window's width: the card the listener set.
func (d *desktopLyrics) width() int {
	return deskWidth(d.a.settings.DesktopW)
}

// height is the window's height: the card the listener set, which sets the
// size of the text too.
func (d *desktopLyrics) height() int {
	return deskHeight(d.a.settings.DesktopH)
}

// resize gives the window the size the settings ask for: it grows about its
// middle and from its foot, where the listener put it, and keeps where it
// now is.
func (d *desktopLyrics) resize() {
	if d.win == nil {
		return
	}
	r := d.win.Bounds()
	w, h := d.width(), d.height()
	if r.Width == w && r.Height == h {
		return
	}
	r.X += (r.Width - w) / 2
	r.Y += r.Height - h
	r.Width, r.Height = w, h
	// A card grown past the edge of the display comes back onto it.
	r = withinDisplay(r)
	d.move(r)
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

// drag moves the lyrics with the pointer. MyGo's own drag asks the window
// server, which keeps a window's frame within the screen's visible frame:
// its foot may not go below the Dock, however far it is dragged. The
// lyrics are moved here instead, by the pointer's own travel, so that they
// go where the listener puts them: the very foot of the screen too.
func (d *desktopLyrics) drag(ev ui.InputEvent) bool {
	if d.win == nil || d.a.settings.DesktopLocked {
		return false
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 || d.onLock(ev.X, ev.Y) {
			return false
		}
		d.dragging = true
		d.dragFrom = d.win.Bounds()
		d.dragAt = mygo.Screen.CursorScreenPoint()
		return true
	case ui.InputPointerMove:
		if !d.dragging || ev.Button != 0 {
			return false
		}
		d.follow()
		return true
	case ui.InputPointerUp:
		if !d.dragging {
			return false
		}
		d.dragging = false
		d.moved()
		return true
	}
	return false
}

// follow puts the lyrics where the pointer has taken them.
func (d *desktopLyrics) follow() {
	if d.win == nil {
		return
	}
	d.move(withinDisplay(deskDrag(d.dragFrom, d.dragAt, mygo.Screen.CursorScreenPoint())))
}

// deskDrag returns where the lyrics stand once the pointer has gone from
// press to at, having taken them at from.
func deskDrag(from mygo.Rectangle, press, at mygo.Point) mygo.Rectangle {
	from.X += at.X - press.X
	from.Y += at.Y - press.Y
	return from
}

// onLock reports whether the pointer at x, y, in the card, is on the lock,
// or near enough: a press there belongs to the button, not to the drag.
func (d *desktopLyrics) onLock(x, y float32) bool {
	r := d.lockRect
	if r.W <= 0 {
		return false
	}
	return x >= r.X-2 && x <= r.X+r.W+2 && y >= r.Y-2 && y <= r.Y+r.H+2
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
	cur, _, sub, begun := desktopShown(lines, pos)

	moving := d.card(c, tr, lines, loaded, cur, sub, begun, pos)

	// Follow the song: the next line as it comes, each word as it is
	// sung, and a line too long for the card as it moves along.
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
		if moving {
			c.After(deskScrollGap)
		}
	}
	if tr != nil && !loaded {
		// Lyrics come a moment after the song, which a paused player
		// would not otherwise show.
		c.After(200 * time.Millisecond)
	}
	d.watch(c)
}

// watch follows where the lyrics lie, and the pointer: dragged, they keep
// where they were left; locked, they look for the pointer coming to the
// lock, which is the one place they take it.
func (d *desktopLyrics) watch(c *ui.Context) {
	if d.win == nil {
		return
	}
	if d.dragging {
		// The pointer may go on while the window's own moves tell the app
		// nothing of it: it is read where it is, every frame.
		d.follow()
		c.After(deskDragGap)
	}
	if !d.a.settings.DesktopLocked {
		d.moved()
		c.After(400 * time.Millisecond)
		return
	}
	at := d.pointing()
	if !at {
		// Away from the lock, it is ready to show again.
		d.armed = true
	}
	if near := at && d.armed; near != d.near {
		d.near = near
		d.win.SetIgnoreMouseEvents(!near)
		d.win.Invalidate()
	}
	if d.near {
		c.After(deskLockNearGap)
	} else {
		c.After(deskLockGap)
	}
}

// pointing reports whether the pointer is at the lock, or near it. A
// window the pointer goes through hears nothing of it: where it is, read
// off the screen, is what tells the lyrics to take the pointer back.
func (d *desktopLyrics) pointing() bool {
	if d.win == nil {
		return false
	}
	r, b := d.lockRect, d.win.Bounds()
	if r.W <= 0 || b.Width <= 0 {
		return false
	}
	p := mygo.Screen.CursorScreenPoint()
	return deskReached(r, float32(p.X-b.X), float32(p.Y-b.Y))
}

// deskReached reports whether x, y, in the window, is at the lock r or
// within reach of it.
func deskReached(r ui.Rect, x, y float32) bool {
	return x >= r.X-deskLockReach && x <= r.X+r.W+deskLockReach &&
		y >= r.Y-deskLockReach && y <= r.Y+r.H+deskLockReach
}

// card is the lyrics themselves: the line sung, and under it what it says
// or what comes, with the lock in the corner. It reports whether the line
// sung is moving, too long for the card to show at once.
func (d *desktopLyrics) card(c *ui.Context, tr *netease.Track, lines []lyrics.Line, loaded bool, cur int, sub string, begun bool, pos time.Duration) (moving bool) {
	s := &d.a.settings
	size, ink := deskTypeSize(d.height()), deskInk(s.DesktopColor)
	alpha := float32(deskAlpha(s.DesktopAlpha)) / 100

	card := ui.Box(c.Key("card")).Fill().Radius(16).
		Background(ui.RGBA(0, 0, 0, alpha)).
		Border(0.5, ui.RGBA(255, 255, 255, 0.16*alpha))
	if !s.DesktopLocked {
		// Anywhere on the lyrics, but not on the lock, which is a button
		// of its own.
		card.HandleInput(d.drag)
	}
	card.Children(func() {
		ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(deskGap(d.height())).
			Padding(deskPadY(d.height()), deskPadX(d.width())).Children(func() {
			switch {
			case tr == nil:
				d.plain(c, ink, "未在播放", size*0.66, 0.5)
			case tr.PodcastID != 0:
				d.plain(c, ink, tr.Name, size*0.73, 0.9)
				d.plain(c, ink, tr.ArtistNames()+" · 播客", size*deskSubScale, 0.55)
			case !loaded:
				d.plain(c, ink, "正在加载歌词…", size*0.66, 0.5)
			case len(lines) == 0:
				d.plain(c, ink, "纯音乐，请欣赏", size*0.73, 0.75)
			default:
				if begun {
					moving = d.lyric(c, ink, size, &lines[cur], pos, deskWordDim)
				} else {
					// Before the first line is sung, it shows faint.
					moving = d.lyric(c, ink, size, &lines[0], pos, deskFirstDim)
				}
				if sub != "" {
					dim := float32(0.62)
					if lines[cur].Translation == "" {
						dim = 0.42
					}
					d.plain(c, ink, sub, size*deskSubScale, dim)
				}
			}
		})
		d.lock(c, ink)
	})
	return moving
}

// lock is the button in the card's corner: unlocked, it locks the lyrics
// where they are, letting the pointer through; locked, it shows as the
// pointer comes near, and unlocks them.
func (d *desktopLyrics) lock(c *ui.Context, ink ui.Color) {
	locked := d.a.settings.DesktopLocked
	label := "锁定桌面歌词"
	if locked {
		label = "解锁桌面歌词"
	}
	b := ui.ButtonBase(c.Key("lock")).Absolute().Top(3).Right(3).
		Size(deskLockSize, deskLockSize).Radius(deskLockSize / 2).Center().
		FocusRing(false).Label(label).Tooltip(label)
	fill := ui.RGBA(255, 255, 255, 0.1)
	if b.Hovered() {
		fill = ui.RGBA(255, 255, 255, 0.22)
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	// Locked, out of reach of the pointer, the lock stays out of sight,
	// with the lyrics it would unlock.
	shown := float32(1)
	if locked && !d.near {
		shown = 0
	}
	b.Opacity(b.AnimateTo("shown", shown, ui.Animation{Duration: 130 * time.Millisecond}))
	b.Children(func() {
		ui.Icon(c, iconLock).Size(11, 11).TextColor(ink.Alpha(0.85))
	})
	if b.Clicked() {
		d.a.lockDesktopLyrics(!locked)
	}
	d.lockRect = b.Bounds()
}

// plain sets a line that is not sung, shrunk to fit the card.
func (d *desktopLyrics) plain(c *ui.Context, ink ui.Color, text string, size, alpha float32) {
	ui.Text(c, text).FontSize(d.fit(c, text, size, 600)).FontWeight(600).
		TextColor(ink.Alpha(alpha)).SingleLine()
}

// lyric sets a line of the lyrics, each of its words lit as far as it is
// sung: in the middle while it fits the card, and when it does not, standing
// still while its first third is sung, then moved along to its tail, which
// shows whole for the last third. It reports whether the line moves.
func (d *desktopLyrics) lyric(c *ui.Context, ink ui.Color, size float32, l *lyrics.Line, pos time.Duration, dim float32) bool {
	// Each word stands on its own, with how lit it is: the line is moved as
	// a whole.
	var spans []ui.Span
	var alphas []float32
	if len(l.Words) == 0 {
		// Without words to light, the line is set a letter at a time.
		for _, r := range l.Text {
			spans = append(spans, ui.Span{Text: string(r), Size: size, Weight: 800, LetterSpacing: -0.4})
			alphas = append(alphas, dim)
		}
	}
	for _, w := range l.Words {
		lit := clamp01(float32((pos - w.Start).Seconds() / max(0.05, (w.End-w.Start).Seconds())))
		spans = append(spans, ui.Span{Text: w.Text, Size: size, Weight: 800, LetterSpacing: -0.4})
		alphas = append(alphas, dim+(1-dim)*lit)
	}

	room := float32(d.width()) - 2*deskPadX(d.width())
	width, _ := c.MeasureText(0, spans...)
	shift, moving := deskScroll(width, room, deskProgress(l, pos))
	draw := func() {
		t := ui.RichText(c.Key("line")).FontSize(size).FontWeight(800).LetterSpacing(-0.4)
		if moving {
			// A line that does not fit is kept whole, and moved, rather
			// than cut short.
			t = t.NoWrap().Translate(shift, 0)
		} else {
			t = t.SingleLine()
		}
		t.Children(func() {
			for i, s := range spans {
				ui.Text(c.Key(i), s.Text).TextColor(ink.Alpha(alphas[i]))
			}
		})
	}
	if !moving {
		draw()
		return false
	}
	ui.Box(c.Key("scroll")).FillWidth().ClipX().Children(draw)
	return true
}

// fit returns the size to set text in, shrunk until it fits the card.
func (d *desktopLyrics) fit(c *ui.Context, text string, size float32, weight int) float32 {
	maxW := float32(d.width()) - 2*deskPadX(d.width()) - 8
	if text == "" || maxW <= 0 {
		return size
	}
	if m, _ := c.MeasureText(0, ui.Span{Text: text, Size: size, Weight: weight}); m > maxW {
		return max(12, size*maxW/m)
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
