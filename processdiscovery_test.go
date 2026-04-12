//go:build windows

package main

import "testing"

func TestDeduplicateProcesses(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", Title: "Tab 1", PID: 100},
		{ExeName: "chrome.exe", Title: "Tab 2", PID: 101},
		{ExeName: "notepad.exe", Title: "Untitled", PID: 200},
		{ExeName: "Chrome.exe", Title: "Tab 3", PID: 102}, // different case
	}

	result := deduplicateProcesses(procs)
	if len(result) != 2 {
		t.Fatalf("expected 2 unique processes, got %d", len(result))
	}
	// First occurrence should be kept
	if result[0].ExeName != "chrome.exe" || result[0].PID != 100 {
		t.Errorf("expected first chrome.exe (PID 100), got %v", result[0])
	}
	if result[1].ExeName != "notepad.exe" {
		t.Errorf("expected notepad.exe, got %v", result[1])
	}
}

func TestFilterProcesses(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", Title: "Gmail - Google Chrome"},
		{ExeName: "notepad.exe", Title: "Untitled - Notepad"},
		{ExeName: "code.exe", Title: "main.go - Visual Studio Code"},
	}

	// Match by ExeName
	result := filterProcesses(procs, "chrome")
	if len(result) != 1 || result[0].ExeName != "chrome.exe" {
		t.Errorf("filter by exe: expected chrome.exe, got %v", result)
	}

	// Match by Title
	result = filterProcesses(procs, "Gmail")
	if len(result) != 1 || result[0].ExeName != "chrome.exe" {
		t.Errorf("filter by title: expected chrome.exe, got %v", result)
	}

	// Case insensitive
	result = filterProcesses(procs, "NOTEPAD")
	if len(result) != 1 || result[0].ExeName != "notepad.exe" {
		t.Errorf("case insensitive: expected notepad.exe, got %v", result)
	}

	// Empty query returns all
	result = filterProcesses(procs, "")
	if len(result) != 3 {
		t.Errorf("empty query: expected 3, got %d", len(result))
	}

	// No matches
	result = filterProcesses(procs, "firefox")
	if len(result) != 0 {
		t.Errorf("no matches: expected 0, got %d", len(result))
	}

	// Multiple matches
	result = filterProcesses(procs, "exe")
	if len(result) != 3 {
		t.Errorf("multiple matches: expected 3, got %d", len(result))
	}
}
