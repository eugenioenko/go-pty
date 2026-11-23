//go:build windows
// +build windows

package pty

import (
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A conpty.dll and OpenConsole.exe pair placed next to the executable replaces
// the system ConPTY. Older Windows releases ship a console host that drops
// mouse-tracking modes, so applications can bundle a newer one.
var (
	bundledConptyOnce sync.Once
	bundledConpty     *conptyProcs
)

type conptyProcs struct {
	create, resize, close *windows.Proc
}

func loadBundledConpty() *conptyProcs {
	bundledConptyOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "OpenConsole.exe")); err != nil {
			return
		}
		dll, err := windows.LoadDLL(filepath.Join(dir, "conpty.dll"))
		if err != nil {
			return
		}
		create, err := dll.FindProc("ConptyCreatePseudoConsole")
		if err != nil {
			return
		}
		resize, err := dll.FindProc("ConptyResizePseudoConsole")
		if err != nil {
			return
		}
		closeProc, err := dll.FindProc("ConptyClosePseudoConsole")
		if err != nil {
			return
		}
		bundledConpty = &conptyProcs{create: create, resize: resize, close: closeProc}
	})
	return bundledConpty
}

func coordArg(c windows.Coord) uintptr {
	return uintptr(*(*uint32)(unsafe.Pointer(&c)))
}

func hresultErr(hr uintptr) error {
	if int32(hr) < 0 {
		return windows.Errno(hr)
	}
	return nil
}

func createPseudoConsole(size windows.Coord, in, out windows.Handle, flags uint32, hpc *windows.Handle) error {
	procs := loadBundledConpty()
	if procs == nil {
		return windows.CreatePseudoConsole(size, in, out, flags, hpc)
	}
	hr, _, _ := procs.create.Call(coordArg(size), uintptr(in), uintptr(out), uintptr(flags), uintptr(unsafe.Pointer(hpc)))
	return hresultErr(hr)
}

func resizePseudoConsole(hpc windows.Handle, size windows.Coord) error {
	procs := loadBundledConpty()
	if procs == nil {
		return windows.ResizePseudoConsole(hpc, size)
	}
	hr, _, _ := procs.resize.Call(uintptr(hpc), coordArg(size))
	return hresultErr(hr)
}

func closePseudoConsole(hpc windows.Handle) {
	procs := loadBundledConpty()
	if procs == nil {
		windows.ClosePseudoConsole(hpc)
		return
	}
	_, _, _ = procs.close.Call(uintptr(hpc))
}
