package main

import (
	"fmt"
	"strings"
)

// Modifier constants for RegisterHotKey.
const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

// bindingState tracks the state for a binding (cycle position, toggle history).
type bindingState struct {
	lastHWND     uintptr // for cycle mode
	previousHWND uintptr // for toggle mode: window we came FROM
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

// canonicalNames maps VK codes to their preferred display name when multiple
// aliases exist (e.g., ENTER vs RETURN). This ensures deterministic output
// regardless of Go map iteration order.
var canonicalNames = map[uint32]string{
	0x0D: "ENTER",
	0x1B: "ESCAPE",
	0x2E: "DELETE",
	0x2D: "INSERT",
}

// FormatVK returns a human-readable name for a virtual key code.
func FormatVK(vk uint32) string {
	// Check canonical names first for deterministic output with aliased keys
	if name, ok := canonicalNames[vk]; ok {
		return name
	}
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

// IsSupportedVK returns true if the given virtual key code is in our supported vocabulary
// (i.e., it can round-trip through FormatVK → ParseKey without producing a hex fallback).
func IsSupportedVK(vk uint32) bool {
	// A-Z
	if vk >= 0x41 && vk <= 0x5A {
		return true
	}
	// 0-9
	if vk >= 0x30 && vk <= 0x39 {
		return true
	}
	// Function keys and named keys
	for _, code := range functionKeys {
		if code == vk {
			return true
		}
	}
	for _, code := range namedKeys {
		if code == vk {
			return true
		}
	}
	return false
}

// VKToModifierBit returns the modifier bitmask for a modifier VK code, or 0 if not a modifier.
func VKToModifierBit(vk uint32) uint32 {
	switch vk {
	case 0xA0, 0xA1, 0x10: // VK_LSHIFT, VK_RSHIFT, VK_SHIFT
		return modShift
	case 0xA2, 0xA3, 0x11: // VK_LCONTROL, VK_RCONTROL, VK_CONTROL
		return modControl
	case 0xA4, 0xA5, 0x12: // VK_LMENU, VK_RMENU, VK_MENU
		return modAlt
	case 0x5B, 0x5C: // VK_LWIN, VK_RWIN
		return modWin
	default:
		return 0
	}
}

// ModifierBitsToStrings converts a modifier bitmask to a slice of modifier name strings.
func ModifierBitsToStrings(bits uint32) []string {
	var result []string
	if bits&modWin != 0 {
		result = append(result, "win")
	}
	if bits&modControl != 0 {
		result = append(result, "ctrl")
	}
	if bits&modAlt != 0 {
		result = append(result, "alt")
	}
	if bits&modShift != 0 {
		result = append(result, "shift")
	}
	return result
}

// IsModifierVK returns true if the given virtual key code is a modifier key.
func IsModifierVK(vk uint32) bool {
	switch vk {
	case 0xA0, 0xA1: // VK_LSHIFT, VK_RSHIFT
		return true
	case 0xA2, 0xA3: // VK_LCONTROL, VK_RCONTROL
		return true
	case 0xA4, 0xA5: // VK_LMENU, VK_RMENU (Alt)
		return true
	case 0x5B, 0x5C: // VK_LWIN, VK_RWIN
		return true
	case 0x10, 0x11, 0x12: // VK_SHIFT, VK_CONTROL, VK_MENU
		return true
	}
	return false
}
