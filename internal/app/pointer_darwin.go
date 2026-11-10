//go:build darwin

package app

import "github.com/ebitengine/purego/objc"

// nativeHidePointer puts the pointer away until it moves, or brings it back
// at once. Hidden this way it comes back by itself on the next move, so that
// the app cannot leave it gone over another app, or when it stands still no
// more: a pointer hidden with NSCursor's hide would wait on its unhide.
func nativeHidePointer(hidden bool) {
	if cls := objc.ID(objc.GetClass("NSCursor")); cls != 0 {
		cls.Send(objc.RegisterName("setHiddenUntilMouseMoves:"), hidden)
	}
}
