//go:build !windows

package app

// A tray is Windows's: elsewhere the window's own buttons keep the app.
type tray struct{}

func startTray(a *app) *tray { return nil }

func (t *tray) remove() {}
