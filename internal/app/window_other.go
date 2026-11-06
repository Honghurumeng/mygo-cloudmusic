//go:build !darwin

package app

func makeBorderless(handle uintptr) {}

func raiseAboveDock(handle uintptr) {}

func moveWindowTo(handle uintptr, x, y, w, h int) {}
