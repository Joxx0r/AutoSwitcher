//go:build windows

package main

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

// ShowBindingEditor displays the binding editor dialog. Returns true if the user saved.
func ShowBindingEditor(owner walk.Form, binding *Binding) bool {
	var dlg *walk.Dialog
	var nameLE, exeLE, launchLE, hotkeyLE *walk.LineEdit
	var argsTE *walk.TextEdit
	var multiCB *walk.ComboBox
	var accepted bool

	capturedMods := make([]string, 0)
	capturedKey := ""

	multiOptions := []string{"Focus Most Recent", "Cycle Through"}
	multiIndex := 0
	if binding.MultiWindow == "cycle" {
		multiIndex = 1
	}

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Edit Binding",
		MinSize:  decl.Size{Width: 400, Height: 350},
		Layout:   decl.Grid{Columns: 2, MarginsZero: false},
		Children: []decl.Widget{
			decl.Label{Text: "Name:"},
			decl.LineEdit{AssignTo: &nameLE, Text: binding.Name},

			decl.Label{Text: "Hotkey:"},
			decl.Composite{
				Layout: decl.HBox{MarginsZero: true},
				Children: []decl.Widget{
					decl.LineEdit{
						AssignTo: &hotkeyLE,
						Text:     binding.Hotkey.Format(),
						ReadOnly: true,
					},
					decl.PushButton{
						Text:    "Record",
						MaxSize: decl.Size{Width: 80},
						OnClicked: func() {
							mods, key, ok := recordHotkeyByKeypress(dlg)
							if ok {
								capturedMods = mods
								capturedKey = key
								hk := HotkeyDef{Modifiers: mods, Key: key}
								_ = hotkeyLE.SetText(hk.Format())
							}
						},
					},
					decl.PushButton{
						Text:    "Edit...",
						MaxSize: decl.Size{Width: 80},
						OnClicked: func() {
							mods, key, ok := recordHotkeyManual(dlg)
							if ok {
								capturedMods = mods
								capturedKey = key
								hk := HotkeyDef{Modifiers: mods, Key: key}
								_ = hotkeyLE.SetText(hk.Format())
							}
						},
					},
				},
			},

			decl.Label{Text: "Executable Name:"},
			decl.LineEdit{AssignTo: &exeLE, Text: binding.ExeName},

			decl.Label{Text: "Launch Command:"},
			decl.Composite{
				Layout: decl.HBox{MarginsZero: true},
				Children: []decl.Widget{
					decl.LineEdit{AssignTo: &launchLE, Text: binding.LaunchCommand},
					decl.PushButton{
						Text:    "Browse...",
						MaxSize: decl.Size{Width: 80},
						OnClicked: func() {
							dlgFile := new(walk.FileDialog)
							dlgFile.Filter = "Executables (*.exe)|*.exe|All Files (*.*)|*.*"
							if ok, _ := dlgFile.ShowOpen(dlg); ok {
								_ = launchLE.SetText(dlgFile.FilePath)
								// Auto-fill exe name from launch command if empty
								if exeLE.Text() == "" {
									_ = exeLE.SetText(filepath.Base(dlgFile.FilePath))
								}
							}
						},
					},
				},
			},

			decl.Label{Text: "Launch Arguments:"},
			decl.TextEdit{
				AssignTo: &argsTE,
				Text:     strings.Join(binding.LaunchArgs, "\r\n"),
				VScroll:  true,
				MinSize:  decl.Size{Height: 50},
			},

			decl.Label{Text: "Multi-Window:"},
			decl.ComboBox{
				AssignTo:     &multiCB,
				Model:        multiOptions,
				CurrentIndex: multiIndex,
			},

			decl.Composite{
				ColumnSpan: 2,
				Layout:     decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						Text: "OK",
						OnClicked: func() {
							// Build candidate binding for validation
							candidate := *binding
							candidate.Name = nameLE.Text()
							candidate.ExeName = exeLE.Text()
							if capturedKey != "" {
								candidate.Hotkey.Modifiers = capturedMods
								candidate.Hotkey.Key = capturedKey
							}
							if err := ValidateBinding(&candidate); err != nil {
								walk.MsgBox(dlg, "Validation", err.Error(), walk.MsgBoxIconWarning)
								return
							}

							binding.Name = candidate.Name
							binding.Hotkey = candidate.Hotkey
							binding.ExeName = candidate.ExeName
							binding.LaunchCommand = launchLE.Text()
							if argsText := strings.TrimSpace(argsTE.Text()); argsText != "" {
								lines := strings.Split(argsText, "\n")
								var args []string
								for _, line := range lines {
									line = strings.TrimSpace(line)
									if line != "" {
										args = append(args, line)
									}
								}
								binding.LaunchArgs = args
							} else {
								binding.LaunchArgs = nil
							}
							if multiCB.CurrentIndex() == 1 {
								binding.MultiWindow = "cycle"
							} else {
								binding.MultiWindow = "most_recent"
							}

							accepted = true
							dlg.Accept()
						},
					},
					decl.PushButton{
						Text: "Cancel",
						OnClicked: func() {
							dlg.Cancel()
						},
					},
				},
			},
		},
	}.Run(owner)

	return accepted
}

// recordHotkeyByKeypress captures a hotkey by listening for actual key presses.
// Uses a temporary low-level keyboard hook to intercept all keys including Win+X combos.
// The hook is scoped to the recording dialog's lifetime only.
func recordHotkeyByKeypress(owner walk.Form) (modifiers []string, key string, ok bool) {
	var dlg *walk.Dialog
	var statusLabel *walk.Label
	var ready bool // set after dialog widgets are assigned

	var heldModifiers uint32
	var capturedKey uint32
	var done bool

	setLabel := func(text string) {
		if statusLabel != nil {
			_ = statusLabel.SetText(text)
		}
	}

	updateLabel := func() {
		text := FormatModifiers(heldModifiers)
		if text != "" {
			text += "+..."
		} else {
			text = "Press your hotkey combination..."
		}
		setLabel(text)
	}

	hookCB := func(vkCode uint32, isKeyDown bool) bool {
		if done || !ready {
			return true // suppress keys before dialog is ready
		}

		if isKeyDown {
			// Check if it's a modifier key
			if modBit := VKToModifierBit(vkCode); modBit != 0 {
				heldModifiers |= modBit
				updateLabel()
				return true
			}

			// Escape with no modifiers cancels
			if vkCode == 0x1B && heldModifiers == 0 {
				done = true
				dlg.Cancel()
				return true
			}

			// Only accept keys that are in our supported vocabulary
			if !IsSupportedVK(vkCode) {
				setLabel("Unsupported key — try another or use manual entry")
				return true
			}

			// Non-function keys require at least one modifier to avoid
			// stealing normal typing (e.g., bare "A" would block all A input)
			isFunctionKey := vkCode >= 0x70 && vkCode <= 0x87
			if !isFunctionKey && heldModifiers == 0 {
				setLabel("Hold a modifier (Ctrl, Alt, Win, Shift) first")
				return true
			}

			// Non-modifier key completes the recording
			capturedKey = vkCode
			done = true
			dlg.Accept()
			return true
		}

		// Key up — clear modifier bits
		if modBit := VKToModifierBit(vkCode); modBit != 0 {
			heldModifiers &^= modBit
			updateLabel()
		}
		return true
	}

	var hookInstalled bool
	var hookFailed bool

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Record Hotkey",
		MinSize:  decl.Size{Width: 350, Height: 150},
		Layout:   decl.VBox{},
		OnBoundsChanged: func() {
			if !hookInstalled && !hookFailed && dlg != nil {
				if err := installKeyboardHook(hookCB, uintptr(dlg.Handle())); err != nil {
					log.Printf("keyboard hook failed, closing recorder: %v", err)
					hookFailed = true
					dlg.Cancel()
					return
				}
				hookInstalled = true
				ready = true
			}
		},
		Children: []decl.Widget{
			decl.Label{
				AssignTo:  &statusLabel,
				Text:      "Press your hotkey combination...",
				Alignment: decl.AlignHCenterVCenter,
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						Text: "Cancel",
						OnClicked: func() {
							done = true
							dlg.Cancel()
						},
					},
				},
			},
		},
	}.Run(owner)

	// Always clean up the hook after the dialog closes
	uninstallKeyboardHook()

	// If hook failed to install, fall back to manual input
	if hookFailed {
		return recordHotkeyManual(owner)
	}

	if capturedKey != 0 && !ok {
		modifiers = ModifierBitsToStrings(heldModifiers)
		key = FormatVK(capturedKey)
		ok = true
	}

	return modifiers, key, ok
}

// recordHotkeyManual provides a text-based hotkey input dialog.
func recordHotkeyManual(owner walk.Form) (modifiers []string, key string, ok bool) {
	var dlg *walk.Dialog
	var modsLE *walk.LineEdit
	var keyLE *walk.LineEdit

	// Capture values before dialog closes — Walk disposes widgets after .Run() returns
	var capturedModsText string
	var capturedKeyText string

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Enter Hotkey",
		MinSize:  decl.Size{Width: 350, Height: 180},
		Layout:   decl.Grid{Columns: 2},
		Children: []decl.Widget{
			decl.Label{Text: "Modifiers:"},
			decl.LineEdit{
				AssignTo:    &modsLE,
				Text:        "win",
				ToolTipText: "Comma-separated: win, ctrl, alt, shift",
			},
			decl.Label{Text: "Key:"},
			decl.LineEdit{
				AssignTo:    &keyLE,
				ToolTipText: "e.g., 1, A, F5, SPACE",
			},
			decl.Label{ColumnSpan: 2, Text: "Modifiers: win, ctrl, alt, shift (comma-separated)\nKeys: A-Z, 0-9, F1-F24, SPACE, ENTER, etc."},
			decl.Composite{
				ColumnSpan: 2,
				Layout:     decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						Text: "OK",
						OnClicked: func() {
							if keyLE.Text() == "" {
								walk.MsgBox(dlg, "Validation", "Key is required", walk.MsgBoxIconWarning)
								return
							}
							if _, err := ParseKey(keyLE.Text()); err != nil {
								walk.MsgBox(dlg, "Validation", "Invalid key: "+err.Error(), walk.MsgBoxIconWarning)
								return
							}
							if err := ValidateModifiers(modsLE.Text()); err != nil {
								walk.MsgBox(dlg, "Validation", err.Error(), walk.MsgBoxIconWarning)
								return
							}
							// Capture values while widgets are still alive
							capturedModsText = modsLE.Text()
							capturedKeyText = keyLE.Text()
							dlg.Accept()
						},
					},
					decl.PushButton{
						Text:      "Cancel",
						OnClicked: func() { dlg.Cancel() },
					},
				},
			},
		},
	}.Run(owner)

	if capturedKeyText == "" {
		return nil, "", false
	}

	modParts := strings.Split(capturedModsText, ",")
	for _, m := range modParts {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == "" {
			continue
		}
		// Canonicalize aliases to standard names
		switch m {
		case "control":
			m = "ctrl"
		case "super":
			m = "win"
		}
		if validModifiers[m] {
			modifiers = append(modifiers, m)
		}
	}

	key = strings.TrimSpace(capturedKeyText)
	return modifiers, key, true
}

