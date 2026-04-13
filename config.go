package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	Version   int       `json:"version"`
	Autostart bool      `json:"autostart"`
	Bindings  []Binding `json:"bindings"`
}

// Binding represents a single hotkey-to-window mapping.
type Binding struct {
	Name          string    `json:"name"`
	Hotkey        HotkeyDef `json:"hotkey"`
	ExeName       string    `json:"exe_name"`
	LaunchCommand string    `json:"launch_command"`
	LaunchArgs    []string  `json:"launch_args"`
	MultiWindow   string    `json:"multi_window"` // "most_recent" or "cycle"
}

// HotkeyDef defines a hotkey combination.
type HotkeyDef struct {
	Modifiers []string `json:"modifiers"` // "ctrl", "alt", "shift", "win"
	Key       string   `json:"key"`       // "1", "F5", "A", etc.
}

// FormatHotkey returns a human-readable hotkey string like "Win+1".
func (h HotkeyDef) Format() string {
	parts := make([]string, 0, len(h.Modifiers)+1)
	for _, m := range h.Modifiers {
		switch strings.ToLower(m) {
		case "win":
			parts = append(parts, "Win")
		case "ctrl":
			parts = append(parts, "Ctrl")
		case "alt":
			parts = append(parts, "Alt")
		case "shift":
			parts = append(parts, "Shift")
		}
	}
	parts = append(parts, strings.ToUpper(h.Key))
	return strings.Join(parts, "+")
}

// DefaultConfig returns a config with no bindings.
func DefaultConfig() *Config {
	return &Config{
		Version:  1,
		Bindings: []Binding{},
	}
}

// cloneBindings returns a deep copy of the given bindings, including their
// nested slice fields. Used when handing off a slice from the settings dialog
// to App.Reload so subsequent edits in the dialog can't mutate live config.
func cloneBindings(src []Binding) []Binding {
	if src == nil {
		return nil
	}
	dst := make([]Binding, len(src))
	for i, b := range src {
		dst[i] = b
		if b.Hotkey.Modifiers != nil {
			dst[i].Hotkey.Modifiers = append([]string(nil), b.Hotkey.Modifiers...)
		}
		if b.LaunchArgs != nil {
			dst[i].LaunchArgs = append([]string(nil), b.LaunchArgs...)
		}
	}
	return dst
}

// ReloadResult describes the outcome of an App.Reload call. It distinguishes
// hotkey registration failures (conflicts, invalid keys) from configuration
// persistence failures so callers can decide how to surface each — e.g. the
// settings dialog keeps itself open if either is non-empty.
type ReloadResult struct {
	RegistrationErrors []error
	SaveError          error
}

// HasErrors reports whether the reload had any failure the caller should
// surface. Used by the settings dialog to decide whether to stay open.
func (r ReloadResult) HasErrors() bool {
	return len(r.RegistrationErrors) > 0 || r.SaveError != nil
}

// ConfigDir returns the application config directory (%APPDATA%\AutoSwitcher).
func ConfigDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	dir := filepath.Join(appData, "AutoSwitcher")
	return dir, nil
}

// ConfigPath returns the full path to config.json.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads config from the given path. If the file doesn't exist,
// returns a default config. If the file is corrupt, backs it up and returns default.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		// Return a default config so the caller always gets a usable config
		return DefaultConfig(), fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Back up the corrupt file
		backupPath := path + ".corrupt"
		_ = os.Rename(path, backupPath)
		return DefaultConfig(), fmt.Errorf("corrupt config backed up to %s: %w", backupPath, err)
	}

	// Ensure bindings slice is never nil
	if cfg.Bindings == nil {
		cfg.Bindings = []Binding{}
	}

	return &cfg, nil
}

// SaveConfig writes config atomically to the given path. Uses
// write-to-temp-then-rename: on Windows, modern Go's os.Rename calls
// MoveFileEx with MOVEFILE_REPLACE_EXISTING, which atomically replaces
// the destination on NTFS. Either the new file is fully in place, or
// the original file is unchanged. There is no partial-write window.
//
// On any failure (temp write, rename), the temp file is removed and an
// error is returned without touching the destination — so callers can
// rely on "if SaveConfig errors, the on-disk file is unchanged."
func SaveConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp config: %w", err)
	}
	return nil
}
