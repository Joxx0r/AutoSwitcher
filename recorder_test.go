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

func TestRecorderProcessKeyEvent_TrackedModifierPlusFunctionKey(t *testing.T) {
	// Regression: pressing a non-modifier key after tracking a modifier must
	// not clobber HeldModifiers. Previously the resync path overwrote tracked
	// state with GetAsyncKeyState, which can miss hook-suppressed modifiers,
	// causing Ctrl+F5 to be captured as bare F5.
	s := &RecorderState{}

	s.ProcessKeyEvent(0xA2, true) // VK_LCONTROL down
	if s.HeldModifiers != modControl {
		t.Fatalf("after Ctrl down: HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modControl)
	}

	action := s.ProcessKeyEvent(0x74, true) // F5 down
	if action != RecorderAccept {
		t.Errorf("Ctrl+F5: got %d, want RecorderAccept", action)
	}
	if s.CapturedKey != 0x74 {
		t.Errorf("CapturedKey = 0x%X, want 0x74", s.CapturedKey)
	}
	if s.CapturedMods != modControl {
		t.Errorf("CapturedMods = 0x%X, want 0x%X", s.CapturedMods, modControl)
	}
}

func TestRecorderProcessKeyEvent_TrackedModifierPlusLetter(t *testing.T) {
	// Regression: Ctrl+A via the tracking path must capture both, not get
	// rejected as bare 'A' due to a clobbered HeldModifiers.
	s := &RecorderState{}

	s.ProcessKeyEvent(0xA2, true) // VK_LCONTROL down
	action := s.ProcessKeyEvent(0x41, true) // 'A' down
	if action != RecorderAccept {
		t.Errorf("Ctrl+A: got %d, want RecorderAccept", action)
	}
	if s.CapturedMods != modControl {
		t.Errorf("CapturedMods = 0x%X, want 0x%X", s.CapturedMods, modControl)
	}
}

func TestRecorderProcessKeyEvent_NonModifierDoesNotTouchHeldModifiers(t *testing.T) {
	// A non-modifier keydown should never mutate HeldModifiers — it either
	// accepts (snapshotting into CapturedMods) or rejects.
	s := &RecorderState{HeldModifiers: modShift | modAlt}

	s.ProcessKeyEvent(0x74, true) // F5 down — accepts
	if s.HeldModifiers != modShift|modAlt {
		t.Errorf("HeldModifiers mutated: 0x%X, want 0x%X", s.HeldModifiers, modShift|modAlt)
	}
	if s.CapturedMods != modShift|modAlt {
		t.Errorf("CapturedMods = 0x%X, want 0x%X", s.CapturedMods, modShift|modAlt)
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

func TestRecorderBackgroundKeyEvent_TracksModifierDownUp(t *testing.T) {
	s := &RecorderState{}

	s.BackgroundKeyEvent(0xA2, true) // VK_LCONTROL down while backgrounded
	if s.HeldModifiers != modControl {
		t.Errorf("after Ctrl down: HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modControl)
	}

	s.BackgroundKeyEvent(0xA4, true) // VK_LMENU down
	if s.HeldModifiers != modControl|modAlt {
		t.Errorf("after Alt down: HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modControl|modAlt)
	}

	s.BackgroundKeyEvent(0xA2, false) // VK_LCONTROL up
	if s.HeldModifiers != modAlt {
		t.Errorf("after Ctrl up: HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modAlt)
	}
}

func TestRecorderBackgroundKeyEvent_IgnoresNonModifiers(t *testing.T) {
	// Non-modifier keys observed while backgrounded must not touch state —
	// the user is typing into another application.
	s := &RecorderState{HeldModifiers: modControl}

	s.BackgroundKeyEvent(0x41, true)  // 'A' down
	s.BackgroundKeyEvent(0x74, true)  // F5 down
	s.BackgroundKeyEvent(0x41, false) // 'A' up

	if s.HeldModifiers != modControl {
		t.Errorf("HeldModifiers mutated by non-modifier key: 0x%X, want 0x%X", s.HeldModifiers, modControl)
	}
	if s.CapturedKey != 0 || s.CapturedMods != 0 {
		t.Errorf("CapturedKey/Mods unexpectedly set: key=0x%X, mods=0x%X", s.CapturedKey, s.CapturedMods)
	}
	if s.Done {
		t.Error("Done unexpectedly set true while backgrounded")
	}
}

func TestRecorderResyncFromSnapshot_RecoversMissedRelease(t *testing.T) {
	// Codex scenario: user holds Ctrl, dialog loses focus during a secure
	// desktop transition, Ctrl is released elsewhere, the LL hook never sees
	// the keyup, dialog regains focus. The focus-regain resync must replace
	// the stale Ctrl bit so a subsequent bare 'A' is rejected and bare Esc
	// cancels.
	s := &RecorderState{HeldModifiers: modControl}

	s.ResyncFromSnapshot(0) // physical state shows nothing held

	if s.HeldModifiers != 0 {
		t.Fatalf("after resync: HeldModifiers = 0x%X, want 0", s.HeldModifiers)
	}

	if action := s.ProcessKeyEvent(0x41, true); action != RecorderNeedModifier {
		t.Errorf("post-resync bare A: got %d, want RecorderNeedModifier", action)
	}

	s2 := &RecorderState{HeldModifiers: modControl}
	s2.ResyncFromSnapshot(0)
	if action := s2.ProcessKeyEvent(0x1B, true); action != RecorderCancel {
		t.Errorf("post-resync bare Esc: got %d, want RecorderCancel", action)
	}
}

func TestRecorderResyncFromSnapshot_PreservesStillHeld(t *testing.T) {
	// If a modifier is still physically held at focus regain, it must remain
	// in HeldModifiers so the next non-modifier key captures it.
	s := &RecorderState{HeldModifiers: modControl}

	s.ResyncFromSnapshot(modShift) // Ctrl was released, Shift is now held instead

	action := s.ProcessKeyEvent(0x41, true) // 'A' with Shift
	if action != RecorderAccept {
		t.Errorf("Shift+A after resync: got %d, want RecorderAccept", action)
	}
	if s.CapturedMods != modShift {
		t.Errorf("CapturedMods = 0x%X, want 0x%X", s.CapturedMods, modShift)
	}
}

func TestRecorderFocusLossThenRegain_FullCycle(t *testing.T) {
	// End-to-end simulation of the integration path that hookCB exercises:
	//   1. Foreground: track Ctrl down via ProcessKeyEvent.
	//   2. Focus loss: simulate user releasing Ctrl outside our process via
	//      BackgroundKeyEvent (the LL hook observes it system-wide).
	//   3. Focus regain: ResyncFromSnapshot reconciles against the physical
	//      keyboard state.
	//   4. Foreground: bare 'A' should be rejected (no modifier held).
	s := &RecorderState{}

	if action := s.ProcessKeyEvent(0xA2, true); action != RecorderUpdateLabel {
		t.Fatalf("Ctrl down: got %d, want RecorderUpdateLabel", action)
	}
	if s.HeldModifiers != modControl {
		t.Fatalf("HeldModifiers = 0x%X, want 0x%X", s.HeldModifiers, modControl)
	}

	// Backgrounded — user releases Ctrl outside the dialog
	s.BackgroundKeyEvent(0xA2, false)
	if s.HeldModifiers != 0 {
		t.Fatalf("after background Ctrl up: HeldModifiers = 0x%X, want 0", s.HeldModifiers)
	}

	// Focus regain — physical state confirms nothing is held
	s.ResyncFromSnapshot(0)

	// Foreground — bare 'A' must be rejected
	if action := s.ProcessKeyEvent(0x41, true); action != RecorderNeedModifier {
		t.Errorf("bare A after focus cycle: got %d, want RecorderNeedModifier", action)
	}
}

func TestRecorderFocusLossWithMissedRelease_RegainResyncs(t *testing.T) {
	// Worst case: the LL hook misses the Ctrl up entirely (e.g. during a
	// secure desktop transition). Without the focus-regain resync the stale
	// Ctrl bit would let bare 'A' incorrectly capture as Ctrl+A. With
	// ResyncFromSnapshot reflecting the true physical state, it must reject.
	s := &RecorderState{}

	s.ProcessKeyEvent(0xA2, true) // Ctrl down

	// Hook never observes the Ctrl up (no BackgroundKeyEvent call).
	// HeldModifiers is now stale.

	s.ResyncFromSnapshot(0) // focus regain, true state is "nothing held"

	if action := s.ProcessKeyEvent(0x41, true); action != RecorderNeedModifier {
		t.Errorf("bare A after missed-release resync: got %d, want RecorderNeedModifier", action)
	}
}
