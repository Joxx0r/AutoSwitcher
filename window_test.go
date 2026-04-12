//go:build windows

package main

import "testing"

func TestFilterByTitle(t *testing.T) {
	wins := []WindowInfo{
		{HWND: 1, Title: "Gmail - Google Chrome"},
		{HWND: 2, Title: "YouTube - Google Chrome"},
		{HWND: 3, Title: "Visual Studio Code"},
	}

	// Substring match
	result := filterByTitle(wins, "Gmail")
	if len(result) != 1 || result[0].HWND != 1 {
		t.Errorf("substring match: expected HWND 1, got %v", result)
	}

	// Case insensitive
	result = filterByTitle(wins, "gmail")
	if len(result) != 1 || result[0].HWND != 1 {
		t.Errorf("case insensitive: expected HWND 1, got %v", result)
	}

	// Empty pattern returns all
	result = filterByTitle(wins, "")
	if len(result) != 3 {
		t.Errorf("empty pattern: expected 3 results, got %d", len(result))
	}

	// No matches returns empty
	result = filterByTitle(wins, "Firefox")
	if len(result) != 0 {
		t.Errorf("no matches: expected 0 results, got %d", len(result))
	}

	// Multiple matches
	result = filterByTitle(wins, "Google Chrome")
	if len(result) != 2 {
		t.Errorf("multiple matches: expected 2 results, got %d", len(result))
	}
}

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
