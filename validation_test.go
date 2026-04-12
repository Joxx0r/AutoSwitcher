package main

import (
	"testing"
)

func TestValidateBinding_Valid(t *testing.T) {
	b := &Binding{
		Name:    "Test",
		ExeName: "test.exe",
		Hotkey:  HotkeyDef{Modifiers: []string{"win"}, Key: "1"},
	}
	if err := ValidateBinding(b); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateBinding_MissingName(t *testing.T) {
	b := &Binding{
		Name:    "",
		ExeName: "test.exe",
		Hotkey:  HotkeyDef{Key: "1"},
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err.Error() != "name is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBinding_MissingExe(t *testing.T) {
	b := &Binding{
		Name:    "Test",
		ExeName: "",
		Hotkey:  HotkeyDef{Key: "1"},
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for missing exe")
	}
	if err.Error() != "executable name is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBinding_MissingKey(t *testing.T) {
	b := &Binding{
		Name:    "Test",
		ExeName: "test.exe",
		Hotkey:  HotkeyDef{Key: ""},
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if err.Error() != "hotkey is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBinding_InvalidKey(t *testing.T) {
	b := &Binding{
		Name:    "Test",
		ExeName: "test.exe",
		Hotkey:  HotkeyDef{Key: "INVALID"},
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestValidateBinding_InvalidModifier(t *testing.T) {
	b := &Binding{
		Name:    "Test",
		ExeName: "test.exe",
		Hotkey:  HotkeyDef{Modifiers: []string{"banana"}, Key: "A"},
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for invalid modifier")
	}
}

func TestValidateWorkspaceBinding_Valid(t *testing.T) {
	b := &Binding{
		Name:   "Dev Workspace",
		Type:   "workspace",
		Hotkey: HotkeyDef{Modifiers: []string{"win"}, Key: "F1"},
		WorkspaceItems: []WorkspaceItem{
			{ExeName: "code.exe", LaunchCommand: "code.exe"},
			{ExeName: "wt.exe", LaunchCommand: "wt.exe"},
		},
	}
	if err := ValidateBinding(b); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateWorkspaceBinding_NoItems(t *testing.T) {
	b := &Binding{
		Name:           "Empty Workspace",
		Type:           "workspace",
		Hotkey:         HotkeyDef{Modifiers: []string{"win"}, Key: "F1"},
		WorkspaceItems: nil,
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	if err.Error() != "workspace must have at least one item" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateWorkspaceBinding_ItemMissingExe(t *testing.T) {
	b := &Binding{
		Name:   "Bad Workspace",
		Type:   "workspace",
		Hotkey: HotkeyDef{Modifiers: []string{"win"}, Key: "F1"},
		WorkspaceItems: []WorkspaceItem{
			{ExeName: "", LaunchCommand: "code.exe"},
		},
	}
	err := ValidateBinding(b)
	if err == nil {
		t.Fatal("expected error for item missing exe")
	}
	if err.Error() != "workspace item 1: executable name is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateModifiers_Valid(t *testing.T) {
	if err := ValidateModifiers("win, ctrl, alt, shift"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateModifiers_Invalid(t *testing.T) {
	err := ValidateModifiers("win, banana")
	if err == nil {
		t.Fatal("expected error for invalid modifier")
	}
}

func TestValidateBinding_ModifierAliases(t *testing.T) {
	b := &Binding{
		Name:    "Test",
		ExeName: "test.exe",
		Hotkey:  HotkeyDef{Modifiers: []string{"control", "super"}, Key: "A"},
	}
	if err := ValidateBinding(b); err != nil {
		t.Errorf("expected aliases 'control'/'super' to be accepted, got: %v", err)
	}
}

func TestValidateModifiers_Aliases(t *testing.T) {
	if err := ValidateModifiers("control, super"); err != nil {
		t.Errorf("expected aliases to be accepted, got: %v", err)
	}
}

func TestValidateModifiers_Empty(t *testing.T) {
	if err := ValidateModifiers(""); err != nil {
		t.Errorf("expected no error for empty string, got: %v", err)
	}
}
