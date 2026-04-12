//go:build windows

package main

import (
	"fmt"
	"log"
)

var (
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
)

// ActionType represents the kind of action to take in response to a hotkey.
type ActionType int

const (
	ActionNone   ActionType = iota
	ActionFocus
	ActionLaunch
	ActionNotify
)

// HotkeyAction describes what should happen when a hotkey is pressed.
type HotkeyAction struct {
	Type    ActionType
	Target  uintptr
	Command string
	Args    []string
	Title   string
	Message string
}

// HotkeyManager handles registration and dispatch of global hotkeys.
type HotkeyManager struct {
	bindings    map[int32]*Binding
	nextID      int32
	cycleState  map[string]bindingState
	hwnd        uintptr
	showBalloon func(title, msg string)
}

// NewHotkeyManager creates a new hotkey manager.
func NewHotkeyManager(hwnd uintptr, showBalloon func(title, msg string)) *HotkeyManager {
	return &HotkeyManager{
		bindings:    make(map[int32]*Binding),
		nextID:      1,
		cycleState:  make(map[string]bindingState),
		hwnd:        hwnd,
		showBalloon: showBalloon,
	}
}

// RegisterAll registers hotkeys for all bindings. Returns errors for any that fail.
// Successfully registered hotkeys remain active even if others fail.
func (hm *HotkeyManager) RegisterAll(bindings []Binding) []error {
	var errs []error
	for i := range bindings {
		b := &bindings[i]
		vk, err := ParseKey(b.Hotkey.Key)
		if err != nil {
			errMsg := fmt.Sprintf("Invalid key %q for %s: %v", b.Hotkey.Key, b.Name, err)
			log.Println(errMsg)
			if hm.showBalloon != nil {
				hm.showBalloon("Hotkey Error", errMsg)
			}
			errs = append(errs, fmt.Errorf("%s", errMsg))
			continue
		}

		mods := ParseModifiers(b.Hotkey.Modifiers)
		id := hm.nextID
		hm.nextID++

		ret, _, _ := procRegisterHotKey.Call(hm.hwnd, uintptr(id), uintptr(mods), uintptr(vk))
		if ret == 0 {
			errMsg := fmt.Sprintf("Hotkey %s is already in use by another application", b.Hotkey.Format())
			log.Println(errMsg)
			if hm.showBalloon != nil {
				hm.showBalloon("Hotkey Conflict", errMsg)
			}
			errs = append(errs, fmt.Errorf("%s", errMsg))
			continue
		}

		hm.bindings[int32(id)] = b
		log.Printf("Registered hotkey %s (id=%d) for %s", b.Hotkey.Format(), id, b.Name)
	}
	return errs
}

// UnregisterAll unregisters all currently registered hotkeys.
func (hm *HotkeyManager) UnregisterAll() {
	for id := range hm.bindings {
		_, _, _ = procUnregisterHotKey.Call(hm.hwnd, uintptr(id))
	}
	hm.bindings = make(map[int32]*Binding)
	hm.cycleState = make(map[string]bindingState)
	log.Println("Unregistered all hotkeys")
}

// ResolveHotkeyAction determines what action to take for a hotkey press.
// It is a pure function that does not perform any side effects.
func ResolveHotkeyAction(binding *Binding, wins []WindowInfo, foreground uintptr, state bindingState) (HotkeyAction, bindingState) {
	if len(wins) == 0 {
		if binding.LaunchCommand != "" {
			return HotkeyAction{
				Type:    ActionLaunch,
				Command: binding.LaunchCommand,
				Args:    binding.LaunchArgs,
			}, state
		}
		return HotkeyAction{
			Type:    ActionNotify,
			Title:   "AutoSwitcher",
			Message: fmt.Sprintf("No window found for %s", binding.Name),
		}, state
	}

	switch binding.MultiWindow {
	case "cycle":
		return resolveCycle(binding, wins, foreground, state)
	case "toggle":
		return resolveToggle(binding, wins, foreground, state)
	default: // "most_recent" or unset
		return resolveMostRecent(wins, foreground, state)
	}
}

func resolveMostRecent(wins []WindowInfo, foreground uintptr, state bindingState) (HotkeyAction, bindingState) {
	// If the foreground window is already one of our matches, do nothing
	for _, w := range wins {
		if w.HWND == foreground {
			return HotkeyAction{Type: ActionNone}, state
		}
	}

	// Focus the first (topmost Z-order) match
	return HotkeyAction{
		Type:   ActionFocus,
		Target: wins[0].HWND,
	}, state
}

func resolveCycle(binding *Binding, wins []WindowInfo, foreground uintptr, state bindingState) (HotkeyAction, bindingState) {
	// Find the index of the foreground window in our list
	foregroundIdx := -1
	for i, w := range wins {
		if w.HWND == foreground {
			foregroundIdx = i
			break
		}
	}

	var targetIdx int
	if foregroundIdx >= 0 {
		// Foreground is one of ours - cycle to next
		targetIdx = (foregroundIdx + 1) % len(wins)
	} else {
		// Foreground is not ours - try to resume from last known HWND
		lastIdx := -1
		for i, w := range wins {
			if w.HWND == state.lastHWND {
				lastIdx = i
				break
			}
		}
		if lastIdx >= 0 {
			targetIdx = lastIdx
		} else {
			targetIdx = 0 // start from beginning
		}
	}

	target := wins[targetIdx]
	newState := bindingState{lastHWND: target.HWND}

	return HotkeyAction{
		Type:   ActionFocus,
		Target: target.HWND,
	}, newState
}

func resolveToggle(binding *Binding, wins []WindowInfo, foreground uintptr, state bindingState) (HotkeyAction, bindingState) {
	// Check if foreground is one of our matches
	isForegroundMatch := false
	for _, w := range wins {
		if w.HWND == foreground {
			isForegroundMatch = true
			break
		}
	}

	if !isForegroundMatch {
		// Focus the first match, save current foreground as previousHWND
		newState := bindingState{
			lastHWND:     state.lastHWND,
			previousHWND: foreground,
		}
		return HotkeyAction{
			Type:   ActionFocus,
			Target: wins[0].HWND,
		}, newState
	}

	// Foreground IS a match
	if state.previousHWND != 0 {
		if isWindowValid(state.previousHWND) {
			// Go back to previous window
			newState := bindingState{
				lastHWND:     state.lastHWND,
				previousHWND: 0,
			}
			return HotkeyAction{
				Type:   ActionFocus,
				Target: state.previousHWND,
			}, newState
		}
		// Previous window is stale — clear it
		newState := bindingState{
			lastHWND:     state.lastHWND,
			previousHWND: 0,
		}
		return HotkeyAction{Type: ActionNone}, newState
	}

	// No previous window to go back to
	return HotkeyAction{Type: ActionNone}, state
}

// handleWorkspace processes a workspace binding by focusing/launching multiple apps.
func (hm *HotkeyManager) handleWorkspace(binding *Binding) {
	foreground := getForegroundHWND()

	type pendingAction struct {
		action  HotkeyAction
		itemExe string
	}
	var launches []pendingAction
	var focuses []pendingAction

	for _, item := range binding.WorkspaceItems {
		syntheticBinding := &Binding{
			Name:          item.ExeName,
			ExeName:       item.ExeName,
			LaunchCommand: item.LaunchCommand,
			LaunchArgs:    item.LaunchArgs,
			MultiWindow:   "most_recent",
		}

		wins, err := findWindowsByExe(item.ExeName)
		if err != nil {
			log.Printf("Workspace %s: error finding %s: %v", binding.Name, item.ExeName, err)
			continue
		}

		if item.TitlePattern != "" {
			wins = filterByTitle(wins, item.TitlePattern)
		}

		action, _ := ResolveHotkeyAction(syntheticBinding, wins, foreground, bindingState{})

		switch action.Type {
		case ActionLaunch:
			launches = append(launches, pendingAction{action: action, itemExe: item.ExeName})
		case ActionFocus:
			focuses = append(focuses, pendingAction{action: action, itemExe: item.ExeName})
		case ActionNotify:
			log.Printf("Workspace %s: %s", binding.Name, action.Message)
			if hm.showBalloon != nil {
				hm.showBalloon(action.Title, action.Message)
			}
		}
	}

	// Execute all launches first
	for _, pa := range launches {
		log.Printf("Workspace %s: launching %s", binding.Name, pa.itemExe)
		if err := launchApp(pa.action.Command, pa.action.Args); err != nil {
			errMsg := fmt.Sprintf("Workspace %s: failed to launch %s: %v", binding.Name, pa.itemExe, err)
			log.Println(errMsg)
			if hm.showBalloon != nil {
				hm.showBalloon("Launch Failed", errMsg)
			}
		}
	}

	// Focus the first focusable item, but only if the foreground isn't already
	// one of our workspace items (avoid stealing focus unnecessarily)
	foregroundIsOurs := false
	for _, item := range binding.WorkspaceItems {
		wins, _ := findWindowsByExe(item.ExeName)
		for _, w := range wins {
			if w.HWND == foreground {
				foregroundIsOurs = true
				break
			}
		}
		if foregroundIsOurs {
			break
		}
	}

	if !foregroundIsOurs && len(focuses) > 0 {
		pa := focuses[0]
		if err := focusWindow(pa.action.Target); err != nil {
			log.Printf("Workspace %s: failed to focus %s: %v", binding.Name, pa.itemExe, err)
			if hm.showBalloon != nil {
				hm.showBalloon("Focus Failed", fmt.Sprintf("Could not focus %s: %v", pa.itemExe, err))
			}
		}
	}
}

// HandleHotkey processes a WM_HOTKEY message.
func (hm *HotkeyManager) HandleHotkey(id int32) {
	binding, ok := hm.bindings[id]
	if !ok {
		return
	}

	if binding.Type == "workspace" {
		hm.handleWorkspace(binding)
		return
	}

	wins, err := findWindowsByExe(binding.ExeName)
	if err != nil {
		log.Printf("Error finding windows for %s: %v", binding.ExeName, err)
		return
	}

	if binding.TitlePattern != "" {
		wins = filterByTitle(wins, binding.TitlePattern)
	}

	foreground := getForegroundHWND()
	state := hm.cycleState[binding.Name]

	action, newState := ResolveHotkeyAction(binding, wins, foreground, state)
	hm.cycleState[binding.Name] = newState

	switch action.Type {
	case ActionFocus:
		if err := focusWindow(action.Target); err != nil {
			log.Printf("Failed to focus %s: %v", binding.Name, err)
			if hm.showBalloon != nil {
				hm.showBalloon("Focus Failed", fmt.Sprintf("Could not focus %s: %v", binding.Name, err))
			}
		}
	case ActionLaunch:
		log.Printf("No window for %s, launching: %s", binding.Name, action.Command)
		if err := launchApp(action.Command, action.Args); err != nil {
			errMsg := fmt.Sprintf("Failed to launch %s: %v", binding.Name, err)
			log.Println(errMsg)
			if hm.showBalloon != nil {
				hm.showBalloon("Launch Failed", errMsg)
			}
		}
	case ActionNotify:
		log.Println(action.Message)
		if hm.showBalloon != nil {
			hm.showBalloon(action.Title, action.Message)
		}
	case ActionNone:
		// do nothing
	}
}
