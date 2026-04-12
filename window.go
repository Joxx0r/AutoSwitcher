//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procShowWindow               = user32.NewProc("ShowWindow")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procGetWindowTextLength      = user32.NewProc("GetWindowTextLengthW")
	procGetWindow                = user32.NewProc("GetWindow")
	procGetWindowLong            = user32.NewProc("GetWindowLongW")

	kernel32                        = windows.NewLazySystemDLL("kernel32.dll")
	procQueryFullProcessImageName   = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetCurrentThreadId          = kernel32.NewProc("GetCurrentThreadId")
)

const (
	swRestore   = 9
	gwOwner     = 4      // GW_OWNER
	wsExToolWin = 0x0080 // WS_EX_TOOLWINDOW
)

// WindowInfo holds information about a top-level window.
type WindowInfo struct {
	HWND    uintptr
	PID     uint32
	ExeName string
}

// findWindowsByExe is the function used to find windows. It's a variable so tests can override it.
var findWindowsByExe = findWindowsByExeImpl

func findWindowsByExeImpl(exeName string) ([]WindowInfo, error) {
	var results []WindowInfo

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		// Skip invisible windows
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1 // continue
		}

		// Skip owned windows (dialogs, popups belonging to another window)
		owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
		if owner != 0 {
			return 1
		}

		// Skip tool windows (WS_EX_TOOLWINDOW)
		exStyle, _, _ := procGetWindowLong.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE = -20
		if exStyle&wsExToolWin != 0 {
			return 1
		}

		// Skip windows with no title (likely helper windows)
		titleLen, _, _ := procGetWindowTextLength.Call(hwnd)
		if titleLen == 0 {
			return 1
		}

		// Get process ID
		var pid uint32
		_, _, _ = procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 {
			return 1
		}

		// Get executable name via OpenProcess + QueryFullProcessImageName
		exePath := getProcessExeName(pid)
		if exePath == "" {
			return 1
		}

		if strings.EqualFold(exePath, exeName) {
			results = append(results, WindowInfo{
				HWND:    hwnd,
				PID:     pid,
				ExeName: exePath,
			})
		}

		return 1 // continue enumeration
	})

	ret, _, err := procEnumWindows.Call(cb, 0)
	if ret == 0 && err != nil && err != windows.ERROR_SUCCESS {
		return nil, fmt.Errorf("EnumWindows: %w", err)
	}

	return results, nil
}

func getProcessExeName(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageName.Call(
		uintptr(handle),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return ""
	}

	fullPath := windows.UTF16ToString(buf[:size])
	return filepath.Base(fullPath)
}

// FocusWindow brings the given window to the foreground.
func FocusWindow(hwnd uintptr) error {
	// Restore if minimized
	iconic, _, _ := procIsIconic.Call(hwnd)
	if iconic != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swRestore)
	}

	// Try SetForegroundWindow directly first (works reliably when running as admin)
	ret, _, _ := procSetForegroundWindow.Call(hwnd)
	if ret != 0 {
		return nil
	}

	// Fallback: AttachThreadInput trick
	foregroundHwnd := GetForegroundHWND()
	if foregroundHwnd == 0 {
		return fmt.Errorf("no foreground window to attach to")
	}

	var foregroundPid uint32
	foregroundThread, _, _ := procGetWindowThreadProcessId.Call(foregroundHwnd, uintptr(unsafe.Pointer(&foregroundPid)))
	currentThread, _, _ := procGetCurrentThreadId.Call()

	if foregroundThread != currentThread {
		_, _, _ = procAttachThreadInput.Call(currentThread, foregroundThread, 1) // attach
		defer func() { _, _, _ = procAttachThreadInput.Call(currentThread, foregroundThread, 0) }() // detach
	}

	ret, _, _ = procSetForegroundWindow.Call(hwnd)
	if ret != 0 {
		return nil
	}

	// Last resort
	_, _, _ = procBringWindowToTop.Call(hwnd)

	// Verify if the window actually became foreground
	if GetForegroundHWND() == hwnd {
		return nil
	}
	return fmt.Errorf("failed to bring window to foreground")
}

// GetForegroundHWND returns the handle of the currently focused window.
func GetForegroundHWND() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return hwnd
}
