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

func TestIsModifierVK(t *testing.T) {
	if !IsModifierVK(0x5B) { // VK_LWIN
		t.Error("expected VK_LWIN to be a modifier")
	}
	if !IsModifierVK(0xA2) { // VK_LCONTROL
		t.Error("expected VK_LCONTROL to be a modifier")
	}
	if IsModifierVK(0x41) { // 'A'
		t.Error("expected 'A' to NOT be a modifier")
	}
}
