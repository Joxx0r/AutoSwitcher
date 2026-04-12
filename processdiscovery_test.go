package main

import (
	"testing"
)

func TestDeduplicateProcesses_GroupsByExeName(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", ExePath: `C:\Chrome\chrome.exe`, Title: "Tab 1", PID: 100},
		{ExeName: "chrome.exe", ExePath: `C:\Chrome\chrome.exe`, Title: "A much longer tab title here", PID: 101},
		{ExeName: "notepad.exe", ExePath: `C:\Windows\notepad.exe`, Title: "Untitled", PID: 200},
	}

	result := deduplicateProcesses(procs)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}

	// Should be sorted alphabetically
	if result[0].ExeName != "chrome.exe" {
		t.Errorf("expected chrome.exe first, got %s", result[0].ExeName)
	}
	if result[1].ExeName != "notepad.exe" {
		t.Errorf("expected notepad.exe second, got %s", result[1].ExeName)
	}

	// Chrome entry should have the longest title
	if result[0].Title != "A much longer tab title here" {
		t.Errorf("expected longest title as representative, got %q", result[0].Title)
	}
}

func TestDeduplicateProcesses_CaseInsensitive(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "Chrome.exe", ExePath: `C:\Chrome\chrome.exe`, Title: "Tab 1", PID: 100},
		{ExeName: "chrome.exe", ExePath: `C:\Chrome\chrome.exe`, Title: "Longer title wins", PID: 101},
	}

	result := deduplicateProcesses(procs)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry after dedup, got %d", len(result))
	}
	if result[0].Title != "Longer title wins" {
		t.Errorf("expected longest title, got %q", result[0].Title)
	}
}

func TestDeduplicateProcesses_Empty(t *testing.T) {
	result := deduplicateProcesses(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestFilterProcesses_ByExeName(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", Title: "Google"},
		{ExeName: "notepad.exe", Title: "Untitled"},
		{ExeName: "firefox.exe", Title: "Mozilla"},
	}

	result := filterProcesses(procs, "chrome")
	if len(result) != 1 || result[0].ExeName != "chrome.exe" {
		t.Errorf("expected chrome.exe, got %v", result)
	}
}

func TestFilterProcesses_ByTitle(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", Title: "Google Search"},
		{ExeName: "notepad.exe", Title: "Untitled"},
	}

	result := filterProcesses(procs, "google")
	if len(result) != 1 || result[0].ExeName != "chrome.exe" {
		t.Errorf("expected chrome.exe matched by title, got %v", result)
	}
}

func TestFilterProcesses_CaseInsensitive(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "Chrome.exe", Title: "GOOGLE"},
	}

	result := filterProcesses(procs, "CHROME")
	if len(result) != 1 {
		t.Errorf("expected 1 result for case-insensitive match, got %d", len(result))
	}

	result = filterProcesses(procs, "google")
	if len(result) != 1 {
		t.Errorf("expected 1 result for case-insensitive title match, got %d", len(result))
	}
}

func TestFilterProcesses_EmptyQuery(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", Title: "Google"},
		{ExeName: "notepad.exe", Title: "Untitled"},
	}

	result := filterProcesses(procs, "")
	if len(result) != 2 {
		t.Errorf("empty query should return all, got %d", len(result))
	}
}

func TestFilterProcesses_NoMatch(t *testing.T) {
	procs := []ProcessInfo{
		{ExeName: "chrome.exe", Title: "Google"},
	}

	result := filterProcesses(procs, "nonexistent")
	if len(result) != 0 {
		t.Errorf("expected no matches, got %d", len(result))
	}
}
