package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// bioLines is how much of a page's description shows before the page
// offers the rest: two lines say what it is without pushing its songs away.
const bioLines = 2

// pageBio shows a page's description over the bioLines it has room for
// under the page's title, with a link opening the rest of it. NetEase's
// descriptions are often long enough to be the whole story of an artist,
// an album, a playlist, a podcast or a person, and two lines cut most of
// it off with nowhere to read on.
//
// Whether a description is open is kept by the page showing it, so that
// going back to a page finds it as it was left.
func (a *app) pageBio(c *ui.Context, desc string) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return
	}
	p := a.pal
	open := a.bioOpen[a.route]
	// As one paragraph while two lines cut it, and as it was written once
	// all of it shows.
	text, lines := strings.ReplaceAll(desc, "\n", " "), bioLines
	if open {
		text, lines = desc, 0
	}
	ui.Text(c, text).FontSize(12.5).TextColor(p.muted).MaxLines(lines).LineHeight(1.45).Margin(2, 0, 0, 0)
	if !open && !bioOverflows(c, text) {
		return
	}
	label := "展开"
	if open {
		label = "收起"
	}
	link := ui.Link(c.Key("bio"), label, "").FontSize(12.5).TextColor(p.accent).Margin(2, 0, 0, 0)
	if link.Hovered() {
		link.TextColor(p.text)
	}
	if link.Clicked() {
		a.setBioOpen(!open)
	}
}

// setBioOpen opens or closes the description of the page showing.
func (a *app) setBioOpen(open bool) {
	if a.bioOpen == nil {
		a.bioOpen = map[route]bool{}
	}
	a.bioOpen[a.route] = open
}

// headerAlign puts the photo at the head of a page's header at the foot of
// it beside the page's buttons, or at its head once a description opens
// and makes the header taller than the photo.
func (a *app) headerAlign() ui.Align {
	if a.bioOpen[a.route] {
		return ui.Start
	}
	return ui.End
}

// bioOverflows reports whether a description takes more than the bioLines
// a page shows of it.
func bioOverflows(c *ui.Context, bio string) bool {
	span := ui.Span{Text: bio, Size: 12.5}
	_, line := c.MeasureText(0, span)
	_, all := c.MeasureText(bioWidth(c), span)
	return line > 0 && all > float32(bioLines)*line*1.005
}

// bioWidth is how wide a page's description has room to be: what the page
// leaves beside the photo at its head.
func bioWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	return max(1, w-sidebarWidth-2*pageInset-headerPhotoSize-headerGap)
}
