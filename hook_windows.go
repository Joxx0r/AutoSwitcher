//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
)

var (
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procRtlMoveMemory       = kernel32.NewProc("RtlMoveMemory")
)

// hookState holds the active keyboard hook state.
var hookState struct {
	handle   uintptr
	callback func(vkCode uint32, isKeyDown bool) bool
}

// readHookVK reads the VkCode (first uint32) from a KBDLLHOOKSTRUCT at the given address.
// Uses RtlMoveMemory to avoid go vet's unsafeptr check on uintptr-to-unsafe.Pointer conversions.
func readHookVK(src uintptr, dst *uint32) {
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(dst)), src, 4)
}

// lowLevelKeyboardProc is the raw hook callback registered with SetWindowsHookEx.
func lowLevelKeyboardProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && hookState.callback != nil {
		// lParam points to a KBDLLHOOKSTRUCT; VkCode is the first uint32 field.
		// lParam points to KBDLLHOOKSTRUCT; VkCode is the first uint32 field.
		var vkCode uint32
		readHookVK(lParam, &vkCode)
		isDown := wParam == wmKeyDown || wParam == wmSysKeyDown
		if hookState.callback(vkCode, isDown) {
			return 1 // suppress the key
		}
	}
	ret, _, _ := procCallNextHookEx.Call(hookState.handle, uintptr(nCode), wParam, lParam)
	return ret
}

// installKeyboardHook installs a temporary WH_KEYBOARD_LL hook.
// The callback receives the VK code and whether it's a key-down event.
// Return true from the callback to suppress the key.
func installKeyboardHook(cb func(vkCode uint32, isKeyDown bool) bool) error {
	if hookState.handle != 0 {
		return fmt.Errorf("keyboard hook already installed")
	}

	hookState.callback = cb
	hookProc := syscall.NewCallback(func(nCode int32, wParam uintptr, lParam uintptr) uintptr {
		return lowLevelKeyboardProc(nCode, wParam, lParam)
	})

	modHandle, _, _ := procGetModuleHandle.Call(0)
	handle, _, err := procSetWindowsHookEx.Call(whKeyboardLL, hookProc, modHandle, 0)
	if handle == 0 {
		hookState.callback = nil
		return fmt.Errorf("SetWindowsHookEx failed: %w", err)
	}

	hookState.handle = handle
	return nil
}

// uninstallKeyboardHook removes the keyboard hook.
func uninstallKeyboardHook() {
	if hookState.handle != 0 {
		procUnhookWindowsHookEx.Call(hookState.handle)
		hookState.handle = 0
		hookState.callback = nil
	}
}
