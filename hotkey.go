//go:build windows

package main

import (
	"fmt"
	"log"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
)

// Modifier constants for RegisterHotKey.
const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

// cycleInfo tracks the cycle state for a binding.
type cycleInfo struct {
	lastHWND uintptr
}

// HotkeyManager handles registration and dispatch of global hotkeys.
type HotkeyManager struct {
	bindings   map[int32]*Binding
	nextID     int32
	cycleState map[string]cycleInfo
	hwnd       uintptr
	showBalloon func(title, msg string)
}

// NewHotkeyManager creates a new hotkey manager.
func NewHotkeyManager(hwnd uintptr, showBalloon func(title, msg string)) *HotkeyManager {
	return &HotkeyManager{
		bindings:    make(map[int32]*Binding),
		nextID:      1,
		cycleState:  make(map[string]cycleInfo),
		hwnd:        hwnd,
		showBalloon: showBalloon,
	}
}

// RegisterAll registers hotkeys for all bindings. Returns errors for any that fail.
// Successfully registered hotkeys remain active even if others fail.
func (hm *HotkeyManager) RegisterAll(bindings []Binding) []error {
	var errs []error
	for i := range bindings {
		b := &bindings[i]
		vk, err := ParseKey(b.Hotkey.Key)
		if err != nil {
			errMsg := fmt.Sprintf("Invalid key %q for %s: %v", b.Hotkey.Key, b.Name, err)
			log.Println(errMsg)
			if hm.showBalloon != nil {
				hm.showBalloon("Hotkey Error", errMsg)
			}
			errs = append(errs, fmt.Errorf("%s", errMsg))
			continue
		}

		mods := ParseModifiers(b.Hotkey.Modifiers)
		id := hm.nextID
		hm.nextID++

		ret, _, _ := procRegisterHotKey.Call(hm.hwnd, uintptr(id), uintptr(mods), uintptr(vk))
		if ret == 0 {
			errMsg := fmt.Sprintf("Hotkey %s is already in use by another application", b.Hotkey.Format())
			log.Println(errMsg)
			if hm.showBalloon != nil {
				hm.showBalloon("Hotkey Conflict", errMsg)
			}
			errs = append(errs, fmt.Errorf("%s", errMsg))
			continue
		}

		hm.bindings[int32(id)] = b
		log.Printf("Registered hotkey %s (id=%d) for %s", b.Hotkey.Format(), id, b.Name)
	}
	return errs
}

// UnregisterAll unregisters all currently registered hotkeys.
func (hm *HotkeyManager) UnregisterAll() {
	for id := range hm.bindings {
		procUnregisterHotKey.Call(hm.hwnd, uintptr(id))
	}
	hm.bindings = make(map[int32]*Binding)
	hm.cycleState = make(map[string]cycleInfo)
	log.Println("Unregistered all hotkeys")
}

// HandleHotkey processes a WM_HOTKEY message.
func (hm *HotkeyManager) HandleHotkey(id int32) {
	binding, ok := hm.bindings[id]
	if !ok {
		return
	}

	wins, err := findWindowsByExe(binding.ExeName)
	if err != nil {
		log.Printf("Error finding windows for %s: %v", binding.ExeName, err)
		return
	}

	if len(wins) == 0 {
		// No matching windows — try to launch
		if binding.LaunchCommand != "" {
			log.Printf("No window for %s, launching: %s", binding.Name, binding.LaunchCommand)
			if err := LaunchApp(binding.LaunchCommand, binding.LaunchArgs); err != nil {
				errMsg := fmt.Sprintf("Failed to launch %s: %v", binding.Name, err)
				log.Println(errMsg)
				if hm.showBalloon != nil {
					hm.showBalloon("Launch Failed", errMsg)
				}
			}
		} else {
			msg := fmt.Sprintf("No window found for %s", binding.Name)
			log.Println(msg)
			if hm.showBalloon != nil {
				hm.showBalloon("AutoSwitcher", msg)
			}
		}
		return
	}

	foreground := GetForegroundHWND()

	switch binding.MultiWindow {
	case "cycle":
		hm.handleCycle(binding, wins, foreground)
	default: // "most_recent" or unset
		hm.handleMostRecent(binding, wins, foreground)
	}
}

func (hm *HotkeyManager) handleMostRecent(binding *Binding, wins []WindowInfo, foreground uintptr) {
	// If the foreground window is already one of our matches, do nothing
	for _, w := range wins {
		if w.HWND == foreground {
			return
		}
	}

	// Focus the first (topmost Z-order) match
	if err := FocusWindow(wins[0].HWND); err != nil {
		log.Printf("Failed to focus %s: %v", binding.Name, err)
		if hm.showBalloon != nil {
			hm.showBalloon("Focus Failed", fmt.Sprintf("Could not focus %s: %v", binding.Name, err))
		}
	}
}

func (hm *HotkeyManager) handleCycle(binding *Binding, wins []WindowInfo, foreground uintptr) {
	state := hm.cycleState[binding.ExeName]

	// Find the index of the foreground window in our list
	foregroundIdx := -1
	for i, w := range wins {
		if w.HWND == foreground {
			foregroundIdx = i
			break
		}
	}

	var targetIdx int
	if foregroundIdx >= 0 {
		// Foreground is one of ours — cycle to next
		targetIdx = (foregroundIdx + 1) % len(wins)
	} else {
		// Foreground is not ours — try to resume from last known HWND
		lastIdx := -1
		for i, w := range wins {
			if w.HWND == state.lastHWND {
				lastIdx = i
				break
			}
		}
		if lastIdx >= 0 {
			targetIdx = lastIdx
		} else {
			targetIdx = 0 // start from beginning
		}
	}

	target := wins[targetIdx]
	hm.cycleState[binding.ExeName] = cycleInfo{lastHWND: target.HWND}

	if err := FocusWindow(target.HWND); err != nil {
		log.Printf("Failed to focus %s (cycle): %v", binding.Name, err)
		if hm.showBalloon != nil {
			hm.showBalloon("Focus Failed", fmt.Sprintf("Could not focus %s: %v", binding.Name, err))
		}
	}
}

// ParseKey converts a key name string to a Windows virtual key code.
func ParseKey(key string) (uint32, error) {
	upper := strings.ToUpper(strings.TrimSpace(key))

	// Single character A-Z
	if len(upper) == 1 && upper[0] >= 'A' && upper[0] <= 'Z' {
		return uint32(upper[0]), nil
	}

	// Single digit 0-9
	if len(upper) == 1 && upper[0] >= '0' && upper[0] <= '9' {
		return uint32(upper[0]), nil
	}

	// Function keys
	if vk, ok := functionKeys[upper]; ok {
		return vk, nil
	}

	// Named keys
	if vk, ok := namedKeys[upper]; ok {
		return vk, nil
	}

	return 0, fmt.Errorf("unknown key: %q", key)
}

// ParseModifiers converts modifier name strings to a combined modifier bitmask.
func ParseModifiers(mods []string) uint32 {
	var result uint32
	for _, m := range mods {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case "alt":
			result |= modAlt
		case "ctrl", "control":
			result |= modControl
		case "shift":
			result |= modShift
		case "win", "super":
			result |= modWin
		}
	}
	result |= modNoRepeat
	return result
}

// FormatVK returns a human-readable name for a virtual key code.
func FormatVK(vk uint32) string {
	for name, code := range functionKeys {
		if code == vk {
			return name
		}
	}
	for name, code := range namedKeys {
		if code == vk {
			return name
		}
	}
	if vk >= 0x30 && vk <= 0x39 {
		return string(rune(vk))
	}
	if vk >= 0x41 && vk <= 0x5A {
		return string(rune(vk))
	}
	return fmt.Sprintf("0x%02X", vk)
}

// FormatModifiers returns a human-readable string for a modifier bitmask.
func FormatModifiers(mods uint32) string {
	var parts []string
	if mods&modWin != 0 {
		parts = append(parts, "Win")
	}
	if mods&modControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&modShift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(parts, "+")
}

var functionKeys = map[string]uint32{
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73,
	"F5": 0x74, "F6": 0x75, "F7": 0x76, "F8": 0x77,
	"F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,
	"F13": 0x7C, "F14": 0x7D, "F15": 0x7E, "F16": 0x7F,
	"F17": 0x80, "F18": 0x81, "F19": 0x82, "F20": 0x83,
	"F21": 0x84, "F22": 0x85, "F23": 0x86, "F24": 0x87,
}

var namedKeys = map[string]uint32{
	"SPACE":     0x20,
	"ENTER":     0x0D,
	"RETURN":    0x0D,
	"TAB":       0x09,
	"ESCAPE":    0x1B,
	"ESC":       0x1B,
	"BACKSPACE": 0x08,
	"DELETE":    0x2E,
	"DEL":       0x2E,
	"INSERT":    0x2D,
	"INS":       0x2D,
	"HOME":      0x24,
	"END":       0x23,
	"PAGEUP":    0x21,
	"PAGEDOWN":  0x22,
	"UP":        0x26,
	"DOWN":      0x28,
	"LEFT":      0x25,
	"RIGHT":     0x27,
	"NUMPAD0":   0x60, "NUMPAD1": 0x61, "NUMPAD2": 0x62, "NUMPAD3": 0x63,
	"NUMPAD4":   0x64, "NUMPAD5": 0x65, "NUMPAD6": 0x66, "NUMPAD7": 0x67,
	"NUMPAD8":   0x68, "NUMPAD9": 0x69,
}

// vkToModString maps virtual key codes to modifier strings (for hotkey recording).
var vkToModString = map[uint32]string{
	0xA0: "shift", 0xA1: "shift", // VK_LSHIFT, VK_RSHIFT
	0xA2: "ctrl", 0xA3: "ctrl",   // VK_LCONTROL, VK_RCONTROL
	0xA4: "alt", 0xA5: "alt",     // VK_LMENU, VK_RMENU
	0x5B: "win", 0x5C: "win",     // VK_LWIN, VK_RWIN
}

// IsModifierVK returns true if the virtual key code is a modifier key.
func IsModifierVK(vk uint32) bool {
	_, ok := vkToModString[vk]
	return ok
}

// RegisterWindowMessage registers a custom window message.
func RegisterWindowMessageW(name string) uint32 {
	ptr, _ := windows.UTF16PtrFromString(name)
	ret, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(ptr)))
	return uint32(ret)
}
