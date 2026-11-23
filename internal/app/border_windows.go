//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

// dwmwaBorderColor is DWMWA_BORDER_COLOR: the color of the hairline Windows 11
// draws around a rounded window, which its apps may set.
const dwmwaBorderColor = 34

var (
	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// outlineWindow puts the color of the categories' column — the first level —
// on the hairline around the window, so the frame of the app carries the theme
// the listener chose rather than the system's accent. Windows 10 has no such
// attribute: the call fails and the window keeps its own outline.
func (a *app) outlineWindow() {
	if a.win == nil {
		return
	}
	c := a.pal.nav.bg
	ref := uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16
	if ref == a.border {
		return
	}
	hwnd := a.win.NativeHandle()
	if hwnd == 0 {
		return
	}
	a.border = ref
	procDwmSetWindowAttribute.Call(hwnd, dwmwaBorderColor, uintptr(unsafe.Pointer(&ref)), 4)
}
