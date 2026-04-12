package main

import (
	"fmt"
	"strings"
)

var validModifiers = map[string]bool{
	"win": true, "ctrl": true, "alt": true, "shift": true,
	"control": true, "super": true, // aliases accepted by ParseModifiers
}

// ValidateBinding validates that a Binding has all required fields and valid values.
func ValidateBinding(b *Binding) error {
	if strings.TrimSpace(b.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(b.ExeName) == "" {
		return fmt.Errorf("executable name is required")
	}
	if b.Hotkey.Key == "" {
		return fmt.Errorf("hotkey is required")
	}
	if _, err := ParseKey(b.Hotkey.Key); err != nil {
		return fmt.Errorf("invalid key %q: %w", b.Hotkey.Key, err)
	}
	for _, m := range b.Hotkey.Modifiers {
		if !validModifiers[strings.ToLower(strings.TrimSpace(m))] {
			return fmt.Errorf("unknown modifier %q, valid: win, ctrl, alt, shift", m)
		}
	}
	return nil
}

// ValidateModifiers checks that all comma-separated modifier names are valid.
func ValidateModifiers(text string) error {
	parts := strings.Split(text, ",")
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !validModifiers[p] {
			return fmt.Errorf("unknown modifier %q, valid: win, ctrl, alt, shift", p)
		}
	}
	return nil
}
