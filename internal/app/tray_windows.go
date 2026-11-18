//go:build windows

package app

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
)

// A tray is the app in the notification area: the window closes to it, and
// its icon brings the window back or quits the app. Windows draws it: a
// message-only window of our own takes what the icon sends.
type tray struct {
	hwnd uintptr
	nid  notifyIconData
	a    *app
	ok   bool // the notification area took the icon
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassW      = user32.NewProc("RegisterClassW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconExW      = shell32.NewProc("ExtractIconExW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmApp           = 0x8001
	wmCommand       = 0x0111
	wmNull          = 0x0000
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	hwndMessage     = ^uintptr(2) // HWND_MESSAGE

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4

	idShow = 1
	idQuit = 2
)

type wndClass struct {
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
}

type point struct{ x, y int32 }

type msgStruct struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	uState           uint32
	wState           uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
}

// startTray puts the app in the notification area, showing a.win when its
// icon is clicked and quitting from its menu.
func startTray(a *app) *tray {
	t := &tray{a: a}
	ready := make(chan struct{})
	go t.run(ready)
	<-ready
	return t
}

func (t *tray) run(ready chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	name, _ := syscall.UTF16PtrFromString("MyGoTray")
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	wc := wndClass{lpfnWndProc: syscall.NewCallback(t.proc), hInstance: hInstance, lpszClassName: name}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, hInstance, 0)
	t.hwnd = hwnd
	t.addIcon()
	close(ready)

	var m msgStruct
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) == 0 {
			return
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// proc handles what the icon sends: a click shows the window, a right
// click the menu.
func (t *tray) proc(hwnd, message, wparam, lparam uintptr) uintptr {
	switch message {
	case wmApp:
		switch lparam {
		case wmLButtonUp, wmLButtonDblClk:
			mygo.RunOnMain(t.a.showMain)
		case wmRButtonUp:
			t.menu()
		}
		return 0
	case wmCommand:
		switch wparam & 0xffff {
		case idShow:
			mygo.RunOnMain(t.a.showMain)
		case idQuit:
			mygo.RunOnMain(mygo.App.Quit)
		}
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return ret
}

// menu shows the icon's menu where the pointer is.
func (t *tray) menu() {
	hmenu, _, _ := procCreatePopupMenu.Call()
	show, _ := syscall.UTF16PtrFromString("显示主窗口")
	quit, _ := syscall.UTF16PtrFromString("退出")
	procAppendMenuW.Call(hmenu, 0, idShow, uintptr(unsafe.Pointer(show)))
	procAppendMenuW.Call(hmenu, 0x800, 0, 0) // a separator
	procAppendMenuW.Call(hmenu, 0, idQuit, uintptr(unsafe.Pointer(quit)))
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWindow.Call(t.hwnd)
	procTrackPopupMenu.Call(hmenu, 0x0002, uintptr(p.x), uintptr(p.y), 0, t.hwnd, 0)
	procDestroyMenu.Call(hmenu)
	// The menu stays until a message comes, as Windows' say it does.
	procPostMessageW.Call(t.hwnd, wmNull, 0, 0)
}

// addIcon puts the icon in the notification area, with the app's own.
func (t *tray) addIcon() {
	var large, small uintptr
	if exe, err := os.Executable(); err == nil {
		path, _ := syscall.UTF16PtrFromString(exe)
		procExtractIconExW.Call(uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
	}
	icon := small
	if icon == 0 {
		icon = large
	}
	t.nid = notifyIconData{
		hWnd:             t.hwnd,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmApp,
		hIcon:            icon,
	}
	t.nid.cbSize = uint32(unsafe.Sizeof(t.nid))
	for i, r := range "MyGo 云音乐" {
		if i >= len(t.nid.szTip)-1 {
			break
		}
		t.nid.szTip[i] = uint16(r)
	}
	ret, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&t.nid)))
	t.ok = ret != 0
}

// remove drops the icon, as the app quits.
func (t *tray) remove() {
	if t.hwnd == 0 {
		return
	}
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid)))
}
