// audioeffects-dump 抓取网易云 neaudioeffects.dll 的参数 dump。
//
// 原理：OutputDebugString 把字符串写进一块命名的共享内存（DBWIN 协议），
// 不需要调试器，任何进程把这块内存和两个事件建起来就能收到全机器的调试输出。
//
//	C:\...>audioeffects-dump.exe -out .\dump
//
// 然后打开网易云、逐个切换「音效」，每次切换会打出一整块参数，工具按块落成 presets.json。
//
// -selftest 只发两行假日志，用来验证抓取链路本身。
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	bufSize   = 4096
	infinite  = 0xFFFFFFFF
	pageRead  = 0x04
	mapAll    = 0x000F001F
	nsGlobal  = `Global\`
	nsSession = ``
)

var (
	k32                = syscall.NewLazyDLL("kernel32.dll")
	pCreateFileMapping = k32.NewProc("CreateFileMappingW")
	pMapViewOfFile     = k32.NewProc("MapViewOfFile")
	pUnmapViewOfFile   = k32.NewProc("UnmapViewOfFile")
	pCreateEvent       = k32.NewProc("CreateEventW")
	pSetEvent          = k32.NewProc("SetEvent")
	pWaitSingle        = k32.NewProc("WaitForSingleObject")
	pOutputDebugString = k32.NewProc("OutputDebugStringW")
)

// channel 是一组 DBWIN 对象（Global\ 或会话内）。
type channel struct {
	ns    string
	view  []byte
	ready uintptr
	data  uintptr
}

func openChannel(prefix string) (*channel, error) {
	name, _ := syscall.UTF16PtrFromString(prefix + "DBWIN_BUFFER")
	h, _, err := pCreateFileMapping.Call(0, 0, pageRead, 0, bufSize, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, err
	}
	addr, _, err2 := pMapViewOfFile.Call(h, mapAll, 0, 0, 0)
	if addr == 0 {
		return nil, err2
	}
	view := unsafe.Slice((*byte)(unsafe.Pointer(addr)), bufSize)

	rname, _ := syscall.UTF16PtrFromString(prefix + "DBWIN_BUFFER_READY")
	hr, _, err3 := pCreateEvent.Call(0, 0, 0, uintptr(unsafe.Pointer(rname)))
	if hr == 0 {
		return nil, err3
	}
	dname, _ := syscall.UTF16PtrFromString(prefix + "DBWIN_DATA_READY")
	hd, _, err4 := pCreateEvent.Call(0, 0, 0, uintptr(unsafe.Pointer(dname)))
	if hd == 0 {
		return nil, err4
	}
	return &channel{ns: prefix, view: view, ready: hr, data: hd}, nil
}

// text 取缓冲区里最长的一段可打印 ASCII。日志正文是英文，中文前缀直接跳过。
func (c *channel) text() string {
	best, cur := "", ""
	for _, b := range c.view[4:] {
		if b == 0 {
			break
		}
		if (b >= 0x20 && b < 0x7f) || b == '\t' {
			cur += string(b)
			if len(cur) > len(best) {
				best = cur
			}
		} else {
			cur = ""
		}
	}
	return strings.TrimSpace(best)
}

// 已知模块名，长的排前面，避免 "Reverb" 抢先匹配 "Reverb ER"。
var modules = []string{
	"Reverb RVB Level", "Reverb Output Level", "Reverb Input Level",
	"Parametric EQ Filter", "Reverb TC Filter", "Sample Format",
	"Processor instance", "Stereo Enhancer", "Parametric EQ", "Bass Treble",
	"Reverb RVB", "Reverb ER", "Reverb TC", "Graph EQ", "Rotate 3D",
	"BypassMode", "Loudness", "Limiter", "Reverb", "Headphone", "FIR",
}

var bracket = regexp.MustCompile(`^[\[\(][^\]\)]*[\]\)]\s*`)
var numOrBool = regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?$`)

func parseVal(s string) (any, bool) {
	s = strings.TrimSpace(s)
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	if !numOrBool.MatchString(s) {
		return nil, false
	}
	if !strings.ContainsAny(s, ".eE") {
		var i int64
		if _, err := fmt.Sscan(s, &i); err == nil {
			return i, true
		}
	}
	var f float64
	if _, err := fmt.Sscan(s, &f); err == nil {
		return f, true
	}
	return nil, false
}

// splitModule 从行首切出模块名和可能的下标（"Parametric EQ Filter 3: ..."）。
func splitModule(line string) (mod string, idx int, rest string, ok bool) {
	for _, m := range modules {
		if !strings.HasPrefix(line, m) {
			continue
		}
		rest = strings.TrimPrefix(line, m)
		idx = -1
		// 形如 " 3:" 或 " 3 " 的下标
		trimmed := strings.TrimLeft(rest, " ")
		if strings.HasPrefix(trimmed, ":") {
			rest = strings.TrimPrefix(trimmed, ":")
		} else {
			n := 0
			for n < len(trimmed) && trimmed[n] >= '0' && trimmed[n] <= '9' {
				n++
			}
			if n > 0 {
				fmt.Sscan(trimmed[:n], &idx)
				trimmed = strings.TrimLeft(trimmed[n:], " ")
				rest = strings.TrimPrefix(trimmed, ":")
			} else {
				rest = trimmed
			}
		}
		return m, idx, strings.TrimSpace(rest), true
	}
	return "", -1, "", false
}

// parse 把一行 "Reverb RVB: PreDelay 0.05, DecayTime 1.20" 拆成模块 + 键值对。
func parse(line string) (mod string, idx int, kv map[string]any, bare []float64, ok bool) {
	line = strings.TrimSpace(bracket.ReplaceAllString(line, ""))
	mod, idx, rest, ok := splitModule(line)
	if !ok {
		return "", -1, nil, nil, false
	}
	kv = map[string]any{}
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, ":"); i >= 0 {
			k := strings.TrimSpace(part[:i])
			v := strings.TrimSpace(part[i+1:])
			if val, good := parseVal(v); good && k != "" {
				kv[k] = val
				continue
			}
			// 剩下的可能是纯数字（Sample Format: 1, ...）
			if val, good := parseVal(v); good {
				bare = append(bare, toF(val))
			}
			continue
		}
		if i := strings.LastIndex(part, " "); i > 0 {
			k := strings.TrimSpace(part[:i])
			v := strings.TrimSpace(part[i+1:])
			if val, good := parseVal(v); good && k != "" {
				kv[k] = val
				continue
			}
		}
		if val, good := parseVal(part); good {
			bare = append(bare, toF(val))
		}
	}
	return mod, idx, kv, bare, true
}

func toF(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	}
	return 0
}

type block struct {
	Instance string                    `json:"instance"`
	Params   map[string]map[string]any `json:"params"`
	Raw      []string                  `json:"raw,omitempty"`
	At       time.Time                 `json:"at"`
	last     time.Time
}

func newBlock(instance string) *block {
	return &block{Instance: instance, Params: map[string]map[string]any{}, At: time.Now(), last: time.Now()}
}

func (b *block) empty() bool { return len(b.Params) == 0 }

func (b *block) put(mod string, idx int, kv map[string]any, bare []float64) {
	key := mod
	if idx >= 0 {
		key = fmt.Sprintf("%s[%d]", mod, idx)
	}
	if _, ok := b.Params[key]; !ok {
		b.Params[key] = map[string]any{}
	}
	for k, v := range kv {
		b.Params[key][k] = v
	}
	if len(bare) > 0 {
		switch mod {
		case "Graph EQ":
			b.Params[key]["bands"] = bare
		case "Sample Format":
			b.Params[key]["values"] = bare
		}
	}
}

type collector struct {
	mu     sync.Mutex
	out    string
	raw    io.Writer
	cur    *block
	blocks []*block
	seen   map[string]int
}

func (c *collector) line(pid uint32, text string) {
	if text == "" {
		return
	}
	text = strings.TrimSpace(bracket.ReplaceAllString(text, ""))

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.raw != nil {
		fmt.Fprintf(c.raw, "%s [%d] %s\n", time.Now().Format("15:04:05.000"), pid, text)
	}

	if strings.HasPrefix(text, "Processor instance") {
		c.flushLocked()
		name := strings.TrimSpace(strings.TrimPrefix(text, "Processor instance"))
		name = strings.TrimSpace(strings.TrimLeft(name, ": "))
		if name == "" {
			name = "unknown"
		}
		c.cur = newBlock(name)
		c.cur.Raw = append(c.cur.Raw, text)
		fmt.Printf("[%d] %s\n", pid, text)
		return
	}

	mod, idx, kv, bare, ok := parse(text)
	if !ok {
		return
	}
	if c.cur == nil {
		c.cur = newBlock("")
	}
	c.cur.put(mod, idx, kv, bare)
	c.cur.Raw = append(c.cur.Raw, text)
	c.cur.last = time.Now()

	fmt.Printf("[%d] %s\n", pid, text)
}

func (c *collector) flushLocked() {
	if c.cur == nil {
		return
	}
	if !c.cur.empty() {
		c.blocks = append(c.blocks, c.cur)
		c.writeLocked()
	}
	c.cur = nil
}

func (c *collector) flushIdle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cur != nil && time.Since(c.cur.last) > 2*time.Second {
		c.flushLocked()
	}
}

func (c *collector) writeLocked() {
	if c.out == "" {
		return
	}
	data, err := json.MarshalIndent(c.blocks, "", "  ")
	if err != nil {
		return
	}
	tmp := filepath.Join(c.out, "presets.json.tmp")
	final := filepath.Join(c.out, "presets.json")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	os.Rename(tmp, final)
}

func (c *collector) listen(ch *channel) {
	pSetEvent.Call(ch.ready)
	for {
		r, _, _ := pWaitSingle.Call(ch.data, infinite)
		if r != 0 {
			continue
		}
		pid := binary.LittleEndian.Uint32(ch.view[:4])
		text := ch.text()
		pSetEvent.Call(ch.ready)
		c.line(pid, text)
	}
}

func main() {
	out := flag.String("out", "dump", "输出目录（presets.json / raw.log 写在这里）")
	selftest := flag.Bool("selftest", false, "发两条假日志验证抓取链路")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "创建输出目录失败:", err)
		os.Exit(1)
	}
	c := &collector{out: *out, seen: map[string]int{}}

	rawPath := filepath.Join(*out, "raw.log")
	if raw, err := os.OpenFile(rawPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		defer raw.Close()
		c.raw = raw
	}

	var chans []*channel
	for _, ns := range []string{nsGlobal, nsSession} {
		ch, err := openChannel(ns)
		if err != nil {
			fmt.Fprintf(os.Stderr, "跳过 %q 通道: %v\n", ns+"DBWIN_BUFFER", err)
			continue
		}
		chans = append(chans, ch)
		label := ns + `DBWIN_BUFFER`
		if ns == "" {
			label = "DBWIN_BUFFER（会话内）"
		}
		fmt.Printf("已监听 %s\n", label)
	}
	if len(chans) == 0 {
		fmt.Fprintln(os.Stderr, "两条通道都建不起来；要么缺少权限，要么已有其他监听器独占。")
		os.Exit(1)
	}

	for _, ch := range chans {
		go c.listen(ch)
	}

	if *selftest {
		time.Sleep(300 * time.Millisecond)
		for _, s := range []string{
			"[AudioEffects] Sample Format: 1, Buffer Frames: 2048, Channels: 2, Samplerate: 44100, Byte per Sample: 4",
			"[AudioEffects] Parametric EQ Filter 0: on true, FilterType 1, Gain 3.000000, Freq 100.000000, Q 0.700000",
		} {
			p, _ := syscall.UTF16PtrFromString(s)
			pOutputDebugString.Call(uintptr(unsafe.Pointer(p)))
			time.Sleep(50 * time.Millisecond)
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			c.flushIdle()
		case <-sig:
			c.mu.Lock()
			c.flushLocked()
			c.mu.Unlock()
			fmt.Printf("\n共 %d 块，写到 %s\n", len(c.blocks), filepath.Join(*out, "presets.json"))
			return
		}
	}
}
