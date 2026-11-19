package app

import (
	"testing"

	"github.com/egoist/mygo"
)

// The window opens where and as big as it was left when that place still
// shows on a display, and in the middle of one when the display it was
// left on has gone.
func TestTheWindowOpensWhereItWasLeft(t *testing.T) {
	defer func(taken func() []mygo.Display) { displays = taken }(displays)
	displays = func() []mygo.Display {
		return []mygo.Display{{WorkArea: mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}}}
	}

	a := newTestApp()
	a.settings.WindowX, a.settings.WindowY = 100, 100
	a.settings.WindowWidth, a.settings.WindowHeight = 1000, 700
	if b := a.place(); b != (windowBounds{X: 100, Y: 100, Width: 1000, Height: 700}) {
		t.Fatalf("place kept %+v, want the bounds it was left at", b)
	}

	a.settings.WindowX, a.settings.WindowY = -5000, -5000
	if b := a.place(); b != (windowBounds{Width: 1000, Height: 700}) {
		t.Fatalf("place kept %+v, want its size in the middle", b)
	}

	a.settings.WindowWidth, a.settings.WindowHeight = 0, 0
	if b := a.place(); b != (windowBounds{Width: windowWidth, Height: windowHeight}) {
		t.Fatalf("place kept %+v, want the size it opens at", b)
	}
}
