package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Bindings == nil {
		t.Error("expected non-nil bindings slice")
	}
	if len(cfg.Bindings) != 0 {
		t.Errorf("expected 0 bindings, got %d", len(cfg.Bindings))
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := &Config{
		Version:   1,
		Autostart: true,
		Bindings: []Binding{
			{
				Name:          "VS Code",
				Hotkey:        HotkeyDef{Modifiers: []string{"win"}, Key: "1"},
				ExeName:       "Code.exe",
				LaunchCommand: `C:\Program Files\VS Code\Code.exe`,
				LaunchArgs:    []string{"--new-window"},
				MultiWindow:   "most_recent",
			},
			{
				Name:          "Unreal Editor",
				Hotkey:        HotkeyDef{Modifiers: []string{"win"}, Key: "2"},
				ExeName:       "UnrealEditor.exe",
				LaunchCommand: "",
				LaunchArgs:    nil,
				MultiWindow:   "cycle",
			},
		},
	}

	if err := SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("version: got %d, want %d", loaded.Version, original.Version)
	}
	if loaded.Autostart != original.Autostart {
		t.Errorf("autostart: got %v, want %v", loaded.Autostart, original.Autostart)
	}
	if len(loaded.Bindings) != len(original.Bindings) {
		t.Fatalf("bindings count: got %d, want %d", len(loaded.Bindings), len(original.Bindings))
	}

	for i, b := range loaded.Bindings {
		orig := original.Bindings[i]
		if b.Name != orig.Name {
			t.Errorf("binding[%d].Name: got %q, want %q", i, b.Name, orig.Name)
		}
		if b.ExeName != orig.ExeName {
			t.Errorf("binding[%d].ExeName: got %q, want %q", i, b.ExeName, orig.ExeName)
		}
		if b.MultiWindow != orig.MultiWindow {
			t.Errorf("binding[%d].MultiWindow: got %q, want %q", i, b.MultiWindow, orig.MultiWindow)
		}
	}
}

func TestLoadConfigMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected default version 1, got %d", cfg.Version)
	}
}

func TestLoadConfigCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{invalid json!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for corrupt config")
	}
	// Should still return a usable default config
	if cfg == nil {
		t.Fatal("expected non-nil default config on corruption")
	}
	if cfg.Version != 1 {
		t.Errorf("expected default version 1, got %d", cfg.Version)
	}

	// Corrupt file should be backed up
	backupPath := path + ".corrupt"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected corrupt backup file to exist")
	}
}

func TestSaveConfigOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// First save
	cfg1 := DefaultConfig()
	cfg1.Autostart = false
	if err := SaveConfig(path, cfg1); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Second save (overwrite) — this must succeed on Windows
	cfg2 := DefaultConfig()
	cfg2.Autostart = true
	cfg2.Bindings = []Binding{{Name: "Test", ExeName: "test.exe"}}
	if err := SaveConfig(path, cfg2); err != nil {
		t.Fatalf("second save (overwrite) failed: %v", err)
	}

	// Verify the overwritten content
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after overwrite: %v", err)
	}
	if !loaded.Autostart {
		t.Error("expected autostart=true after overwrite")
	}
	if len(loaded.Bindings) != 1 {
		t.Errorf("expected 1 binding after overwrite, got %d", len(loaded.Bindings))
	}
}

func TestSaveConfigAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	// Temp file should not remain
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}

	// Config should be valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var check Config
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("saved config is not valid JSON: %v", err)
	}
}

func TestLoadConfigNilBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write config with null bindings
	if err := os.WriteFile(path, []byte(`{"version":1,"bindings":null}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bindings == nil {
		t.Error("expected non-nil bindings slice even when JSON has null")
	}
}

func TestSaveConfigCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig should create nested dirs: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file should exist after save")
	}
}

func TestConfigRoundTripNewFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := &Config{
		Version: 1,
		Bindings: []Binding{
			{
				Name:         "Gmail Chrome",
				Hotkey:       HotkeyDef{Modifiers: []string{"win"}, Key: "1"},
				ExeName:      "chrome.exe",
				TitlePattern: "Gmail",
				MultiWindow:  "toggle",
			},
			{
				Name:   "Dev Workspace",
				Hotkey: HotkeyDef{Modifiers: []string{"win"}, Key: "F1"},
				Type:   "workspace",
				WorkspaceItems: []WorkspaceItem{
					{ExeName: "Code.exe", LaunchCommand: `C:\VS Code\Code.exe`, LaunchArgs: []string{"--new-window"}},
					{ExeName: "WindowsTerminal.exe", TitlePattern: "dev", LaunchCommand: "wt.exe"},
				},
			},
		},
	}

	if err := SaveConfig(path, original); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(loaded.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(loaded.Bindings))
	}

	b0 := loaded.Bindings[0]
	if b0.TitlePattern != "Gmail" {
		t.Errorf("binding[0].TitlePattern: got %q, want %q", b0.TitlePattern, "Gmail")
	}
	if b0.MultiWindow != "toggle" {
		t.Errorf("binding[0].MultiWindow: got %q, want %q", b0.MultiWindow, "toggle")
	}

	b1 := loaded.Bindings[1]
	if b1.Type != "workspace" {
		t.Errorf("binding[1].Type: got %q, want %q", b1.Type, "workspace")
	}
	if len(b1.WorkspaceItems) != 2 {
		t.Fatalf("binding[1].WorkspaceItems: got %d items, want 2", len(b1.WorkspaceItems))
	}
	if b1.WorkspaceItems[0].ExeName != "Code.exe" {
		t.Errorf("workspace item 0 ExeName: got %q, want %q", b1.WorkspaceItems[0].ExeName, "Code.exe")
	}
	if b1.WorkspaceItems[1].TitlePattern != "dev" {
		t.Errorf("workspace item 1 TitlePattern: got %q, want %q", b1.WorkspaceItems[1].TitlePattern, "dev")
	}
	if len(b1.WorkspaceItems[0].LaunchArgs) != 1 || b1.WorkspaceItems[0].LaunchArgs[0] != "--new-window" {
		t.Errorf("workspace item 0 LaunchArgs: got %v", b1.WorkspaceItems[0].LaunchArgs)
	}
}

func TestConfigBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Old-format JSON without new fields
	oldJSON := `{
		"version": 1,
		"bindings": [
			{
				"name": "Notepad",
				"hotkey": {"modifiers": ["win"], "key": "1"},
				"exe_name": "notepad.exe",
				"launch_command": "",
				"launch_args": null,
				"multi_window": "most_recent"
			}
		]
	}`
	if err := os.WriteFile(path, []byte(oldJSON), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	b := loaded.Bindings[0]
	if b.TitlePattern != "" {
		t.Errorf("expected empty TitlePattern, got %q", b.TitlePattern)
	}
	if b.Type != "" {
		t.Errorf("expected empty Type, got %q", b.Type)
	}
	if b.WorkspaceItems != nil {
		t.Errorf("expected nil WorkspaceItems, got %v", b.WorkspaceItems)
	}
}

func TestHotkeyDefFormat(t *testing.T) {
	tests := []struct {
		hotkey HotkeyDef
		want   string
	}{
		{HotkeyDef{Modifiers: []string{"win"}, Key: "1"}, "Win+1"},
		{HotkeyDef{Modifiers: []string{"ctrl", "alt"}, Key: "d"}, "Ctrl+Alt+D"},
		{HotkeyDef{Modifiers: []string{"ctrl", "shift"}, Key: "F5"}, "Ctrl+Shift+F5"},
		{HotkeyDef{Modifiers: nil, Key: "F13"}, "F13"},
	}
	for _, tt := range tests {
		got := tt.hotkey.Format()
		if got != tt.want {
			t.Errorf("Format(%v) = %q, want %q", tt.hotkey, got, tt.want)
		}
	}
}
