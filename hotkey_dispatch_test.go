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
	action, _ := ResolveHotkeyAction(b, nil, 0, bindingState{})
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
	action, _ := ResolveHotkeyAction(b, nil, 0, bindingState{})
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
	action, _ := ResolveHotkeyAction(b, wins, 100, bindingState{})
	if action.Type != ActionNone {
		t.Fatalf("expected ActionNone, got %d", action.Type)
	}
}

func TestResolveHotkeyAction_MostRecent_FocusTopmost(t *testing.T) {
	b := &Binding{Name: "test", ExeName: "test.exe"}
	wins := []WindowInfo{{HWND: 100}, {HWND: 200}}
	action, _ := ResolveHotkeyAction(b, wins, 999, bindingState{})
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
	action, state := ResolveHotkeyAction(b, wins, 100, bindingState{})
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
	action, state := ResolveHotkeyAction(b, wins, 300, bindingState{})
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
	action, state := ResolveHotkeyAction(b, wins, 999, bindingState{lastHWND: 200})
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
	action, state := ResolveHotkeyAction(b, wins, 999, bindingState{lastHWND: 9999})
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
