//go:build windows

package main

import (
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetWindowTextW = user32.NewProc("GetWindowTextW")
)

// ProcessInfo holds information about a running process with a visible window.
type ProcessInfo struct {
	ExeName string // basename, e.g. "chrome.exe"
	ExePath string // full path, e.g. "C:\Program Files\Google\Chrome\chrome.exe"
	Title   string // window title of one representative window
	PID     uint32
}

// Function variable for testability.
var discoverProcesses = discoverProcessesImpl

func discoverProcessesImpl() ([]ProcessInfo, error) {
	var results []ProcessInfo

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

		title := getWindowText(hwnd)

		results = append(results, ProcessInfo{
			ExeName: filepath.Base(fullPath),
			ExePath: fullPath,
			Title:   title,
			PID:     pid,
		})

		return 1
	})

	ret, _, err := procEnumWindows.Call(cb, 0)
	if ret == 0 && err != nil && err != windows.ERROR_SUCCESS {
		return nil, err
	}

	return results, nil
}

func getWindowText(hwnd uintptr) string {
	titleLen, _, _ := procGetWindowTextLength.Call(hwnd)
	if titleLen == 0 {
		return ""
	}
	buf := make([]uint16, titleLen+1)
	_, _, _ = procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), titleLen+1)
	return windows.UTF16ToString(buf)
}

// deduplicateProcesses groups by exe basename (case-insensitive), matching
// how runtime hotkey resolution identifies processes. Keeps the entry with
// the longest window title as representative.
func deduplicateProcesses(procs []ProcessInfo) []ProcessInfo {
	type entry struct {
		proc     ProcessInfo
		titleLen int
	}
	seen := make(map[string]*entry)

	for _, p := range procs {
		key := strings.ToLower(p.ExeName)
		if e, ok := seen[key]; !ok {
			seen[key] = &entry{proc: p, titleLen: len(p.Title)}
		} else if len(p.Title) > e.titleLen {
			e.proc = p
			e.titleLen = len(p.Title)
		}
	}

	result := make([]ProcessInfo, 0, len(seen))
	for _, e := range seen {
		result = append(result, e.proc)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := strings.ToLower(result[i].ExeName), strings.ToLower(result[j].ExeName)
		if a != b {
			return a < b
		}
		return strings.ToLower(result[i].ExePath) < strings.ToLower(result[j].ExePath)
	})
	return result
}

// filterProcesses returns processes whose exe name, title, or path contains the query (case-insensitive).
// An empty query returns all processes.
func filterProcesses(procs []ProcessInfo, query string) []ProcessInfo {
	query = strings.TrimSpace(query)
	if query == "" {
		return procs
	}
	q := strings.ToLower(query)
	var result []ProcessInfo
	for _, p := range procs {
		if strings.Contains(strings.ToLower(p.ExeName), q) ||
			strings.Contains(strings.ToLower(p.Title), q) ||
			strings.Contains(strings.ToLower(p.ExePath), q) {
			result = append(result, p)
		}
	}
	return result
}
