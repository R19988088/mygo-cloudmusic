//go:build windows

package app

import (
	"syscall"
	"time"
	"unsafe"

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
	whGetMessage  = 3 // WH_GETMESSAGE

	wmLButtonDown   = 0x0201
	wmNcLButtonDown = 0x00A1
	wmActivate      = 0x0006

	// themeCheck is how often the title bar's color is read again, which
	// changes when the listener changes Windows' theme.
	themeCheck = 2 * time.Second

	// threadCheck is how often the process's threads are listed again, to watch
	// the ones the runtime makes later.
	threadCheck = 1500 * time.Millisecond

	thumbPrev = 1
	thumbPlay = 2
	thumbNext = 3

	// ITaskbarList3's methods after IUnknown's three.
	tbHrInit                = 3
	tbThumbBarAddButtons    = 15
	tbThumbBarUpdateButtons = 16
	tbThumbBarSetImageList  = 17
	tbSetThumbnailTooltip   = 19

	// ImageList_Create's flags, images of 32 bits a pixel with their own
	// alpha.
	ilcColor32 = 0x0020

	// The pictures of the four buttons in the list, in the order it holds them.
	imagePrev  = 0
	imagePlay  = 1
	imagePause = 2
	imageNext  = 3
)

var (
	ole32    = syscall.NewLazyDLL("ole32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procCoInitializeEx           = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance         = ole32.NewProc("CoCreateInstance")
	procRegGetValueW             = advapi32.NewProc("RegGetValueW")
	procSetWindowsHookExW        = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx           = user32.NewProc("CallNextHookEx")
	procGetCurrentThreadID       = kernel32.NewProc("GetCurrentThreadId")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procRegisterWindowMessage    = user32.NewProc("RegisterWindowMessageW")

	procImageListCreate  = comctl32.NewProc("ImageList_Create")
	procImageListAdd     = comctl32.NewProc("ImageList_Add")
	procImageListReplace = comctl32.NewProc("ImageList_Replace")
	procCreateDIBSection = gdi32.NewProc("CreateDIBSection")
	procDeleteObject     = gdi32.NewProc("DeleteObject")

	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procThread32First            = kernel32.NewProc("Thread32First")
	procThread32Next             = kernel32.NewProc("Thread32Next")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
)

// threadEntry is THREADENTRY32, one thread of the process the snapshot found.
type threadEntry struct {
	size     uint32
	usage    uint32
	id       uint32
	owner    uint32
	basePri  int32
	deltaPri int32
	flags    uint32
}

// cwpMsg is CWPSTRUCT: the message a window procedure is about to get, as
// the hook sees it.
type cwpMsg struct {
	lparam  uintptr
	wparam  uintptr
	message uint32
	hwnd    uintptr
}

// thumbMsg is MSG, a message taken off a thread's queue, as a WH_GETMESSAGE
// hook sees it.
type thumbMsg struct {
	hwnd    uintptr
	message uint32
	_       uint32
	wparam  uintptr
	lparam  uintptr
	time    uint32
	x       int32
	y       int32
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
	cb2      uintptr // the queue hook's, likewise
	hooked   bool
	threads  map[uint32]bool // the threads of this process already watched
	threadAt time.Time       // when the threads were last listed
	added    bool
	word     ui.Color  // the color of the title bar's words, which the icons are
	themedAt time.Time // when word was last read from Windows
	instance uintptr   // ITaskbarList3
	created  uint32    // the message Explorer sends once it made the button
	// il is the image list the buttons take their pictures from, which
	// Explorer draws from; a picture it keeps by name, so a button is given
	// another one by name rather than by handle.
	il      uintptr
	built   bool
	playing bool
	has     bool
	known   bool
	// The click last acted on, and when: the two hooks can both see a click
	// that is posted and then dispatched, and it counts once.
	lastClick uintptr
	lastAt    time.Time
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
	t.cb2 = syscall.NewCallback(t.procQueue)
	// The hooks go on now, while the window is still hidden: Explorer tells it
	// about the taskbar button it makes as it is shown, and the buttons have to
	// be put on again after that — it forgets the ones from before.
	t.hookThreads()
	return t
}

// ensure watches every thread this process runs, and puts the buttons on the
// taskbar button; a restart of Explorer takes them away, which add() puts back.
func (t *thumbBar) ensure() {
	if t.hooked && time.Since(t.threadAt) < threadCheck {
		return
	}
	t.hookThreads()
	if !t.added && t.hooked {
		t.add()
	}
}

// hookThreads watches all the threads of this process. A window's messages are
// handed to the thread that owns it, and a goroutine is not tied to one: the
// runtime may move the code that made the window, while the loop that reads
// its messages stays on the thread Windows made it on. Watching every thread
// is the only way to be sure the clicks are seen, whichever one they come to,
// and threads made later are picked up on the next pass.
func (t *thumbBar) hookThreads() {
	t.threadAt = time.Now()
	if t.threads == nil {
		t.threads = map[uint32]bool{}
	}
	var pid uint32
	if tid, _, _ := procGetWindowThreadProcessID.Call(t.hwnd, uintptr(unsafe.Pointer(&pid))); tid != 0 {
		t.hookThread(uint32(tid))
	}
	if pid == 0 {
		return
	}
	// TH32CS_SNAPTHREAD: the threads of the whole machine, filtered by process.
	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x4, uintptr(pid))
	if snap == 0 || snap == uintptr(0xffffffff) {
		return
	}
	defer procCloseHandle.Call(snap)
	var e threadEntry
	e.size = uint32(unsafe.Sizeof(e))
	for ok, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&e))); ok != 0; ok, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&e))) {
		if e.owner == pid {
			t.hookThread(e.id)
		}
	}
}

// hookThread watches one thread, once.
func (t *thumbBar) hookThread(tid uint32) {
	if tid == 0 || t.threads[tid] {
		return
	}
	hook, _, _ := procSetWindowsHookExW.Call(whCallWndProc, t.cb, 0, uintptr(tid))
	if hook == 0 {
		return
	}
	// The queue as well: a message posted to the thread, and never dispatched
	// — taken out and dropped — would never reach the window procedure's hook.
	procSetWindowsHookExW.Call(whGetMessage, t.cb2, 0, uintptr(tid))
	t.threads[tid] = true
	if !t.hooked {
		t.hook, t.hooked = hook, true
	}
}

// proc sees the messages of the window before its procedure does: the clicks
// of the buttons, and the button Explorer makes again when it starts. The
// clicks count for any window of the thread: Explorer sends them to the one it
// took the buttons onto, which is not always the one this started with.
func (t *thumbBar) proc(code, wparam, lparam uintptr) uintptr {
	if int32(uint32(code)) >= 0 && lparam != 0 {
		m := (*cwpMsg)(ptrOf(lparam))
		switch {
		case m.message == wmCommand && uint32(m.wparam>>16) == thbnClicked:
			t.press(m.wparam & 0xffff)
		case uint32(m.wparam>>16) == thbnClicked:
			// Whatever message Explorer chose: the mark of a button pressed.
			t.press(m.wparam & 0xffff)
		case t.created != 0 && m.message == t.created:
			// A taskbar button to put them on: the first one, or the one
			// Explorer made again as it restarted.
			t.add()
		case m.message == wmCommand:
		case m.hwnd == t.hwnd && (m.message == wmLButtonDown || m.message == wmNcLButtonDown || m.message == wmActivate):
			// Whether the taskbar preview's clicks reach this window at all.
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, code, wparam, lparam)
	return r
}

// procQueue sees the messages of the thread's queue as it takes them out, even
// the ones no window procedure will run for, and takes the clicks of the buttons
// from there too: the same mark of a button pressed, whatever carries it.
func (t *thumbBar) procQueue(code, wparam, lparam uintptr) uintptr {
	if int32(uint32(code)) >= 0 && lparam != 0 {
		m := (*thumbMsg)(ptrOf(lparam))
		switch {
		case m.message == wmCommand || uint32(m.wparam>>16) == thbnClicked:
			if uint32(m.wparam>>16) == thbnClicked {
				t.press(m.wparam & 0xffff)
			}
		case t.created != 0 && m.message == t.created:
			t.add()
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, code, wparam, lparam)
	return r
}

// press acts on a button pressed once, whichever hook saw it first.
func (t *thumbBar) press(id uintptr) {
	if t.a == nil {
		return
	}
	if id == t.lastClick && time.Since(t.lastAt) < 200*time.Millisecond {
		return
	}
	t.lastClick, t.lastAt = id, time.Now()
	t.a.click(id)
}

// click plays, pauses, or moves the player as the button clicked says, and
// draws the main window's bar again with it: a press on the taskbar's preview
// is no press the window itself hears, so nothing else would.
func (a *app) click(id uintptr) {
	a.post(func() {
		switch id {
		case thumbPrev:
			a.pl.Prev()
		case thumbPlay:
			a.pl.TogglePlay()
		case thumbNext:
			a.pl.Next()
		}
		if a.thumb == nil {
			return
		}
		// The middle button's picture follows at once, as the button that was
		// pressed is still drawn.
		playing := a.pl.Playing()
		if playing == a.thumb.playing {
			return
		}
		a.thumb.playing = playing
		a.thumb.push()
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
	if t.word.A == 0 {
		// The color the icons are drawn in is read before they are, which
		// happens before a frame shows the player's state.
		t.word = t.words()
	}
	t.images(tl)
	var buttons [3]thumbButtonInfo
	t.button(&buttons[0], thumbPrev, imagePrev, "上一首")
	t.button(&buttons[1], thumbPlay, t.playImage(), "播放/暂停")
	t.button(&buttons[2], thumbNext, imageNext, "下一首")
	hr := comCallFull(tl, tbThumbBarAddButtons, t.hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
	t.built = !failedHR(hr)
	t.added = true
	if t.built && t.known {
		// What the buttons were built with is what they show.
		t.push()
	}
}

// button fills b with the picture Explorer draws and the tip it shows.
func (t *thumbBar) button(b *thumbButtonInfo, id uint32, image int, tip string) {
	b.mask = thbBitmap | thbTip | thbFlags
	b.id = id
	b.bitmap = uint32(image)
	copy(b.tip[:], utf16Of(tip))
	b.flags = thbfEnabled
}

// playImage is the picture of the middle button: play, or pause while playing.
func (t *thumbBar) playImage() int {
	if t.playing {
		return imagePause
	}
	return imagePlay
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
	stateChanged := !t.known || playing != t.playing
	t.known, t.playing, t.has, t.word = true, playing, has, word
	t.ensure()
	if t.built {
		if colorChanged {
			t.images(t.instance)
			t.push()
			return
		}
		if stateChanged {
			t.push()
		}
	}
}

// push writes the state to the buttons Windows already shows. It names their
// pictures only, as a button whose tips are set once keeps them.
func (t *thumbBar) push() {
	var buttons [3]thumbButtonInfo
	place := func(i int, id uint32, image int) {
		b := &buttons[i]
		b.mask = thbBitmap | thbFlags
		b.id = id
		b.bitmap = uint32(image)
		b.flags = thbfEnabled
	}
	place(0, thumbPrev, imagePrev)
	place(1, thumbPlay, t.playImage())
	place(2, thumbNext, imageNext)
	comCallFull(t.instance, tbThumbBarUpdateButtons, t.hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
}

// playTip names what the middle button does now, which Explorer shows as its
// tip and which changes with the state the icon does.
func (t *thumbBar) playTip() string {
	if t.playing {
		return "暂停"
	}
	return "播放"
}

// words is the color of the title bar's words: white on a dark one, near black
// on a light one, as Windows draws the title bar of the theme it is set to.
func (t *thumbBar) words() ui.Color {
	t.themedAt = time.Now()
	if lightTheme() {
		return ui.RGB(0x1a, 0x1a, 0x1a)
	}
	return ui.RGB(0xff, 0xff, 0xff)
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

// images draws the four pictures into the image list Explorer gives the
// buttons their pictures from, and gives it that list once: the buttons name
// their picture by its place in it, which is what changes when the state does.
func (t *thumbBar) images(tl uintptr) {
	glyphs := []func(x, y float32) bool{drawPrev, drawPlay, drawPause, drawNext}
	if t.il == 0 {
		il, _, _ := procImageListCreate.Call(32, 32, ilcColor32, uintptr(len(glyphs)), 1)
		if il == 0 {
			return
		}
		t.il = il
		for _, glyph := range glyphs {
			bitmap := glyphBitmap(glyph, t.word, 32)
			procImageListAdd.Call(t.il, bitmap, 0)
			procDeleteObject.Call(bitmap)
		}
		comCallFull(tl, tbThumbBarSetImageList, t.hwnd, t.il)
		return
	}
	// The list is the one Explorer has: its pictures are written over in it,
	// not handed to it again.
	for i, glyph := range glyphs {
		bitmap := glyphBitmap(glyph, t.word, 32)
		procImageListReplace.Call(t.il, uintptr(i), bitmap, 0)
		procDeleteObject.Call(bitmap)
	}
}

// glyphBitmap draws glyph at size pixels into a bitmap of 32 bits a pixel:
// its color over its coverage, premultiplied, which is what an image list of
// ILC_COLOR32 takes.
func glyphBitmap(glyph func(x, y float32) bool, color ui.Color, size int) uintptr {
	var h bitmapInfoHeader
	h.size = 40
	h.width = int32(size)
	h.height = -int32(size) // the top row first
	h.planes = 1
	h.bitCount = 32
	var bits unsafe.Pointer
	bitmap, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&h)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return 0
	}
	coverage := glyphCoverage(glyph, size)
	pixels := unsafe.Slice((*byte)(bits), size*size*4)
	for i := 0; i < size*size; i++ {
		a := uint32(coverage[i]) * uint32(color.A) / 255
		pixels[i*4] = byte(uint32(color.B) * a / 255)
		pixels[i*4+1] = byte(uint32(color.G) * a / 255)
		pixels[i*4+2] = byte(uint32(color.R) * a / 255)
		pixels[i*4+3] = byte(a)
	}
	return bitmap
}

// bitmapInfoHeader is Windows' BITMAPINFOHEADER, the shape of the pixels a
// bitmap is made with.
type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
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

// glyphCoverage is glyph at size pixels, its coverage smoothed by four samples
// a side: how much of each pixel it fills, 0 to 255.
func glyphCoverage(glyph func(x, y float32) bool, size int) []byte {
	const samples = 4
	side := float32(size)
	alpha := make([]byte, size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hit := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					if glyph((float32(x)+(float32(sx)+0.5)/samples)/side, (float32(y)+(float32(sy)+0.5)/samples)/side) {
						hit++
					}
				}
			}
			alpha[y*size+x] = byte(255 * hit / (samples * samples))
		}
	}
	return alpha
}
