//go:build !windows

package app

// outlineWindow leaves the window's outline as the platform draws it.
func (a *app) outlineWindow() {}
