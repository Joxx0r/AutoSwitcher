package main

import (
	"testing"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		input   string
		wantVK  uint32
		wantErr bool
	}{
		// Single letters
		{"A", 0x41, false},
		{"z", 0x5A, false},
		{"m", 0x4D, false},

		// Digits
		{"0", 0x30, false},
		{"1", 0x31, false},
		{"9", 0x39, false},

		// Function keys
		{"F1", 0x70, false},
		{"F5", 0x74, false},
		{"F12", 0x7B, false},
		{"F13", 0x7C, false},
		{"F24", 0x87, false},
		{"f5", 0x74, false}, // case insensitive

		// Named keys
		{"SPACE", 0x20, false},
		{"ENTER", 0x0D, false},
		{"ESCAPE", 0x1B, false},
		{"ESC", 0x1B, false},
		{"TAB", 0x09, false},
		{"DELETE", 0x2E, false},

		// Errors
		{"", 0, true},
		{"INVALID", 0, true},
		{"F25", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseKey(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseKey(%q) expected error, got %d", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseKey(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.wantVK {
			t.Errorf("ParseKey(%q) = 0x%02X, want 0x%02X", tt.input, got, tt.wantVK)
		}
	}
}

func TestParseModifiers(t *testing.T) {
	tests := []struct {
		input []string
		want  uint32
	}{
		{nil, modNoRepeat},
		{[]string{}, modNoRepeat},
		{[]string{"win"}, modWin | modNoRepeat},
		{[]string{"ctrl"}, modControl | modNoRepeat},
		{[]string{"alt"}, modAlt | modNoRepeat},
		{[]string{"shift"}, modShift | modNoRepeat},
		{[]string{"ctrl", "alt"}, modControl | modAlt | modNoRepeat},
		{[]string{"Win", "Shift"}, modWin | modShift | modNoRepeat},
		{[]string{"CTRL", "ALT", "SHIFT", "WIN"}, modControl | modAlt | modShift | modWin | modNoRepeat},
		{[]string{"control"}, modControl | modNoRepeat}, // alias
		{[]string{"super"}, modWin | modNoRepeat},       // alias
	}

	for _, tt := range tests {
		got := ParseModifiers(tt.input)
		if got != tt.want {
			t.Errorf("ParseModifiers(%v) = 0x%04X, want 0x%04X", tt.input, got, tt.want)
		}
	}
}

func TestFormatVK(t *testing.T) {
	tests := []struct {
		vk   uint32
		want string
	}{
		{0x41, "A"},
		{0x31, "1"},
		{0x74, "F5"},
		{0x7B, "F12"},
	}

	for _, tt := range tests {
		got := FormatVK(tt.vk)
		if got != tt.want {
			t.Errorf("FormatVK(0x%02X) = %q, want %q", tt.vk, got, tt.want)
		}
	}
}

func TestFormatModifiers(t *testing.T) {
	tests := []struct {
		mods uint32
		want string
	}{
		{modWin, "Win"},
		{modControl | modAlt, "Ctrl+Alt"},
		{modWin | modShift, "Win+Shift"},
		{0, ""},
	}

	for _, tt := range tests {
		got := FormatModifiers(tt.mods)
		if got != tt.want {
			t.Errorf("FormatModifiers(0x%04X) = %q, want %q", tt.mods, got, tt.want)
		}
	}
}

func TestFormatVKCanonicalAliases(t *testing.T) {
	// Keys with aliases must always produce the canonical (preferred) name
	tests := []struct {
		vk   uint32
		want string
	}{
		{0x0D, "ENTER"},  // not RETURN
		{0x1B, "ESCAPE"}, // not ESC
		{0x2E, "DELETE"}, // not DEL
		{0x2D, "INSERT"}, // not INS
	}

	for _, tt := range tests {
		got := FormatVK(tt.vk)
		if got != tt.want {
			t.Errorf("FormatVK(0x%02X) = %q, want canonical %q", tt.vk, got, tt.want)
		}
	}
}

func TestIsSupportedVK(t *testing.T) {
	tests := []struct {
		vk   uint32
		want bool
	}{
		{0x41, true},  // A
		{0x5A, true},  // Z
		{0x30, true},  // 0
		{0x39, true},  // 9
		{0x70, true},  // F1
		{0x87, true},  // F24
		{0x20, true},  // SPACE
		{0x0D, true},  // ENTER
		{0x1B, true},  // ESCAPE
		{0x09, true},  // TAB
		{0x26, true},  // UP arrow
		{0x60, true},  // NUMPAD0
		{0xBA, false}, // OEM_1 (semicolon) — not in our vocabulary
		{0xBB, false}, // OEM_PLUS — not in our vocabulary
		{0x01, false}, // VK_LBUTTON — not supported
		{0xA0, false}, // VK_LSHIFT — modifier, not a target key
	}

	for _, tt := range tests {
		got := IsSupportedVK(tt.vk)
		if got != tt.want {
			t.Errorf("IsSupportedVK(0x%02X) = %v, want %v", tt.vk, got, tt.want)
		}
	}
}

func TestSupportedVKRoundTrip(t *testing.T) {
	// Every supported VK should produce a name that ParseKey can resolve back
	testVKs := []uint32{
		0x41, 0x5A, 0x30, 0x39, // A, Z, 0, 9
		0x70, 0x7B, 0x87,       // F1, F12, F24
		0x20, 0x0D, 0x1B, 0x09, // SPACE, ENTER, ESCAPE, TAB
		0x26, 0x28, 0x25, 0x27, // arrows
		0x60, 0x69,             // NUMPAD0, NUMPAD9
	}
	for _, vk := range testVKs {
		name := FormatVK(vk)
		parsed, err := ParseKey(name)
		if err != nil {
			t.Errorf("FormatVK(0x%02X) = %q, but ParseKey returned error: %v", vk, name, err)
			continue
		}
		if parsed != vk {
			t.Errorf("Round-trip failed: VK 0x%02X → %q → 0x%02X", vk, name, parsed)
		}
	}
}

func TestVKToModifierBit(t *testing.T) {
	tests := []struct {
		vk   uint32
		want uint32
	}{
		{0xA0, modShift},   // VK_LSHIFT
		{0xA1, modShift},   // VK_RSHIFT
		{0x10, modShift},   // VK_SHIFT
		{0xA2, modControl}, // VK_LCONTROL
		{0xA3, modControl}, // VK_RCONTROL
		{0x11, modControl}, // VK_CONTROL
		{0xA4, modAlt},     // VK_LMENU
		{0xA5, modAlt},     // VK_RMENU
		{0x12, modAlt},     // VK_MENU
		{0x5B, modWin},     // VK_LWIN
		{0x5C, modWin},     // VK_RWIN
		{0x41, 0},          // A — not a modifier
		{0x74, 0},          // F5 — not a modifier
	}

	for _, tt := range tests {
		got := VKToModifierBit(tt.vk)
		if got != tt.want {
			t.Errorf("VKToModifierBit(0x%02X) = 0x%04X, want 0x%04X", tt.vk, got, tt.want)
		}
	}
}

func TestModifierBitsToStrings(t *testing.T) {
	tests := []struct {
		bits uint32
		want []string
	}{
		{0, nil},
		{modWin, []string{"win"}},
		{modControl, []string{"ctrl"}},
		{modAlt, []string{"alt"}},
		{modShift, []string{"shift"}},
		{modWin | modControl, []string{"win", "ctrl"}},
		{modWin | modControl | modAlt | modShift, []string{"win", "ctrl", "alt", "shift"}},
	}

	for _, tt := range tests {
		got := ModifierBitsToStrings(tt.bits)
		if len(got) != len(tt.want) {
			t.Errorf("ModifierBitsToStrings(0x%04X) = %v, want %v", tt.bits, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ModifierBitsToStrings(0x%04X)[%d] = %q, want %q", tt.bits, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsModifierVK(t *testing.T) {
	tests := []struct {
		vk   uint32
		want bool
	}{
		{0xA0, true},  // VK_LSHIFT
		{0xA1, true},  // VK_RSHIFT
		{0xA2, true},  // VK_LCONTROL
		{0xA3, true},  // VK_RCONTROL
		{0xA4, true},  // VK_LMENU
		{0xA5, true},  // VK_RMENU
		{0x5B, true},  // VK_LWIN
		{0x5C, true},  // VK_RWIN
		{0x10, true},  // VK_SHIFT
		{0x11, true},  // VK_CONTROL
		{0x12, true},  // VK_MENU
		{0x41, false}, // A
		{0x74, false}, // F5
	}

	for _, tt := range tests {
		got := IsModifierVK(tt.vk)
		if got != tt.want {
			t.Errorf("IsModifierVK(0x%02X) = %v, want %v", tt.vk, got, tt.want)
		}
	}
}
