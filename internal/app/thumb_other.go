//go:build !windows

package app

// thumbBar is the playback controls on a window's taskbar preview: Windows
// draws those, and no other platform has such a place.
type thumbBar struct{}

func startThumbBar(a *app) *thumbBar { return nil }

func (t *thumbBar) update(playing, has bool) {}
