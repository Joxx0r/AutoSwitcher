//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessInfo holds information about a running process with a visible window.
type ProcessInfo struct {
	ExeName string
	ExePath string
	Title   string
	PID     uint32
}

// Function variable for testability.
var discoverProcesses = discoverProcessesImpl

// discoverProcessesImpl enumerates all visible top-level windows and collects process info.
func discoverProcessesImpl() []ProcessInfo {
	var results []ProcessInfo

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		// Skip invisible windows
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}

		// Skip owned windows
		owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
		if owner != 0 {
			return 1
		}

		// Skip tool windows
		exStyle, _, _ := procGetWindowLong.Call(hwnd, ^uintptr(19)) // GWL_EXSTYLE = -20
		if exStyle&wsExToolWin != 0 {
			return 1
		}

		// Skip windows with no title
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

		// Get executable path
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			return 1
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
			return 1
		}

		exePath := windows.UTF16ToString(buf[:size])
		exeName := filepath.Base(exePath)
		title := getWindowText(hwnd)

		results = append(results, ProcessInfo{
			ExeName: exeName,
			ExePath: exePath,
			Title:   title,
			PID:     pid,
		})

		return 1
	})

	ret, _, err := procEnumWindows.Call(cb, 0)
	if ret == 0 && err != nil && err != windows.ERROR_SUCCESS {
		return nil
	}

	return results
}

// deduplicateProcesses removes duplicate entries by ExeName, keeping the first
// occurrence (which has the topmost window in Z-order).
func deduplicateProcesses(procs []ProcessInfo) []ProcessInfo {
	seen := make(map[string]bool)
	var result []ProcessInfo
	for _, p := range procs {
		key := strings.ToLower(p.ExeName)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}
	return result
}

// filterProcesses returns processes where ExeName or Title contains the query
// (case-insensitive substring match).
func filterProcesses(procs []ProcessInfo, query string) []ProcessInfo {
	if query == "" {
		return procs
	}
	lowerQuery := strings.ToLower(query)
	var result []ProcessInfo
	for _, p := range procs {
		if strings.Contains(strings.ToLower(p.ExeName), lowerQuery) ||
			strings.Contains(strings.ToLower(p.Title), lowerQuery) {
			result = append(result, p)
		}
	}
	return result
}
