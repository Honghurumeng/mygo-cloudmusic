//go:build darwin

package app

import "github.com/ebitengine/purego/objc"

// makeBorderless takes the title bar off a window MyGo made, which it hides
// but keeps: AppKit holds a titled window's title bar inside the screen's
// visible frame, so the lyrics could not reach the foot of the screen, over
// the Dock. A borderless window it lets lie where it is put.
func makeBorderless(handle uintptr) {
	if handle == 0 {
		return
	}
	const nonactivatingPanel, fullSizeContentView = 1 << 7, 1 << 15
	objc.ID(handle).Send(objc.RegisterName("setStyleMask:"), uint(nonactivatingPanel|fullSizeContentView))
}

// raiseAboveDock puts the lyrics over the Dock, which a floating panel is
// under: the card may lie at the very foot of the screen, where the Dock
// would otherwise hide it whole. Below the menu bar, which the lyrics have
// no business covering.
func raiseAboveDock(handle uintptr) {
	if handle == 0 {
		return
	}
	const aboveTheDock = 21 // kCGDockWindowLevel is 20
	objc.ID(handle).Send(objc.RegisterName("setLevel:"), aboveTheDock)
}

// moveWindowTo puts the window whose handle it is so that its top-left is
// at x, y, with the size w by h, in the screen coordinates the rest of the
// app uses. MyGo moves a window by setting its frame, and AppKit keeps a
// frame a title bar's height above the foot of the screen, inside the
// visible frame where the Dock is: the lyrics may lie at the very foot, so
// the origin is set here instead, which AppKit leaves alone.
func moveWindowTo(handle uintptr, x, y, w, h int) {
	if handle == 0 {
		return
	}
	screen := menuBarScreen()
	if screen == 0 {
		return
	}
	f := objc.Send[nsRect](screen, objc.RegisterName("frame"))
	// AppKit counts y up from the foot of the display with the menu bar,
	// the one the screen space begins at.
	at := nsPoint{X: float64(x), Y: f.Size.H - float64(y) - float64(h)}
	objc.ID(handle).Send(objc.RegisterName("setFrameOrigin:"), at)
}
