//go:build windows

package main

import (
	"strings"

	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

// WorkspaceItemModel implements walk.TableModel for workspace items.
type WorkspaceItemModel struct {
	walk.TableModelBase
	items []WorkspaceItem
}

func (m *WorkspaceItemModel) RowCount() int {
	return len(m.items)
}

func (m *WorkspaceItemModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return ""
	}
	item := m.items[row]
	switch col {
	case 0:
		return item.ExeName
	case 1:
		return item.LaunchCommand
	case 2:
		return item.TitlePattern
	}
	return ""
}

// showWorkspaceItemEditor shows a dialog for editing a single workspace item.
func showWorkspaceItemEditor(owner walk.Form, item *WorkspaceItem) bool {
	var dlg *walk.Dialog
	var exeLE, titleLE, launchLE *walk.LineEdit
	var argsTE *walk.TextEdit
	var accepted bool

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Edit Workspace Item",
		MinSize:  decl.Size{Width: 400, Height: 300},
		Layout:   decl.Grid{Columns: 2, MarginsZero: false},
		Children: []decl.Widget{
			decl.Label{Text: "Executable Name:"},
			decl.Composite{
				Layout: decl.HBox{MarginsZero: true},
				Children: []decl.Widget{
					decl.LineEdit{AssignTo: &exeLE, Text: item.ExeName},
					decl.PushButton{
						Text:    "Pick...",
						MaxSize: decl.Size{Width: 80},
						OnClicked: func() {
							proc := ShowProcessPicker(dlg)
							if proc != nil {
								_ = exeLE.SetText(proc.ExeName)
								if launchLE.Text() == "" {
									_ = launchLE.SetText(proc.ExePath)
								}
							}
						},
					},
				},
			},

			decl.Label{Text: "Title Filter:"},
			decl.LineEdit{
				AssignTo:    &titleLE,
				Text:        item.TitlePattern,
				ToolTipText: "Optional: only match windows with this text in the title",
			},

			decl.Label{Text: "Launch Command:"},
			decl.LineEdit{AssignTo: &launchLE, Text: item.LaunchCommand},

			decl.Label{Text: "Launch Arguments:"},
			decl.TextEdit{
				AssignTo: &argsTE,
				Text:     strings.Join(item.LaunchArgs, "\r\n"),
				VScroll:  true,
				MinSize:  decl.Size{Height: 50},
			},

			decl.Composite{
				ColumnSpan: 2,
				Layout:     decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						Text: "OK",
						OnClicked: func() {
							if strings.TrimSpace(exeLE.Text()) == "" {
								walk.MsgBox(dlg, "Validation", "Executable name is required", walk.MsgBoxIconWarning)
								return
							}
							item.ExeName = exeLE.Text()
							item.TitlePattern = titleLE.Text()
							item.LaunchCommand = launchLE.Text()
							if argsText := strings.TrimSpace(argsTE.Text()); argsText != "" {
								lines := strings.Split(argsText, "\n")
								var args []string
								for _, line := range lines {
									line = strings.TrimSpace(line)
									if line != "" {
										args = append(args, line)
									}
								}
								item.LaunchArgs = args
							} else {
								item.LaunchArgs = nil
							}
							accepted = true
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

	return accepted
}

// ShowWorkspaceEditor displays the workspace binding editor dialog.
// Returns true if the user saved.
func ShowWorkspaceEditor(owner walk.Form, binding *Binding) bool {
	var dlg *walk.Dialog
	var nameLE, hotkeyLE *walk.LineEdit
	var tv *walk.TableView
	var accepted bool

	capturedMods := make([]string, 0)
	capturedKey := ""

	// Work on a copy of workspace items
	items := make([]WorkspaceItem, len(binding.WorkspaceItems))
	copy(items, binding.WorkspaceItems)

	model := &WorkspaceItemModel{items: items}

	refreshTable := func() {
		model.items = items
		model.PublishRowsReset()
	}

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Edit Workspace Binding",
		MinSize:  decl.Size{Width: 550, Height: 400},
		Layout:   decl.VBox{},
		Children: []decl.Widget{
			decl.Composite{
				Layout: decl.Grid{Columns: 2, MarginsZero: false},
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
				},
			},
			decl.Label{Text: "Workspace Items:"},
			decl.TableView{
				AssignTo:         &tv,
				AlternatingRowBG: true,
				Model:            model,
				Columns: []decl.TableViewColumn{
					{Title: "Exe Name", Width: 150},
					{Title: "Launch Command", Width: 200},
					{Title: "Title Filter", Width: 120},
				},
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.PushButton{
						Text: "Add Item",
						OnClicked: func() {
							item := WorkspaceItem{}
							if showWorkspaceItemEditor(dlg, &item) {
								items = append(items, item)
								refreshTable()
							}
						},
					},
					decl.PushButton{
						Text: "Edit Item",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx < 0 || idx >= len(items) {
								return
							}
							item := items[idx]
							if showWorkspaceItemEditor(dlg, &item) {
								items[idx] = item
								refreshTable()
							}
						},
					},
					decl.PushButton{
						Text: "Delete Item",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx < 0 || idx >= len(items) {
								return
							}
							items = append(items[:idx], items[idx+1:]...)
							refreshTable()
						},
					},
					decl.HSpacer{},
				},
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						Text: "OK",
						OnClicked: func() {
							candidate := *binding
							candidate.Name = nameLE.Text()
							candidate.Type = "workspace"
							candidate.WorkspaceItems = items
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
							binding.Type = "workspace"
							binding.WorkspaceItems = items

							accepted = true
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

	return accepted
}
