//go:build windows

package main

import (
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// ShowBindingEditor displays the binding editor dialog. Returns true if the user saved.
func ShowBindingEditor(owner walk.Form, binding *Binding) bool {
	var dlg *walk.Dialog
	var nameLE, exeLE, launchLE, argsLE, hotkeyLE *walk.LineEdit
	var multiCB *walk.ComboBox
	var accepted bool

	// Track the captured hotkey
	capturedMods := make([]string, 0)
	capturedKey := ""

	multiOptions := []string{"Focus Most Recent", "Cycle Through"}
	multiIndex := 0
	if binding.MultiWindow == "cycle" {
		multiIndex = 1
	}

	Dialog{
		AssignTo:  &dlg,
		Title:     "Edit Binding",
		MinSize:   Size{Width: 400, Height: 300},
		Layout:    Grid{Columns: 2, MarginsZero: false},
		Children: []Widget{
			Label{Text: "Name:"},
			LineEdit{AssignTo: &nameLE, Text: binding.Name},

			Label{Text: "Hotkey:"},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{
						AssignTo: &hotkeyLE,
						Text:     binding.Hotkey.Format(),
						ReadOnly: true,
					},
					PushButton{
						Text:    "Record",
						MaxSize: Size{Width: 80},
						OnClicked: func() {
							mods, key, ok := recordHotkeyManual(dlg)
							if ok {
								capturedMods = mods
								capturedKey = key
								hk := HotkeyDef{Modifiers: mods, Key: key}
								hotkeyLE.SetText(hk.Format())
							}
						},
					},
				},
			},

			Label{Text: "Executable Name:"},
			LineEdit{AssignTo: &exeLE, Text: binding.ExeName},

			Label{Text: "Launch Command:"},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{AssignTo: &launchLE, Text: binding.LaunchCommand},
					PushButton{
						Text:    "Browse...",
						MaxSize: Size{Width: 80},
						OnClicked: func() {
							dlgFile := new(walk.FileDialog)
							dlgFile.Filter = "Executables (*.exe)|*.exe|All Files (*.*)|*.*"
							if ok, _ := dlgFile.ShowOpen(dlg); ok {
								launchLE.SetText(dlgFile.FilePath)
							}
						},
					},
				},
			},

			Label{Text: "Launch Arguments:"},
			LineEdit{
				AssignTo:    &argsLE,
				Text:        strings.Join(binding.LaunchArgs, "\n"),
				ToolTipText: "One argument per line. Spaces within an argument are preserved.",
			},

			Label{Text: "Multi-Window:"},
			ComboBox{
				AssignTo:     &multiCB,
				Model:        multiOptions,
				CurrentIndex: multiIndex,
			},

			// Buttons row spanning both columns
			Composite{
				ColumnSpan: 2,
				Layout:     HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "OK",
						OnClicked: func() {
							// Validate
							if nameLE.Text() == "" {
								walk.MsgBox(dlg, "Validation", "Name is required", walk.MsgBoxIconWarning)
								return
							}
							if hotkeyLE.Text() == "" && len(capturedMods) == 0 && capturedKey == "" {
								// Keep existing hotkey if nothing was recorded
								if binding.Hotkey.Key == "" {
									walk.MsgBox(dlg, "Validation", "Hotkey is required", walk.MsgBoxIconWarning)
									return
								}
							}

							binding.Name = nameLE.Text()
							if capturedKey != "" {
								binding.Hotkey.Modifiers = capturedMods
								binding.Hotkey.Key = capturedKey
							}
							binding.ExeName = exeLE.Text()
							binding.LaunchCommand = launchLE.Text()
							if argsText := strings.TrimSpace(argsLE.Text()); argsText != "" {
								// Split by newlines to preserve spaces within arguments
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
					PushButton{
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

// recordHotkeyManual provides a text-based hotkey input dialog.
// This is more reliable than trying to capture key events, since the Win key
// is intercepted by the shell before reaching WM_KEYDOWN in a regular control.
func recordHotkeyManual(owner walk.Form) (modifiers []string, key string, ok bool) {
	var dlg *walk.Dialog
	var modsLE *walk.LineEdit
	var keyLE *walk.LineEdit

	result, _ := Dialog{
		AssignTo: &dlg,
		Title:    "Enter Hotkey",
		MinSize:  Size{Width: 350, Height: 180},
		Layout:   Grid{Columns: 2},
		Children: []Widget{
			Label{Text: "Modifiers:"},
			LineEdit{
				AssignTo:    &modsLE,
				Text:        "win",
				ToolTipText: "Comma-separated: win, ctrl, alt, shift",
			},
			Label{Text: "Key:"},
			LineEdit{
				AssignTo:    &keyLE,
				ToolTipText: "e.g., 1, A, F5, SPACE",
			},
			Label{ColumnSpan: 2, Text: "Modifiers: win, ctrl, alt, shift (comma-separated)\nKeys: A-Z, 0-9, F1-F24, SPACE, ENTER, etc."},
			Composite{
				ColumnSpan: 2,
				Layout:     HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "OK",
						OnClicked: func() {
							if keyLE.Text() == "" {
								walk.MsgBox(dlg, "Validation", "Key is required", walk.MsgBoxIconWarning)
								return
							}
							// Validate the key
							if _, err := ParseKey(keyLE.Text()); err != nil {
								walk.MsgBox(dlg, "Validation", "Invalid key: "+err.Error(), walk.MsgBoxIconWarning)
								return
							}
							dlg.Accept()
						},
					},
					PushButton{
						Text:      "Cancel",
						OnClicked: func() { dlg.Cancel() },
					},
				},
			},
		},
	}.Run(owner)

	if result != walk.DlgCmdOK {
		return nil, "", false
	}

	// Parse modifiers
	modParts := strings.Split(modsLE.Text(), ",")
	for _, m := range modParts {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == "win" || m == "ctrl" || m == "alt" || m == "shift" {
			modifiers = append(modifiers, m)
		}
	}

	key = strings.TrimSpace(keyLE.Text())
	return modifiers, key, true
}
