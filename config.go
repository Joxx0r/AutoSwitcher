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
// settings dialog keeps itself open if any field is non-empty.
//
// SaveError is the error from the initial SaveConfig that gates the commit.
// If non-nil, no live state was changed and the on-disk file is unchanged.
//
// RegistrationErrors lists per-binding failures from the initial register
// attempt. When non-empty, Reload rolls back to the previous state.
//
// RollbackSaveError is the error from re-persisting the previous state
// during rollback. If non-nil, the in-memory state was successfully reverted
// but the on-disk file may still hold the rejected candidate set — a real
// inconsistency the user needs to know about.
type ReloadResult struct {
	RegistrationErrors []error
	SaveError          error
	RollbackSaveError  error
}

// HasErrors reports whether the reload had any failure the caller should
// surface. Used by the settings dialog to decide whether to stay open.
func (r ReloadResult) HasErrors() bool {
	return len(r.RegistrationErrors) > 0 || r.SaveError != nil || r.RollbackSaveError != nil
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

// renameFile is the rename primitive SaveConfig uses; tests swap it to
// inject deterministic failures without touching the real filesystem.
var renameFile = os.Rename

// SaveConfig writes config to the given path using a temp-file-then-rename
// strategy. On Windows/NTFS, Go's os.Rename calls MoveFileEx with
// MOVEFILE_REPLACE_EXISTING, which atomically replaces the destination —
// so on the supported platform, the on-disk file is either fully the new
// content or fully the previous content, with no partial-write window.
// Go's general os.Rename docs caveat that this is not guaranteed on every
// non-Unix filesystem (e.g. FAT32, network shares); on those, the rename
// step itself may be non-atomic, but the temp-file-first approach still
// avoids the failure mode of a half-written destination from a single
// truncating write.
//
// On any failure (temp write or rename), the temp file is cleaned up and
// an error is returned. The destination is not touched on the rename-error
// path — callers can rely on "if SaveConfig returns an error from a rename
// failure, the on-disk file is unchanged."
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

	if err := renameFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp config: %w", err)
	}
	return nil
}
