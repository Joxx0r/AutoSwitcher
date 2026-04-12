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
