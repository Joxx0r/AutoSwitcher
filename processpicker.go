//go:build windows

package main

import (
	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

// ProcessPickerModel implements walk.TableModel for the process picker dialog.
type ProcessPickerModel struct {
	walk.TableModelBase
	all      []ProcessInfo // deduplicated full list
	filtered []ProcessInfo // after search filter
}

func NewProcessPickerModel(procs []ProcessInfo) *ProcessPickerModel {
	deduped := deduplicateProcesses(procs)
	return &ProcessPickerModel{
		all:      deduped,
		filtered: deduped,
	}
}

func (m *ProcessPickerModel) RowCount() int {
	return len(m.filtered)
}

func (m *ProcessPickerModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.filtered) {
		return ""
	}
	p := m.filtered[row]
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

func (m *ProcessPickerModel) ApplyFilter(query string) {
	m.filtered = filterProcesses(m.all, query)
	m.PublishRowsReset()
}

// ShowProcessPicker displays a dialog listing running processes.
// Returns the selected ProcessInfo, or nil if cancelled.
func ShowProcessPicker(owner walk.Form) *ProcessInfo {
	procs, err := discoverProcesses()
	if err != nil || len(procs) == 0 {
		if err != nil {
			walk.MsgBox(owner, "Process Discovery", "Failed to enumerate processes: "+err.Error(), walk.MsgBoxIconError)
		} else {
			walk.MsgBox(owner, "Process Discovery", "No processes with visible windows found.", walk.MsgBoxIconInformation)
		}
		return nil
	}

	model := NewProcessPickerModel(procs)

	var dlg *walk.Dialog
	var tv *walk.TableView
	var filterLE *walk.LineEdit
	var selected *ProcessInfo

	_, _ = decl.Dialog{
		AssignTo: &dlg,
		Title:    "Pick a Running Process",
		MinSize:  decl.Size{Width: 650, Height: 400},
		Layout:   decl.VBox{},
		Children: []decl.Widget{
			decl.Composite{
				Layout: decl.HBox{MarginsZero: true},
				Children: []decl.Widget{
					decl.Label{Text: "Filter:"},
					decl.LineEdit{
						AssignTo:    &filterLE,
						ToolTipText: "Type to filter by exe name or window title",
						OnTextChanged: func() {
							model.ApplyFilter(filterLE.Text())
						},
					},
				},
			},
			decl.TableView{
				AssignTo:         &tv,
				AlternatingRowBG: true,
				ColumnsOrderable: true,
				Model:            model,
				Columns: []decl.TableViewColumn{
					{Title: "Exe Name", Width: 150},
					{Title: "Window Title", Width: 250},
					{Title: "Path", Width: 220},
				},
				OnItemActivated: func() {
					idx := tv.CurrentIndex()
					if idx >= 0 && idx < len(model.filtered) {
						p := model.filtered[idx]
						selected = &p
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
							if idx < 0 || idx >= len(model.filtered) {
								walk.MsgBox(dlg, "Selection", "Please select a process.", walk.MsgBoxIconInformation)
								return
							}
							p := model.filtered[idx]
							selected = &p
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

	return selected
}
