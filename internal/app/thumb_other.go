//go:build !windows

package app

import "github.com/egoist/mygo/ui"

// thumbBar is the playback controls on a window's taskbar preview: Windows
// draws those, and no other platform has such a place.
type thumbBar struct{}

func startThumbBar(a *app) *thumbBar { return nil }

func (t *thumbBar) update(playing, has bool, word ui.Color) {}
