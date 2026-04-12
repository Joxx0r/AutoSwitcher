//go:build windows

package main

import (
	"fmt"
	"log"
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
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
)

// hookState holds the active keyboard hook state.
var hookState struct {
	handle   uintptr
	dlgHWND  uintptr // HWND of the recording dialog, for focus checking
	callback func(vkCode uint32, isKeyDown bool) bool
}

// hookProc is a package-level callback stub for the low-level keyboard hook.
// syscall.NewCallback allocates permanent, non-reclaimable memory on Windows,
// so we create it once and reuse it across recorder sessions.
var hookProc = syscall.NewCallback(func(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	return lowLevelKeyboardProc(nCode, wParam, lParam)
})

// readHookVK reads the VkCode (first uint32) from a KBDLLHOOKSTRUCT at the given address.
// Uses RtlMoveMemory to avoid go vet's unsafeptr check on uintptr-to-unsafe.Pointer conversions.
func readHookVK(src uintptr, dst *uint32) {
	_, _, _ = procRtlMoveMemory.Call(uintptr(unsafe.Pointer(dst)), src, 4)
}

// lowLevelKeyboardProc is the raw hook callback registered with SetWindowsHookEx.
func lowLevelKeyboardProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && hookState.callback != nil {
		// Only intercept keys when the recording dialog is in the foreground
		if hookState.dlgHWND != 0 {
			fg, _, _ := procGetForegroundWindow.Call()
			if fg != hookState.dlgHWND {
				ret, _, _ := procCallNextHookEx.Call(hookState.handle, uintptr(nCode), wParam, lParam)
				return ret
			}
		}

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
// dlgHWND is the recording dialog's window handle — keys are only intercepted
// when this window is in the foreground.
func installKeyboardHook(cb func(vkCode uint32, isKeyDown bool) bool, dlgHWND uintptr) error {
	if hookState.handle != 0 {
		return fmt.Errorf("keyboard hook already installed")
	}

	hookState.callback = cb
	hookState.dlgHWND = dlgHWND

	modHandle, _, _ := procGetModuleHandle.Call(0)
	handle, _, err := procSetWindowsHookEx.Call(whKeyboardLL, hookProc, modHandle, 0)
	if handle == 0 {
		hookState.callback = nil
		hookState.dlgHWND = 0
		return fmt.Errorf("SetWindowsHookEx failed: %w", err)
	}

	hookState.handle = handle
	return nil
}

// getHeldModifiers checks the current keyboard state and returns a modifier
// bitmask for any modifiers currently held down. This seeds the recorder state
// so that modifiers already held when the dialog opens are recognized.
func getHeldModifiers() uint32 {
	var mods uint32
	// GetAsyncKeyState returns negative (high bit set) if key is currently down
	checkKey := func(vk uintptr, bit uint32) {
		ret, _, _ := procGetAsyncKeyState.Call(vk)
		if int16(ret) < 0 {
			mods |= bit
		}
	}
	checkKey(0xA0, modShift)   // VK_LSHIFT
	checkKey(0xA1, modShift)   // VK_RSHIFT
	checkKey(0xA2, modControl) // VK_LCONTROL
	checkKey(0xA3, modControl) // VK_RCONTROL
	checkKey(0xA4, modAlt)     // VK_LMENU
	checkKey(0xA5, modAlt)     // VK_RMENU
	checkKey(0x5B, modWin)     // VK_LWIN
	checkKey(0x5C, modWin)     // VK_RWIN
	return mods
}

// uninstallKeyboardHook removes the keyboard hook.
// Always clears state so the next session can retry, even if the unhook call fails.
func uninstallKeyboardHook() {
	if hookState.handle == 0 {
		return
	}
	ret, _, err := procUnhookWindowsHookEx.Call(hookState.handle)
	if ret == 0 {
		log.Printf("UnhookWindowsHookEx failed for handle %v: %v", hookState.handle, err)
	}
	// Always clear state — a stale handle is worse than a leaked hook,
	// because it permanently blocks future recorder sessions.
	hookState.handle = 0
	hookState.callback = nil
	hookState.dlgHWND = 0
}
