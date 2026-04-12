//go:build windows

package main

import "testing"

func TestBindingModel_Empty(t *testing.T) {
	model := NewBindingModel(nil)
	if model.RowCount() != 0 {
		t.Errorf("expected 0 rows, got %d", model.RowCount())
	}
}

func TestBindingModel_RowCount(t *testing.T) {
	bindings := []Binding{
		{Name: "A", Hotkey: HotkeyDef{Modifiers: []string{"win"}, Key: "1"}, ExeName: "a.exe", MultiWindow: "most_recent"},
		{Name: "B", Hotkey: HotkeyDef{Modifiers: []string{"ctrl", "alt"}, Key: "F5"}, ExeName: "b.exe", MultiWindow: "cycle"},
	}
	model := NewBindingModel(bindings)
	if model.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", model.RowCount())
	}
}

func TestBindingModel_Value(t *testing.T) {
	bindings := []Binding{
		{Name: "VS Code", Hotkey: HotkeyDef{Modifiers: []string{"win"}, Key: "1"}, ExeName: "Code.exe", MultiWindow: "most_recent"},
		{Name: "Terminal", Hotkey: HotkeyDef{Modifiers: []string{"ctrl", "alt"}, Key: "t"}, ExeName: "WindowsTerminal.exe", MultiWindow: "cycle"},
	}
	model := NewBindingModel(bindings)

	// Row 0
	if v := model.Value(0, 0); v != "VS Code" {
		t.Errorf("row 0, col 0: expected 'VS Code', got %v", v)
	}
	if v := model.Value(0, 1); v != "Win+1" {
		t.Errorf("row 0, col 1: expected 'Win+1', got %v", v)
	}
	if v := model.Value(0, 2); v != "Code.exe" {
		t.Errorf("row 0, col 2: expected 'Code.exe', got %v", v)
	}
	if v := model.Value(0, 3); v != "Most Recent" {
		t.Errorf("row 0, col 3: expected 'Most Recent', got %v", v)
	}

	// Row 1
	if v := model.Value(1, 0); v != "Terminal" {
		t.Errorf("row 1, col 0: expected 'Terminal', got %v", v)
	}
	if v := model.Value(1, 1); v != "Ctrl+Alt+T" {
		t.Errorf("row 1, col 1: expected 'Ctrl+Alt+T', got %v", v)
	}
	if v := model.Value(1, 2); v != "WindowsTerminal.exe" {
		t.Errorf("row 1, col 2: expected 'WindowsTerminal.exe', got %v", v)
	}
	if v := model.Value(1, 3); v != "Cycle" {
		t.Errorf("row 1, col 3: expected 'Cycle', got %v", v)
	}
}

func TestBindingModel_ValueOutOfBounds(t *testing.T) {
	model := NewBindingModel([]Binding{{Name: "X", Hotkey: HotkeyDef{Key: "A"}, ExeName: "x.exe"}})

	// Negative row
	if v := model.Value(-1, 0); v != "" {
		t.Errorf("negative row should return empty, got %v", v)
	}
	// Row beyond length
	if v := model.Value(5, 0); v != "" {
		t.Errorf("out of bounds row should return empty, got %v", v)
	}
	// Invalid column
	if v := model.Value(0, 99); v != "" {
		t.Errorf("invalid column should return empty, got %v", v)
	}
}

func TestBindingModel_UpdateFrom(t *testing.T) {
	bindings := []Binding{
		{Name: "A", Hotkey: HotkeyDef{Key: "1"}, ExeName: "a.exe"},
	}
	model := NewBindingModel(bindings)

	if model.RowCount() != 1 {
		t.Fatalf("expected 1 row initially, got %d", model.RowCount())
	}

	// Simulate adding a binding
	newBindings := []Binding{
		{Name: "A", Hotkey: HotkeyDef{Key: "1"}, ExeName: "a.exe"},
		{Name: "B", Hotkey: HotkeyDef{Modifiers: []string{"win"}, Key: "2"}, ExeName: "b.exe", MultiWindow: "cycle"},
	}
	model.updateFrom(newBindings)

	if model.RowCount() != 2 {
		t.Fatalf("expected 2 rows after update, got %d", model.RowCount())
	}
	if v := model.Value(1, 0); v != "B" {
		t.Errorf("row 1 name should be 'B', got %v", v)
	}
	if v := model.Value(1, 1); v != "Win+2" {
		t.Errorf("row 1 hotkey should be 'Win+2', got %v", v)
	}
	if v := model.Value(1, 3); v != "Cycle" {
		t.Errorf("row 1 multi-window should be 'Cycle', got %v", v)
	}
}

func TestBindingModel_UpdateFromEmpty(t *testing.T) {
	bindings := []Binding{
		{Name: "A", Hotkey: HotkeyDef{Key: "1"}, ExeName: "a.exe"},
		{Name: "B", Hotkey: HotkeyDef{Key: "2"}, ExeName: "b.exe"},
	}
	model := NewBindingModel(bindings)

	// Clear all bindings
	model.updateFrom(nil)
	if model.RowCount() != 0 {
		t.Errorf("expected 0 rows after clearing, got %d", model.RowCount())
	}
}
