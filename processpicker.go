//go:build windows

package main

import (
	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

// ProcessPickerModel implements walk.TableModel for displaying running processes.
type ProcessPickerModel struct {
	walk.TableModelBase
	items []ProcessInfo
}

func (m *ProcessPickerModel) RowCount() int {
	return len(m.items)
}

func (m *ProcessPickerModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.items) {
		return ""
	}
	p := m.items[row]
	switch col {
	case 0:
		return p.ExeName
	case 1:
		return p.Title
	case 2:
		return p.ExePath
	}
	return ""
}

// ApplyFilter updates the model with filtered process data.
func (m *ProcessPickerModel) ApplyFilter(allProcs []ProcessInfo, query string) {
	filtered := filterProcesses(allProcs, query)
	m.items = filtered
	m.PublishRowsReset()
}

// ShowProcessPicker displays a dialog for selecting a running process.
// Returns nil if the user cancels.
func ShowProcessPicker(owner walk.Form) *ProcessInfo {
	var dlg *walk.Dialog
	var searchLE *walk.LineEdit
	var tv *walk.TableView
	var result *ProcessInfo

	allProcs := deduplicateProcesses(discoverProcesses())
	model := &ProcessPickerModel{items: allProcs}

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Pick Process",
		MinSize:  decl.Size{Width: 650, Height: 400},
		Layout:   decl.VBox{},
		Children: []decl.Widget{
			decl.Composite{
				Layout: decl.HBox{MarginsZero: true},
				Children: []decl.Widget{
					decl.Label{Text: "Search:"},
					decl.LineEdit{
						AssignTo: &searchLE,
						OnTextChanged: func() {
							model.ApplyFilter(allProcs, searchLE.Text())
						},
					},
				},
			},
			decl.TableView{
				AssignTo:         &tv,
				AlternatingRowBG: true,
				Model:            model,
				Columns: []decl.TableViewColumn{
					{Title: "Exe Name", Width: 150},
					{Title: "Window Title", Width: 250},
					{Title: "Path", Width: 230},
				},
				OnItemActivated: func() {
					idx := tv.CurrentIndex()
					if idx >= 0 && idx < len(model.items) {
						item := model.items[idx]
						result = &item
						dlg.Accept()
					}
				},
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.HSpacer{},
					decl.PushButton{
						Text: "OK",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx >= 0 && idx < len(model.items) {
								item := model.items[idx]
								result = &item
								dlg.Accept()
							}
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

	return result
}
