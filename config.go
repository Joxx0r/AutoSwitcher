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

// SaveConfig writes config to the given path.
// Uses a write-to-temp-then-rename strategy for safety. On Windows,
// os.Rename cannot overwrite, so we fall back to direct write if needed.
func SaveConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Try temp-file-then-rename for new files
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err == nil {
		return nil // fresh write succeeded
	}

	// Rename failed (likely because destination exists on Windows).
	// Clean up temp file and write directly to the destination.
	_ = os.Remove(tmpPath)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
