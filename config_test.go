package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestSaveConfigPreservesOriginalOnRenameFailure(t *testing.T) {
	// SaveConfig's contract: if the rename step fails, the destination file
	// is unchanged on disk and the temp file is cleaned up. Reload's
	// transactional rollback depends on this. We use the renameFile seam to
	// inject a deterministic rename failure and then read the destination
	// bytes BEFORE and AFTER the failed save to prove they're identical.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := DefaultConfig()
	original.Autostart = true
	original.Bindings = []Binding{{Name: "Original", ExeName: "orig.exe"}}
	if err := SaveConfig(path, original); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	beforeBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Inject a rename failure for the next SaveConfig call.
	origRename := renameFile
	renameFile = func(_, _ string) error {
		return fmt.Errorf("simulated rename failure")
	}
	defer func() { renameFile = origRename }()

	// Attempt to overwrite with different content. Should fail.
	bad := DefaultConfig()
	bad.Autostart = false
	bad.Bindings = []Binding{{Name: "Replacement", ExeName: "bad.exe"}}
	if err := SaveConfig(path, bad); err == nil {
		t.Fatal("SaveConfig should have failed with injected rename error")
	}

	// The temp file must be cleaned up even on failure.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file leaked after rename failure: %v", err)
	}

	// Read the destination bytes — must be byte-identical to the original
	// content from before the failed save attempt.
	afterBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read destination after failed save: %v", err)
	}
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Errorf("destination file modified by failed SaveConfig\nbefore: %s\nafter:  %s", beforeBytes, afterBytes)
	}

	// Sanity check: loading should still give the original config.
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after failed save: %v", err)
	}
	if !loaded.Autostart || len(loaded.Bindings) != 1 || loaded.Bindings[0].Name != "Original" {
		t.Errorf("original config not preserved: %+v", loaded)
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

func TestCloneBindings_Independence(t *testing.T) {
	// Mutating the result of cloneBindings — including its nested slice
	// fields — must not affect the source. This is the regression test for
	// the slice-aliasing bug between settings dialog working state and
	// App.config.Bindings.
	src := []Binding{
		{
			Name:          "VS Code",
			Hotkey:        HotkeyDef{Modifiers: []string{"win"}, Key: "1"},
			ExeName:       "Code.exe",
			LaunchCommand: "C:\\code.exe",
			LaunchArgs:    []string{"--new-window"},
			MultiWindow:   "most_recent",
		},
		{
			Name:        "Term",
			Hotkey:      HotkeyDef{Modifiers: []string{"ctrl", "alt"}, Key: "T"},
			ExeName:     "wt.exe",
			MultiWindow: "cycle",
		},
	}

	dst := cloneBindings(src)

	if len(dst) != len(src) {
		t.Fatalf("len(dst) = %d, want %d", len(dst), len(src))
	}

	// Mutate dst — top-level fields and nested slices — and verify src
	// is untouched.
	dst[0].Name = "MUTATED"
	dst[0].Hotkey.Modifiers[0] = "alt"
	dst[0].LaunchArgs[0] = "--evil"
	dst[1].Hotkey.Modifiers = append(dst[1].Hotkey.Modifiers, "shift")

	if src[0].Name != "VS Code" {
		t.Errorf("src[0].Name mutated: %q", src[0].Name)
	}
	if src[0].Hotkey.Modifiers[0] != "win" {
		t.Errorf("src[0].Hotkey.Modifiers[0] mutated: %q", src[0].Hotkey.Modifiers[0])
	}
	if src[0].LaunchArgs[0] != "--new-window" {
		t.Errorf("src[0].LaunchArgs[0] mutated: %q", src[0].LaunchArgs[0])
	}
	if len(src[1].Hotkey.Modifiers) != 2 {
		t.Errorf("src[1].Hotkey.Modifiers grew: %v", src[1].Hotkey.Modifiers)
	}
}

func TestCloneBindings_Nil(t *testing.T) {
	if got := cloneBindings(nil); got != nil {
		t.Errorf("cloneBindings(nil) = %v, want nil", got)
	}
}

func TestCloneBindings_Empty(t *testing.T) {
	got := cloneBindings([]Binding{})
	if got == nil {
		t.Error("cloneBindings([]) returned nil")
	}
	if len(got) != 0 {
		t.Errorf("cloneBindings([]) len = %d, want 0", len(got))
	}
}

func TestReloadResult_HasErrors(t *testing.T) {
	tests := []struct {
		name   string
		result ReloadResult
		want   bool
	}{
		{"empty", ReloadResult{}, false},
		{"only registration errors", ReloadResult{RegistrationErrors: []error{errExample}}, true},
		{"only save error", ReloadResult{SaveError: errExample}, true},
		{"only rollback save error", ReloadResult{RollbackSaveError: errExample}, true},
		{"registration + save", ReloadResult{RegistrationErrors: []error{errExample}, SaveError: errExample}, true},
		{"registration + rollback", ReloadResult{RegistrationErrors: []error{errExample}, RollbackSaveError: errExample}, true},
		{"all three", ReloadResult{RegistrationErrors: []error{errExample}, SaveError: errExample, RollbackSaveError: errExample}, true},
	}
	for _, tt := range tests {
		if got := tt.result.HasErrors(); got != tt.want {
			t.Errorf("%s: HasErrors() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

var errExample = &simpleError{"boom"}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }
