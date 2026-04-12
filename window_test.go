//go:build windows

package main

import "testing"

func TestMatchExeName(t *testing.T) {
	tests := []struct {
		processExe string
		pattern    string
		want       bool
	}{
		// Exact match
		{"notepad.exe", "notepad.exe", true},
		{"Notepad.exe", "notepad.exe", true},
		{"notepad.exe", "Notepad.exe", true},

		// Without .exe extension
		{"notepad.exe", "notepad", true},
		{"Notepad.exe", "Notepad", true},
		{"Code.exe", "code", true},

		// Prefix match
		{"wezterm-gui.exe", "wez", true},
		{"wezterm-gui.exe", "wezterm", true},
		{"wezterm-gui.exe", "wezterm-gui", true},
		{"WindowsTerminal.exe", "windows", true},

		// No match
		{"notepad.exe", "chrome", false},
		{"notepad.exe", "note.exe", false},
		{"chrome.exe", "chromedriver", false},

		// Edge cases
		{"notepad.exe", "", false},
		{"notepad.exe", "  notepad  ", true},
	}

	for _, tt := range tests {
		got := matchExeName(tt.processExe, tt.pattern)
		if got != tt.want {
			t.Errorf("matchExeName(%q, %q) = %v, want %v", tt.processExe, tt.pattern, got, tt.want)
		}
	}
}
