// audioeffects-enablelog 让 neaudioeffects.dll 的调试日志真正输出。
//
// 这个 DLL 的日志入口先调 IsDebuggerPresent，没调试器就直接丢弃。本工具把该
// IAT 槽改成指向注入内存里的 "mov eax,1; ret" 桩，日志就会走 OutputDebugString，
// 由 audioeffects-dump 抓走。
//
// 只改运行中的进程内存，不落地、不改文件；重启云音乐即恢复。
//
//	go build -o audioeffects-enablelog.exe .   (GOARCH=386)
package main

import (
	"fmt"
	"os"
	"unsafe"

	"syscall"
)

const (
	th32csSnapProcess   = 0x00000002
	th32csSnapModule    = 0x00000008
	th32csSnapModule32  = 0x00000010
	processVMOperation  = 0x0008
	processVMWrite      = 0x0020
	processVMRead       = 0x0010
	processQueryInfo    = 0x0400
	memCommit           = 0x1000
	memReserve          = 0x2000
	pageExecuteReadWrite = 0x40
	pageReadWrite        = 0x04
	invalidHandle        = ^uintptr(0)
)

var (
	k32 = syscall.NewLazyDLL("kernel32.dll")

	pCreateToolhelp32Snapshot = k32.NewProc("CreateToolhelp32Snapshot")
	pProcess32First           = k32.NewProc("Process32First")
	pProcess32Next            = k32.NewProc("Process32Next")
	pModule32First            = k32.NewProc("Module32First")
	pModule32Next             = k32.NewProc("Module32Next")
	pOpenProcess              = k32.NewProc("OpenProcess")
	pVirtualAllocEx           = k32.NewProc("VirtualAllocEx")
	pVirtualProtectEx         = k32.NewProc("VirtualProtectEx")
	pWriteProcessMemory       = k32.NewProc("WriteProcessMemory")
	pFlushInstructionCache    = k32.NewProc("FlushInstructionCache")
	pCloseHandle              = k32.NewProc("CloseHandle")
)

type processEntry32 struct {
	Size              uint32
	CntUsage          uint32
	Th32ProcessID     uint32
	Th32DefaultHeapID uint32
	Th32ModuleID      uint32
	CntThreads        uint32
	Th32ParentPID     uint32
	PcPriClassBase    int32
	DwFlags           uint32
	SzExeFile         [260]byte
}

type moduleEntry32 struct {
	Size          uint32
	Th32ModuleID  uint32
	Th32ProcessID uint32
	GlblcntUsage  uint32
	ProccntUsage  uint32
	ModBaseAddr   uint32
	ModBaseSize   uint32
	HModule       uint32
	SzModule      [256]byte
	SzExePath     [260]byte
}

func moduleBase(pid uint32, name string) (uint32, bool) {
	snap, _, _ := pCreateToolhelp32Snapshot.Call(th32csSnapModule|th32csSnapModule32, uintptr(pid))
	if snap == invalidHandle || snap == 0 {
		return 0, false
	}
	defer pCloseHandle.Call(snap)

	var me moduleEntry32
	me.Size = uint32(unsafe.Sizeof(me))
	r, _, _ := pModule32First.Call(snap, uintptr(unsafe.Pointer(&me)))
	for r != 0 {
		if cstr(me.SzModule[:]) == name {
			return me.ModBaseAddr, true
		}
		r, _, _ = pModule32Next.Call(snap, uintptr(unsafe.Pointer(&me)))
	}
	return 0, false
}

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func findTarget() (uint32, string) {
	snap, _, _ := pCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == invalidHandle || snap == 0 {
		return 0, ""
	}
	defer pCloseHandle.Call(snap)

	var pe processEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	r, _, _ := pProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		if cstr(pe.SzExeFile[:]) == "CloudMusic.exe" {
			if base, ok := moduleBase(pe.Th32ProcessID, "neaudioeffects.dll"); ok {
				return pe.Th32ProcessID, fmt.Sprintf("base=0x%08x", base)
			}
		}
		r, _, _ = pProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return 0, ""
}

func main() {
	pid, note := findTarget()
	if pid == 0 {
		fmt.Println("没找到加载了 neaudioeffects.dll 的 CloudMusic.exe，请先把客户端打开。")
		os.Exit(1)
	}
	fmt.Printf("目标进程 pid=%d %s\n", pid, note)

	base, ok := moduleBase(pid, "neaudioeffects.dll")
	if !ok {
		fmt.Println("拿不到模块基址")
		os.Exit(1)
	}

	h, _, err := pOpenProcess.Call(processVMOperation|processVMWrite|processVMRead|processQueryInfo, 0, uintptr(pid))
	if h == 0 {
		fmt.Println("OpenProcess 失败:", err)
		os.Exit(1)
	}
	defer pCloseHandle.Call(h)

	// "mov eax,1; ret"
	stub := []byte{0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3}
	mem, _, err2 := pVirtualAllocEx.Call(h, 0, 0x1000, memCommit|memReserve, pageExecuteReadWrite)
	if mem == 0 {
		fmt.Println("VirtualAllocEx 失败:", err2)
		os.Exit(1)
	}
	var written uint32
	pWriteProcessMemory.Call(h, mem, uintptr(unsafe.Pointer(&stub[0])), uintptr(len(stub)), uintptr(unsafe.Pointer(&written)))

	slot := uintptr(base) + 0xb6084 // IsDebuggerPresent 的 IAT 槽
	var oldProtect uint32
	pr, _, err3 := pVirtualProtectEx.Call(h, slot, 4, pageReadWrite, uintptr(unsafe.Pointer(&oldProtect)))
	if pr == 0 {
		fmt.Println("VirtualProtectEx 失败:", err3)
		os.Exit(1)
	}

	var patched uint32
	okWrite, _, err4 := pWriteProcessMemory.Call(h, slot, uintptr(unsafe.Pointer(&mem)), 4, uintptr(unsafe.Pointer(&patched)))
	if okWrite == 0 {
		fmt.Println("写 IAT 失败:", err4)
		os.Exit(1)
	}
	var restore uint32
	pVirtualProtectEx.Call(h, slot, 4, uintptr(oldProtect), uintptr(unsafe.Pointer(&restore)))
	pFlushInstructionCache.Call(h, mem, 6)

	fmt.Printf("已把 IsDebuggerPresent 指向 0x%08x 的桩，重启客户端即还原。\n", uint32(mem))
	fmt.Println("现在切一次音效，日志就会出现在监听器输出里。")
}
