package app

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// mark is how the source of a song looks in the transport: the color of
// the square it shows, and on it either an icon of its own, or the first
// letters of its name.
type mark struct {
	name  string // what the source is called, as its tooltip says it
	color ui.Color
	icon  *ui.SVG
	text  string
}

// markOf returns the mark of the service a stream came from. NetEase has
// its own; one this app learned to play through comes as its name says,
// which is all it is known by. A stream kept before the app kept its
// source, or one not yet resolved, is NetEase's, which is tried first.
func markOf(source string) mark {
	switch source {
	case "", netease.SourceNetEase:
		return mark{name: "网易云音乐", color: ui.Hex(neteaseRed), icon: iconNetease}
	case netease.SourceChKSz:
		return lettersMark("ChKSz")
	}
	return lettersMark(source)
}

// lettersMark is the mark of a source with no icon: the first letters of
// its name, on a hue of its own, so that two sources never look alike.
func lettersMark(name string) mark {
	var h uint32
	for _, r := range strings.ToLower(name) {
		h = h*31 + uint32(r)
	}
	return mark{
		name:  name,
		color: ui.Oklch(0.58, 0.16, float32(h%360)),
		text:  initials(name),
	}
}

// initials is what a source with no icon shows: the first two letters of
// its name, or the one character it starts with, as a Chinese name has no
// letters.
func initials(name string) string {
	var out []rune
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		out = append(out, unicode.ToUpper(r))
		if len(out) == 2 || unicode.Is(unicode.Han, r) {
			return string(out)
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

// markScale is the width of a source's square as a share of the
// transport's size, a little wider than the icons beside it.
const markScale = 1.1

// sourceBadge shows where the song playing comes from, at the right of the
// transport: NetEase's mark, or the letters of the service that served it,
// on a rounded square as tall as the icons beside it. It takes no clicks,
// only the pointer's rest, which names the source.
func (a *app) sourceBadge(c *ui.Context, size float32) {
	st := a.pl.Stream()
	source := ""
	if st != nil {
		source = st.Source
	}
	m := markOf(source)
	side := size * markScale
	// As much room as a button of the icons' size takes, so the mark sits
	// on the same line and under the same reach.
	box := side + 16
	b := ui.Box(c.Key("source")).Size(box, size+16).Center().
		Label("音源：" + m.name).Tooltip("音源：" + m.name)
	if st == nil {
		// Nothing plays, or nothing has come of it yet: where it would
		// come from is still only what to expect.
		b.Opacity(0.45)
	}
	b.Children(func() {
		square := ui.Box(c).Size(side, side).Radius(side * 0.28).Center().
			Background(m.color).TextColor(white)
		square.Children(func() {
			if m.icon != nil {
				ui.Icon(c, m.icon).Size(side*0.88, side*0.88)
				return
			}
			// Two letters take less of the square than one character
			// does, or they would touch its sides.
			face := side * 0.42
			if len([]rune(m.text)) == 1 {
				face = side * 0.52
			}
			ui.Text(c, m.text).FontSize(face).FontWeight(800).TextColor(white)
		})
	})
}
