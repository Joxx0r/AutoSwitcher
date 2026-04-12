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

// Function variables for testability.
var findWindowsByExe = findWindowsByExeImpl
var focusWindow = FocusWindowImpl
var getForegroundHWND = GetForegroundHWNDImpl

// isTopLevelAppWindow returns true if the window handle is a visible, titled,
// non-tool, non-owned top-level window (i.e., a normal application window).
func isTopLevelAppWindow(hwnd uintptr) bool {
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return false
	}
	owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
	if owner != 0 {
		return false
	}
	exStyle, _, _ := procGetWindowLong.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE = -20
	if exStyle&wsExToolWin != 0 {
		return false
	}
	titleLen, _, _ := procGetWindowTextLength.Call(hwnd)
	return titleLen > 0
}

func findWindowsByExeImpl(exeName string) ([]WindowInfo, error) {
	var results []WindowInfo

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		if !isTopLevelAppWindow(hwnd) {
			return 1
		}

		var pid uint32
		_, _, _ = procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 {
			return 1
		}

		fullPath := getProcessExePath(pid)
		if fullPath == "" {
			return 1
		}

		exePath := filepath.Base(fullPath)
		if matchExeName(exePath, exeName) {
			results = append(results, WindowInfo{
				HWND:    hwnd,
				PID:     pid,
				ExeName: exePath,
			})
		}

		return 1
	})

	ret, _, err := procEnumWindows.Call(cb, 0)
	if ret == 0 && err != nil && err != windows.ERROR_SUCCESS {
		return nil, fmt.Errorf("EnumWindows: %w", err)
	}

	return results, nil
}

// matchExeName checks if a process exe name matches the user-provided pattern.
// Supports: exact match ("notepad.exe"), without extension ("notepad"),
// and prefix match ("wez" matches "wezterm-gui.exe").
func matchExeName(processExe, pattern string) bool {
	processExe = strings.ToLower(processExe)
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	if pattern == "" {
		return false
	}

	// Exact match (case-insensitive)
	if processExe == pattern {
		return true
	}

	// Match without .exe extension: "notepad" matches "notepad.exe"
	if !strings.HasSuffix(pattern, ".exe") {
		if processExe == pattern+".exe" {
			return true
		}
	}

	// Prefix match: "wez" matches "wezterm-gui.exe"
	nameWithoutExt := strings.TrimSuffix(processExe, ".exe")
	return strings.HasPrefix(nameWithoutExt, pattern)
}

// getProcessExePath returns the full executable path for the given PID, or "" on failure.
func getProcessExePath(pid uint32) string {
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

	return windows.UTF16ToString(buf[:size])
}

// FocusWindow brings the given window to the foreground.
func FocusWindow(hwnd uintptr) error { return focusWindow(hwnd) }

// FocusWindowImpl is the real implementation of FocusWindow.
func FocusWindowImpl(hwnd uintptr) error {
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
	foregroundHwnd := getForegroundHWND()
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
	if getForegroundHWND() == hwnd {
		return nil
	}
	return fmt.Errorf("failed to bring window to foreground")
}

// GetForegroundHWNDImpl returns the handle of the currently focused window.
func GetForegroundHWNDImpl() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return hwnd
}
