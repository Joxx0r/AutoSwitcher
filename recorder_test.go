package main

import "testing"

func TestRecorderProcessKeyEvent_ModifierTracking(t *testing.T) {
	s := &RecorderState{}

	// Press Ctrl (VK_LCONTROL = 0xA2)
	action := s.ProcessKeyEvent(0xA2, true)
	if action != RecorderUpdateLabel {
		t.Errorf("Ctrl down: got %d, want RecorderUpdateLabel", action)
	}
	if s.HeldModifiers != modControl {
		t.Errorf("HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modControl)
	}

	// Press Alt (VK_LMENU = 0xA4)
	action = s.ProcessKeyEvent(0xA4, true)
	if action != RecorderUpdateLabel {
		t.Errorf("Alt down: got %d, want RecorderUpdateLabel", action)
	}
	if s.HeldModifiers != modControl|modAlt {
		t.Errorf("HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modControl|modAlt)
	}

	// Release Ctrl
	action = s.ProcessKeyEvent(0xA2, false)
	if action != RecorderUpdateLabel {
		t.Errorf("Ctrl up: got %d, want RecorderUpdateLabel", action)
	}
	if s.HeldModifiers != modAlt {
		t.Errorf("HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modAlt)
	}
}

func TestRecorderProcessKeyEvent_AcceptWithModifier(t *testing.T) {
	s := &RecorderState{HeldModifiers: modWin}

	// Press '1' (VK = 0x31) with Win held
	action := s.ProcessKeyEvent(0x31, true)
	if action != RecorderAccept {
		t.Errorf("Win+1: got %d, want RecorderAccept", action)
	}
	if s.CapturedKey != 0x31 {
		t.Errorf("CapturedKey = 0x%X, want 0x31", s.CapturedKey)
	}
	if s.CapturedMods != modWin {
		t.Errorf("CapturedMods = 0x%X, want 0x%X", s.CapturedMods, modWin)
	}
	if !s.Done {
		t.Error("expected Done = true after accept")
	}
}

func TestRecorderProcessKeyEvent_FunctionKeyWithoutModifier(t *testing.T) {
	s := &RecorderState{}

	// F5 (VK = 0x74) without any modifier — should be accepted
	action := s.ProcessKeyEvent(0x74, true)
	if action != RecorderAccept {
		t.Errorf("bare F5: got %d, want RecorderAccept", action)
	}
	if s.CapturedKey != 0x74 {
		t.Errorf("CapturedKey = 0x%X, want 0x74", s.CapturedKey)
	}
}

func TestRecorderProcessKeyEvent_RejectBareNonFunctionKey(t *testing.T) {
	s := &RecorderState{}

	// Press 'A' (VK = 0x41) without modifier — should require modifier
	action := s.ProcessKeyEvent(0x41, true)
	if action != RecorderNeedModifier {
		t.Errorf("bare A: got %d, want RecorderNeedModifier", action)
	}
	if s.Done {
		t.Error("expected Done = false after NeedModifier")
	}
}

func TestRecorderProcessKeyEvent_EscapeCancel(t *testing.T) {
	s := &RecorderState{}

	// Escape with no modifiers cancels
	action := s.ProcessKeyEvent(0x1B, true)
	if action != RecorderCancel {
		t.Errorf("bare Escape: got %d, want RecorderCancel", action)
	}
	if !s.Done {
		t.Error("expected Done = true after cancel")
	}
}

func TestRecorderProcessKeyEvent_EscapeWithModifier(t *testing.T) {
	s := &RecorderState{HeldModifiers: modControl}

	// Ctrl+Escape should be accepted as a hotkey, not cancel
	action := s.ProcessKeyEvent(0x1B, true)
	if action != RecorderAccept {
		t.Errorf("Ctrl+Escape: got %d, want RecorderAccept", action)
	}
	if s.CapturedKey != 0x1B {
		t.Errorf("CapturedKey = 0x%X, want 0x1B", s.CapturedKey)
	}
}

func TestRecorderProcessKeyEvent_UnsupportedKey(t *testing.T) {
	s := &RecorderState{HeldModifiers: modControl}

	// OEM key (0xBA = semicolon) — not in supported vocabulary
	action := s.ProcessKeyEvent(0xBA, true)
	if action != RecorderRejectKey {
		t.Errorf("Ctrl+semicolon: got %d, want RecorderRejectKey", action)
	}
	if s.Done {
		t.Error("expected Done = false after reject")
	}
}

func TestRecorderProcessKeyEvent_SuppressAfterDone(t *testing.T) {
	s := &RecorderState{Done: true}

	action := s.ProcessKeyEvent(0x41, true)
	if action != RecorderSuppress {
		t.Errorf("after done: got %d, want RecorderSuppress", action)
	}
}

func TestRecorderProcessKeyEvent_ModifierSnapshotBeforeRelease(t *testing.T) {
	s := &RecorderState{}

	// Hold Win + Ctrl
	s.ProcessKeyEvent(0x5B, true)  // VK_LWIN
	s.ProcessKeyEvent(0xA2, true)  // VK_LCONTROL

	// Press '1' — captures both modifiers
	s.ProcessKeyEvent(0x31, true)
	if s.CapturedMods != modWin|modControl {
		t.Errorf("CapturedMods = 0x%X, want 0x%X", s.CapturedMods, modWin|modControl)
	}

	// Release keys after accept — CapturedMods should not change
	s.ProcessKeyEvent(0x5B, false)
	s.ProcessKeyEvent(0xA2, false)
	if s.CapturedMods != modWin|modControl {
		t.Errorf("CapturedMods after release = 0x%X, want 0x%X (should be snapshot)", s.CapturedMods, modWin|modControl)
	}
}
