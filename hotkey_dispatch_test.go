//go:build windows

package main

import (
	"fmt"
	"testing"
)

// --- ResolveHotkeyAction unit tests ---

func TestResolveHotkeyAction_NoWindows_WithLaunch(t *testing.T) {
	b := &Binding{
		Name:          "test",
		ExeName:       "test.exe",
		LaunchCommand: "C:\\test.exe",
		LaunchArgs:    []string{"--flag"},
	}
	action, _ := ResolveHotkeyAction(b, nil, 0, cycleInfo{})
	if action.Type != ActionLaunch {
		t.Fatalf("expected ActionLaunch, got %d", action.Type)
	}
	if action.Command != "C:\\test.exe" {
		t.Errorf("expected command %q, got %q", "C:\\test.exe", action.Command)
	}
	if len(action.Args) != 1 || action.Args[0] != "--flag" {
		t.Errorf("expected args [--flag], got %v", action.Args)
	}
}

func TestResolveHotkeyAction_NoWindows_NoLaunch(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe"}
	action, _ := ResolveHotkeyAction(b, nil, 0, cycleInfo{})
	if action.Type != ActionNotify {
		t.Fatalf("expected ActionNotify, got %d", action.Type)
	}
	if action.Message != "No window found for test" {
		t.Errorf("unexpected message: %q", action.Message)
	}
}

func TestResolveHotkeyAction_MostRecent_ForegroundMatches(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}}
	action, _ := ResolveHotkeyAction(b, wins, 100, cycleInfo{})
	if action.Type != ActionNone {
		t.Fatalf("expected ActionNone, got %d", action.Type)
	}
}

func TestResolveHotkeyAction_MostRecent_FocusTopmost(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}}
	action, _ := ResolveHotkeyAction(b, wins, 999, cycleInfo{})
	if action.Type != ActionFocus {
		t.Fatalf("expected ActionFocus, got %d", action.Type)
	}
	if action.Target != 100 {
		t.Errorf("expected target 100, got %d", action.Target)
	}
}

func TestResolveHotkeyAction_Cycle_AdvancesToNext(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe", MultiWindow: "cycle"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}, {HWND: 300}}
	// Foreground is wins[0], should advance to wins[1]
	action, state := ResolveHotkeyAction(b, wins, 100, cycleInfo{})
	if action.Type != ActionFocus {
		t.Fatalf("expected ActionFocus, got %d", action.Type)
	}
	if action.Target != 200 {
		t.Errorf("expected target 200, got %d", action.Target)
	}
	if state.lastHWND != 200 {
		t.Errorf("expected lastHWND 200, got %d", state.lastHWND)
	}
}

func TestResolveHotkeyAction_Cycle_WrapsAround(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe", MultiWindow: "cycle"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}, {HWND: 300}}
	// Foreground is last window, should wrap to first
	action, state := ResolveHotkeyAction(b, wins, 300, cycleInfo{})
	if action.Type != ActionFocus {
		t.Fatalf("expected ActionFocus, got %d", action.Type)
	}
	if action.Target != 100 {
		t.Errorf("expected target 100, got %d", action.Target)
	}
	if state.lastHWND != 100 {
		t.Errorf("expected lastHWND 100, got %d", state.lastHWND)
	}
}

func TestResolveHotkeyAction_Cycle_ResumesFromLast(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe", MultiWindow: "cycle"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}, {HWND: 300}}
	// Foreground is not ours, last known is wins[1]
	action, state := ResolveHotkeyAction(b, wins, 999, cycleInfo{lastHWND: 200})
	if action.Type != ActionFocus {
		t.Fatalf("expected ActionFocus, got %d", action.Type)
	}
	if action.Target != 200 {
		t.Errorf("expected target 200 (resume), got %d", action.Target)
	}
	if state.lastHWND != 200 {
		t.Errorf("expected lastHWND 200, got %d", state.lastHWND)
	}
}

func TestResolveHotkeyAction_Cycle_StaleHWND(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe", MultiWindow: "cycle"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}}
	// Last known HWND no longer in list, should start from beginning
	action, state := ResolveHotkeyAction(b, wins, 999, cycleInfo{lastHWND: 9999})
	if action.Type != ActionFocus {
		t.Fatalf("expected ActionFocus, got %d", action.Type)
	}
	if action.Target != 100 {
		t.Errorf("expected target 100 (start from beginning), got %d", action.Target)
	}
	if state.lastHWND != 100 {
		t.Errorf("expected lastHWND 100, got %d", state.lastHWND)
	}
}

// --- HandleHotkey integration tests with mocked function variables ---

func TestHandleHotkey_FindError(t *testing.T) {
	origFind := findWindowsByExe
	defer func() { findWindowsByExe = origFind }()
	findWindowsByExe = func(exe string) ([]WindowInfo, error) {
		return nil, fmt.Errorf("enum failed")
	}

	balloonCalled := false
	hm := NewHotkeyManager(0, func(title, msg string) { balloonCalled = true })
	hm.bindings[1] = &Binding{Name: "test", ExeName: "test.exe"}

	hm.HandleHotkey(1)

	if balloonCalled {
		t.Error("showBalloon should not be called on findWindowsByExe error")
	}
}

func TestHandleHotkey_Launch(t *testing.T) {
	origFind := findWindowsByExe
	origLaunch := launchApp
	origFg := getForegroundHWND
	defer func() {
		findWindowsByExe = origFind
		launchApp = origLaunch
		getForegroundHWND = origFg
	}()

	findWindowsByExe = func(exe string) ([]WindowInfo, error) {
		return nil, nil // no windows
	}
	getForegroundHWND = func() uintptr { return 0 }

	var launchedCmd string
	var launchedArgs []string
	launchApp = func(cmd string, args []string) error {
		launchedCmd = cmd
		launchedArgs = args
		return nil
	}

	hm := NewHotkeyManager(0, func(title, msg string) {})
	hm.bindings[1] = &Binding{
		Name:          "test",
		ExeName:       "test.exe",
		LaunchCommand: "C:\\test.exe",
		LaunchArgs:    []string{"--arg"},
	}

	hm.HandleHotkey(1)

	if launchedCmd != "C:\\test.exe" {
		t.Errorf("expected launch command %q, got %q", "C:\\test.exe", launchedCmd)
	}
	if len(launchedArgs) != 1 || launchedArgs[0] != "--arg" {
		t.Errorf("expected args [--arg], got %v", launchedArgs)
	}
}

func TestHandleHotkey_Focus(t *testing.T) {
	origFind := findWindowsByExe
	origFocus := focusWindow
	origFg := getForegroundHWND
	defer func() {
		findWindowsByExe = origFind
		focusWindow = origFocus
		getForegroundHWND = origFg
	}()

	findWindowsByExe = func(exe string) ([]WindowInfo, error) {
		return []WindowInfo{{HWND: 42}}, nil
	}
	getForegroundHWND = func() uintptr { return 999 }

	var focusedHWND uintptr
	focusWindow = func(hwnd uintptr) error {
		focusedHWND = hwnd
		return nil
	}

	hm := NewHotkeyManager(0, func(title, msg string) {})
	hm.bindings[1] = &Binding{Name: "test", ExeName: "test.exe"}

	hm.HandleHotkey(1)

	if focusedHWND != 42 {
		t.Errorf("expected focus on HWND 42, got %d", focusedHWND)
	}
}

func TestHandleHotkey_FocusFailure(t *testing.T) {
	origFind := findWindowsByExe
	origFocus := focusWindow
	origFg := getForegroundHWND
	defer func() {
		findWindowsByExe = origFind
		focusWindow = origFocus
		getForegroundHWND = origFg
	}()

	findWindowsByExe = func(exe string) ([]WindowInfo, error) {
		return []WindowInfo{{HWND: 42}}, nil
	}
	getForegroundHWND = func() uintptr { return 999 }
	focusWindow = func(hwnd uintptr) error {
		return fmt.Errorf("focus failed")
	}

	var balloonTitle, balloonMsg string
	hm := NewHotkeyManager(0, func(title, msg string) {
		balloonTitle = title
		balloonMsg = msg
	})
	hm.bindings[1] = &Binding{Name: "test", ExeName: "test.exe"}

	hm.HandleHotkey(1)

	if balloonTitle != "Focus Failed" {
		t.Errorf("expected balloon title %q, got %q", "Focus Failed", balloonTitle)
	}
	if balloonMsg == "" {
		t.Error("expected non-empty balloon message")
	}
}

func TestHotkeyManager_UnregisterAllClearsBindingsAndCycleState(t *testing.T) {
	hm := NewHotkeyManager(0, nil)

	// Simulate post-RegisterAll state without actually calling Win32.
	hm.bindings[1] = &Binding{Name: "a"}
	hm.bindings[2] = &Binding{Name: "b"}
	hm.nextID = 42
	hm.cycleState["a"] = cycleInfo{lastHWND: 7}

	hm.UnregisterAll()

	if len(hm.bindings) != 0 {
		t.Errorf("bindings not cleared: %d entries", len(hm.bindings))
	}
	if len(hm.cycleState) != 0 {
		t.Errorf("cycleState not cleared: %d entries", len(hm.cycleState))
	}
	// nextID must NOT be reset — reusing IDs across reload would risk
	// dispatching a queued WM_HOTKEY message to the wrong binding.
	if hm.nextID != 42 {
		t.Errorf("nextID was reset to %d; must stay monotonic to prevent stale message misdispatch", hm.nextID)
	}
}

func TestReloadSummary(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		enabled bool
		result  ReloadResult
		want    string
	}{
		{
			name: "enabled all active",
			total: 5, enabled: true, result: ReloadResult{},
			want: "5 hotkeys active",
		},
		{
			name: "enabled with registration errors",
			total: 5, enabled: true,
			result: ReloadResult{RegistrationErrors: []error{errExample, errExample}},
			want:   "3 hotkeys active, 2 registration error(s)",
		},
		{
			name: "enabled all failed",
			total: 3, enabled: true,
			result: ReloadResult{RegistrationErrors: []error{errExample, errExample, errExample}},
			want:   "0 hotkeys active, 3 registration error(s)",
		},
		{
			name: "disabled — does not claim active",
			total: 5, enabled: false, result: ReloadResult{},
			want: "5 bindings saved (hotkeys disabled — conflicts will be checked on enable)",
		},
		{
			name: "disabled with stale registration errors ignored",
			total: 5, enabled: false,
			result: ReloadResult{RegistrationErrors: []error{errExample}},
			want:   "5 bindings saved (hotkeys disabled — conflicts will be checked on enable)",
		},
		{
			name: "zero bindings enabled",
			total: 0, enabled: true, result: ReloadResult{},
			want: "0 hotkeys active",
		},
	}
	for _, tt := range tests {
		got := reloadSummary(tt.total, tt.enabled, tt.result)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
