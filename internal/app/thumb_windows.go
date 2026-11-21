//go:build windows

package app

import (
	"encoding/binary"
	"syscall"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The thumbnail buttons Windows draws under a window's taskbar preview:
// 上一首, 播放 or 暂停, 下一首. The shell's ITaskbarList3 adds them; what the
// listener clicks comes back as WM_COMMAND to the window's thread, which this
// watches with a WH_CALLWNDPROC hook — the window procedure stays mygo's.

const (
	// THBN_CLICKED, and the flags of a thumbnail button.
	thbnClicked  = 0x1800
	thbfEnabled  = 0x0000
	thbfDisabled = 0x0001

	// THUMBBUTTONMASK: which of a button's parts the call sets.
	thbBitmap = 0x1
	thbIcon   = 0x2
	thbTip    = 0x4
	thbFlags  = 0x8

	whCallWndProc = 4 // WH_CALLWNDPROC

	// themeCheck is how often the title bar's color is read again, which
	// changes when the listener changes Windows' theme.
	themeCheck = 2 * time.Second

	thumbPrev = 1
	thumbPlay = 2
	thumbNext = 3

	// ITaskbarList3's methods after IUnknown's three.
	tbHrInit                = 3
	tbThumbBarAddButtons    = 15
	tbThumbBarUpdateButtons = 16
)

var (
	ole32    = syscall.NewLazyDLL("ole32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCoInitializeEx        = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance      = ole32.NewProc("CoCreateInstance")
	procRegGetValueW          = advapi32.NewProc("RegGetValueW")
	procSetWindowsHookExW     = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx        = user32.NewProc("CallNextHookEx")
	procGetCurrentThreadID    = kernel32.NewProc("GetCurrentThreadId")
	procCreateIconFromRes     = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIconThumb      = user32.NewProc("DestroyIcon")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
)

// cwpMsg is CWPSTRUCT: the message a window procedure is about to get, as
// the hook sees it.
type cwpMsg struct {
	lparam  uintptr
	wparam  uintptr
	message uint32
	hwnd    uintptr
}

// thumbButtonInfo is THUMBBUTTON: a button of the thumbnail toolbar.
type thumbButtonInfo struct {
	mask   uint32
	id     uint32
	bitmap uint32
	_      uint32
	icon   uintptr
	tip    [260]uint16
	flags  uint32
}

// thumbBar holds the window's thumbnail buttons and the state they show.
type thumbBar struct {
	a        *app
	hwnd     uintptr
	hook     uintptr // the message hook, watching the window's thread
	cb       uintptr // what the hook calls, so Windows keeps it alive
	hooked   bool
	word     ui.Color   // the color of the title bar's words, which the icons are
	themedAt time.Time  // when word was last read from Windows
	instance uintptr    // ITaskbarList3
	created  uint32     // the message Explorer sends once it made the button
	icons    [4]uintptr // 上一首, 播放, 暂停, 下一首
	built    bool
	playing  bool
	has      bool
	known    bool
}

// startThumbBar puts the playback controls on the window's taskbar preview.
func startThumbBar(a *app) *thumbBar {
	if a.win == nil {
		return nil
	}
	t := &thumbBar{a: a, hwnd: a.win.NativeHandle()}
	if t.hwnd == 0 {
		return nil
	}
	// Explorer tells a new taskbar button's window which message it sends.
	name, _ := syscall.UTF16PtrFromString("TaskbarButtonCreated")
	at, _, _ := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(name)))
	t.created = uint32(at)
	t.cb = syscall.NewCallback(t.proc)
	// The hook goes on as the view builds, not here: it watches the thread that
	// shows the window, which is the one making the first frame.
	return t
}

// ensure watches the window's thread, once, and puts the buttons on the
// taskbar button; a restart of Explorer takes them away, which add() puts back.
func (t *thumbBar) ensure() {
	if t.hooked {
		return
	}
	tid, _, _ := procGetCurrentThreadID.Call()
	hook, _, _ := procSetWindowsHookExW.Call(whCallWndProc, t.cb, 0, tid)
	if hook == 0 {
		return
	}
	t.hook, t.hooked = hook, true
	t.add()
}

// proc sees the messages of the window before its procedure does: the clicks
// of the buttons, and the button Explorer makes again when it starts.
func (t *thumbBar) proc(code, wparam, lparam uintptr) uintptr {
	if int32(uint32(code)) >= 0 && lparam != 0 {
		if m := (*cwpMsg)(ptrOf(lparam)); m.hwnd == t.hwnd {
			switch {
			case m.message == wmCommand && uint32(m.wparam>>16) == thbnClicked:
				if t.a != nil {
					t.a.click(m.wparam & 0xffff)
				}
			case t.created != 0 && m.message == t.created:
				// A taskbar button to put them on: the first one, or the one
				// Explorer made again as it restarted.
				t.add()
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, code, wparam, lparam)
	return r
}

// click plays, pauses, or moves the player as the button clicked says.
func (a *app) click(id uintptr) {
	mygo.RunOnMain(func() {
		switch id {
		case thumbPrev:
			a.pl.Prev()
		case thumbPlay:
			a.pl.TogglePlay()
		case thumbNext:
			a.pl.Next()
		}
	})
}

// taskbar returns the shell's ITaskbarList3, or 0.
func (t *thumbBar) taskbar() uintptr {
	if t.instance != 0 {
		return t.instance
	}
	// The window's thread talks to the shell: give it a flat COM apartment.
	_, _, _ = procCoInitializeEx.Call(0, 2 /* COINIT_APARTMENTTHREADED */)
	clsid := guidOf("56fdf344-fd6d-11d0-958a-006097c9a090")
	iid := guidOf("ea1afb91-9e28-4b86-90e9-9e9f8a5eefaf")
	var tl uintptr
	hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, /* CLSCTX_INPROC_SERVER */
		uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&tl)))
	if failedHR(hr) || tl == 0 || failedHR(comCall(tl, tbHrInit)) {
		return 0
	}
	t.instance = tl
	return tl
}

// add puts the buttons on the taskbar button: the first time, and again after
// Explorer restarted and took them away.
func (t *thumbBar) add() {
	tl := t.taskbar()
	if tl == 0 {
		return
	}
	t.build()
	var buttons [3]thumbButtonInfo
	set := func(i int, id uint32, icon uintptr, tip string) {
		b := &buttons[i]
		b.mask = thbIcon | thbTip | thbFlags
		b.id = id
		b.icon = icon
		copy(b.tip[:], utf16Of(tip))
		b.flags = thbfEnabled
	}
	set(0, thumbPrev, t.icons[0], "上一首")
	set(1, thumbPlay, t.playIcon(), "播放/暂停")
	set(2, thumbNext, t.icons[3], "下一首")
	hr := comCallFull(tl, tbThumbBarAddButtons, t.hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
	t.built = !failedHR(hr)
	if t.built && t.known {
		// What the buttons were built with is what they show.
		t.push()
	}
}

// playIcon is the icon of the middle button: play, or pause while playing.
func (t *thumbBar) playIcon() uintptr {
	if t.playing {
		return t.icons[2]
	}
	return t.icons[1]
}

// update shows the player's state on the buttons, and the color of the title
// bar's words on them, drawing them again when either changed. The buttons stay
// enabled whatever plays: a disabled one takes no clicks, and Explorer sends
// nothing for it.
func (t *thumbBar) update(playing, has bool) {
	state := t.known && playing == t.playing
	if state && time.Since(t.themedAt) < themeCheck {
		return
	}
	word := t.words()
	colorChanged := word != t.word
	t.known, t.playing, t.has, t.word = true, playing, has, word
	t.ensure()
	if t.built {
		if colorChanged && t.icons[0] != 0 {
			t.build()
		}
		t.push()
	}
}

// push writes the state to the buttons Windows already shows.
func (t *thumbBar) push() {
	var buttons [3]thumbButtonInfo
	set := func(i int, id uint32, icon uintptr) {
		b := &buttons[i]
		b.mask = thbIcon | thbFlags
		b.id = id
		b.icon = icon
		b.flags = thbfEnabled
	}
	set(0, thumbPrev, t.icons[0])
	set(1, thumbPlay, t.playIcon())
	set(2, thumbNext, t.icons[3])
	comCallFull(t.instance, tbThumbBarUpdateButtons, t.hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
}

// words is the color of the title bar's words: white on a dark one, near black
// on a light one, as Windows draws the title bar of the theme it is set to.
func (t *thumbBar) words() ui.Color {
	t.themedAt = time.Now()
	if lightTheme() {
		return ui.RGBA(0x1a, 0x1a, 0x1a, 1)
	}
	return ui.RGBA(0xff, 0xff, 0xff, 1)
}

// lightTheme reports whether Windows draws its taskbar and the preview over a
// window light, as personalization says.
func lightTheme() bool {
	const hkeyCurrentUser, rrfRtRegDword = 0x80000001, 0x10
	key, _ := syscall.UTF16PtrFromString("Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize")
	value, _ := syscall.UTF16PtrFromString("SystemUsesLightTheme")
	var v, size uint32 = 0, 4
	hr, _, _ := procRegGetValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(value)),
		rrfRtRegDword, 0, uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&size)))
	return !failedHR(hr) && v != 0
}

// build makes the icons in the color of the title bar's words, drawing them
// over again when the listener changes it.
func (t *thumbBar) build() {
	for _, icon := range t.icons {
		if icon != 0 {
			procDestroyIconThumb.Call(icon)
		}
	}
	t.icons = [4]uintptr{}
	t.icons[0] = iconOf(drawPrev, t.word)
	t.icons[1] = iconOf(drawPlay, t.word)
	t.icons[2] = iconOf(drawPause, t.word)
	t.icons[3] = iconOf(drawNext, t.word)
}

// guid is Windows' GUID: its first three groups as numbers, which Windows
// reads little endian.
type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

// guidOf reads a GUID in its text form, 8-4-4-4-12.
func guidOf(s string) guid {
	hex := func(s string) uint32 {
		var v uint32
		for _, c := range s {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= uint32(c - '0')
			case c >= 'a' && c <= 'f':
				v |= uint32(c-'a') + 10
			case c >= 'A' && c <= 'F':
				v |= uint32(c-'A') + 10
			}
		}
		return v
	}
	digits := make([]byte, 0, 32)
	for _, c := range s {
		if c != '-' {
			digits = append(digits, byte(c))
		}
	}
	d := string(digits)
	g := guid{data1: hex(d[0:8]), data2: uint16(hex(d[8:12])), data3: uint16(hex(d[12:16]))}
	for i := 0; i < 8; i++ {
		g.data4[i] = byte(hex(d[16+2*i : 18+2*i]))
	}
	return g
}

// comCall calls a method of a COM object with no arguments but its own.
func comCall(obj uintptr, method int) uintptr {
	return comCallFull(obj, method)
}

// ptrOf is the pointer at the address p, which the shell hands over as a
// number.
func ptrOf(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

// comCallFull calls a method of a COM object with the arguments after it.
func comCallFull(obj uintptr, method int, args ...uintptr) uintptr {
	// The object's first word is the address of its table of methods.
	vt := *(*uintptr)(ptrOf(obj))
	fn := *(*uintptr)(ptrOf(vt + uintptr(method)*unsafe.Sizeof(uintptr(0))))
	all := append([]uintptr{obj}, args...)
	ret, _, _ := syscall.SyscallN(fn, all...)
	return ret
}

// failedHR reports whether a COM result stands for a failure.
func failedHR(hr uintptr) bool { return int32(uint32(hr)) < 0 }

// utf16Of is the UTF-16 of s, no longer than a thumbnail button's tip.
func utf16Of(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		if len(out) >= 259 {
			break
		}
		out = append(out, uint16(r))
	}
	return out
}

// The glyphs of the buttons, in a box of their own: a triangle for play, two
// bars for pause, a bar beside a triangle for the ones that move. Play is the
// wider shape and pause the squarer one, so the middle button says which it is
// at a glance.
var (
	drawPlay = func(x, y float32) bool {
		return inTriangle(x, y, 0.26, 0.12, 0.26, 0.88, 0.86, 0.50)
	}
	drawPause = func(x, y float32) bool {
		return inBar(x, y, 0.24, 0.44, 0.12, 0.88) || inBar(x, y, 0.56, 0.76, 0.12, 0.88)
	}
	drawPrev = func(x, y float32) bool {
		return inBar(x, y, 0.20, 0.32, 0.14, 0.86) ||
			inTriangle(x, y, 0.80, 0.14, 0.80, 0.86, 0.36, 0.50)
	}
	drawNext = func(x, y float32) bool {
		return inBar(x, y, 0.68, 0.80, 0.14, 0.86) ||
			inTriangle(x, y, 0.20, 0.14, 0.20, 0.86, 0.64, 0.50)
	}
)

func inBar(x, y, x0, x1, y0, y1 float32) bool {
	return x >= x0 && x <= x1 && y >= y0 && y <= y1
}

// inTriangle reports whether (x, y) is in the triangle of three corners.
func inTriangle(x, y, ax, ay, bx, by, cx, cy float32) bool {
	d1 := (x-bx)*(ay-by) - (ax-bx)*(y-by)
	d2 := (x-cx)*(by-cy) - (bx-cx)*(y-cy)
	d3 := (x-ax)*(cy-ay) - (cx-ax)*(y-ay)
	negative := d1 < 0 || d2 < 0 || d3 < 0
	positive := d1 > 0 || d2 > 0 || d3 > 0
	return !(negative && positive)
}

// iconOf draws glyph at 32 pixels, smoothed by four samples a side, and makes
// it an icon of color, which its coverage and the color's own alpha show.
func iconOf(glyph func(x, y float32) bool, color ui.Color) uintptr {
	const size, samples = 32, 4
	alpha := make([]byte, size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hit := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					if glyph((float32(x)+(float32(sx)+0.5)/samples)/size, (float32(y)+(float32(sy)+0.5)/samples)/size) {
						hit++
					}
				}
			}
			alpha[y*size+x] = byte(255 * hit / (samples * samples))
		}
	}
	data := icoBytes(alpha, size, color)
	icon, _, _ := procCreateIconFromRes.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 1,
		0x00030000, size, size, 0)
	return icon
}

// icoBytes is an icon's image of one size, its pixels in 32 bits and its mask
// in one bit, as the shell reads them: the header first, without the icon
// directory a .ico file puts around it, which this call does not take.
func icoBytes(alpha []byte, size int, color ui.Color) []byte {
	const bitmapInfo = 40
	pixels := size * size * 4
	maskRow := ((size + 31) / 32) * 4
	mask := maskRow * size
	h := make([]byte, bitmapInfo+pixels+mask)
	binary.LittleEndian.PutUint32(h[0:], bitmapInfo)
	binary.LittleEndian.PutUint32(h[4:], uint32(size))
	binary.LittleEndian.PutUint32(h[8:], uint32(size*2)) // the mask under it
	binary.LittleEndian.PutUint16(h[12:], 1)
	binary.LittleEndian.PutUint16(h[14:], 32)
	binary.LittleEndian.PutUint32(h[20:], uint32(pixels+mask))
	// The pixels, bottom row first, the color over the glyph's coverage and
	// kept to what an icon takes: straight over its alpha, which premultiplies
	// it.
	pix := h[bitmapInfo:]
	for y := 0; y < size; y++ {
		src := (size - 1 - y) * size
		for x := 0; x < size; x++ {
			a := uint32(alpha[src+x]) * uint32(color.A) / 255
			i := (y*size + x) * 4
			pix[i] = byte(uint32(color.B) * a / 255)
			pix[i+1] = byte(uint32(color.G) * a / 255)
			pix[i+2] = byte(uint32(color.R) * a / 255)
			pix[i+3] = byte(a)
		}
	}
	m := h[bitmapInfo+pixels:]
	for y := 0; y < size; y++ {
		src := (size - 1 - y) * size
		for x := 0; x < size; x++ {
			if alpha[src+x] == 0 {
				row := y*maskRow + x/8
				m[row] |= 0x80 >> uint(x%8)
			}
		}
	}
	return h
}
