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

func TestValidateModifiers_Empty(t *testing.T) {
	if err := ValidateModifiers(""); err != nil {
		t.Errorf("expected no error for empty string, got: %v", err)
	}
}
