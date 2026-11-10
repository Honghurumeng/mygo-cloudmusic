//go:build !darwin

package app

// nativeHidePointer leaves the pointer as the system draws it: only the
// Mac's goes with Now Playing's chrome, for now.
func nativeHidePointer(hidden bool) {}
