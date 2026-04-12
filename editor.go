//go:build windows

package main

import (
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

