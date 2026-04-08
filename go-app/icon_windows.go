package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procSendMessageW    = user32.NewProc("SendMessageW")
	procFindWindowW     = user32.NewProc("FindWindowW")
	procEnumWindows     = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procLoadImageW      = user32.NewProc("LoadImageW")
)

const (
	WM_SETICON    = 0x0080
	ICON_SMALL    = 0
	ICON_BIG      = 1
	IMAGE_ICON    = 1
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040
)

func setWindowIcon() {
	// Find icon path next to exe
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	icoPath := filepath.Join(filepath.Dir(exePath), "icon.ico")
	if _, err := os.Stat(icoPath); os.IsNotExist(err) {
		return
	}

	icoPathW, _ := syscall.UTF16PtrFromString(icoPath)

	// Load icons
	hIconBig, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(icoPathW)), IMAGE_ICON, 32, 32, LR_LOADFROMFILE)
	hIconSmall, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(icoPathW)), IMAGE_ICON, 16, 16, LR_LOADFROMFILE)

	if hIconBig == 0 && hIconSmall == 0 {
		return
	}

	// Find our window by enumerating and matching PID
	pid := os.Getpid()
	type enumData struct {
		pid  int
		hwnd uintptr
	}
	data := &enumData{pid: pid}

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		d := (*enumData)(unsafe.Pointer(lParam))
		var windowPid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPid)))
		if int(windowPid) == d.pid {
			d.hwnd = hwnd
			return 0 // stop
		}
		return 1 // continue
	})

	procEnumWindows.Call(cb, uintptr(unsafe.Pointer(data)))

	if data.hwnd != 0 {
		if hIconBig != 0 {
			procSendMessageW.Call(data.hwnd, WM_SETICON, ICON_BIG, hIconBig)
		}
		if hIconSmall != 0 {
			procSendMessageW.Call(data.hwnd, WM_SETICON, ICON_SMALL, hIconSmall)
		}
	}
}
