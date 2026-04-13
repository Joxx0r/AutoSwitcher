package main

// RecorderAction describes what the recorder should do in response to a key event.
type RecorderAction int

const (
	RecorderSuppress     RecorderAction = iota // Suppress key, no UI change
	RecorderUpdateLabel                        // Modifier changed, update label
	RecorderCancel                             // Escape pressed, cancel dialog
	RecorderAccept                             // Key accepted, close dialog
	RecorderRejectKey                          // Unsupported key pressed
	RecorderNeedModifier                       // Non-function key without modifier
)

// RecorderState holds the mutable state of the hotkey recorder.
type RecorderState struct {
	HeldModifiers uint32
	CapturedKey   uint32
	CapturedMods  uint32
	Done          bool
}

// ProcessKeyEvent is the pure decision function for the recorder.
// Given a key event and the current state, it returns the action to take
// and mutates state accordingly. All Win32/GUI side effects are left to the caller.
func (s *RecorderState) ProcessKeyEvent(vkCode uint32, isKeyDown bool) RecorderAction {
	if s.Done {
		return RecorderSuppress
	}

	if isKeyDown {
		// Modifier key — track it
		if modBit := VKToModifierBit(vkCode); modBit != 0 {
			s.HeldModifiers |= modBit
			return RecorderUpdateLabel
		}

		// Escape with no modifiers cancels
		if vkCode == 0x1B && s.HeldModifiers == 0 {
			s.Done = true
			return RecorderCancel
		}

		// Unsupported key
		if !IsSupportedVK(vkCode) {
			return RecorderRejectKey
		}

		// Non-function keys require at least one modifier
		isFunctionKey := vkCode >= 0x70 && vkCode <= 0x87
		if !isFunctionKey && s.HeldModifiers == 0 {
			return RecorderNeedModifier
		}

		// Accept the key — snapshot modifiers
		s.CapturedKey = vkCode
		s.CapturedMods = s.HeldModifiers
		s.Done = true
		return RecorderAccept
	}

	// Key up — clear modifier bits
	if modBit := VKToModifierBit(vkCode); modBit != 0 {
		s.HeldModifiers &^= modBit
		return RecorderUpdateLabel
	}
	return RecorderSuppress
}

// BackgroundKeyEvent updates HeldModifiers from a key event observed while
// the recorder dialog is not the foreground window. Only modifier transitions
// are tracked; non-modifier keys are ignored. Use this from the hook callback
// so HeldModifiers stays current across focus loss without ever clobbering
// the dialog's own captures.
func (s *RecorderState) BackgroundKeyEvent(vkCode uint32, isKeyDown bool) {
	modBit := VKToModifierBit(vkCode)
	if modBit == 0 {
		return
	}
	if isKeyDown {
		s.HeldModifiers |= modBit
	} else {
		s.HeldModifiers &^= modBit
	}
}

// ResyncFromSnapshot replaces HeldModifiers with a fresh snapshot of the
// physical keyboard state. This is the safety net for cases the LL keyboard
// hook can't observe — secure desktop transitions (UAC, Ctrl+Alt+Del),
// session switches, etc. — where modifier up/down events may not reach the
// hook. Call this on focus regain, not in the middle of a keydown decision:
// at the moment of focus regain no hook suppression is in flight, so
// GetAsyncKeyState reliably reflects the physical state.
func (s *RecorderState) ResyncFromSnapshot(snapshot uint32) {
	s.HeldModifiers = snapshot
}

// RouteHookEvent decides what the recorder should do with a key event
// arriving from the system-wide LL keyboard hook. It owns the
// background/foreground/ready branching so the GUI layer only has to
// apply the returned action.
//
// suppress is what the LowLevelKeyboardProc should return: false means
// "let the key flow through to the active app" (always false when
// backgrounded), true means "swallow it" (foreground events the recorder
// consumes, including the not-ready window before the dialog finishes
// wiring up).
//
// action is the UI-side effect to apply. RecorderSuppress here means
// "no UI work needed" — used for background events, key-up events that
// aren't modifiers, and events processed while !ready.
func (s *RecorderState) RouteHookEvent(vkCode uint32, isKeyDown, isForeground, ready bool) (suppress bool, action RecorderAction) {
	if !isForeground {
		s.BackgroundKeyEvent(vkCode, isKeyDown)
		return false, RecorderSuppress
	}
	if !ready {
		return true, RecorderSuppress
	}
	return true, s.ProcessKeyEvent(vkCode, isKeyDown)
}

// FocusTracker observes the recorder dialog's activation state and runs a
// resync on focus regain. Focus loss must be detected independently of
// keyboard events because the LL hook can't observe events during secure
// desktop transitions, lock screens, or session switches — so we drive this
// from the dialog's WM_ACTIVATE-backed events instead. Activate/Deactivate
// are expected to run on a single thread (the GUI thread) so the type is
// not safe for concurrent use.
//
// HasFocus should be initialized to true: the dialog is the foreground
// window the moment it's shown, so the very first Activate is a no-op and
// resync only kicks in after a real Deactivate.
type FocusTracker struct {
	State    *RecorderState
	Snapshot func() uint32
	OnResync func()
	HasFocus bool
}

// Deactivate marks the dialog as no longer the foreground window.
func (f *FocusTracker) Deactivate() {
	f.HasFocus = false
}

// Activate marks the dialog as the foreground window. If we were previously
// inactive, reconcile HeldModifiers against the physical keyboard state and
// fire OnResync so the UI can refresh.
func (f *FocusTracker) Activate() {
	if f.HasFocus {
		return
	}
	f.HasFocus = true
	if f.State == nil || f.Snapshot == nil {
		return
	}
	f.State.ResyncFromSnapshot(f.Snapshot())
	if f.OnResync != nil {
		f.OnResync()
	}
}
