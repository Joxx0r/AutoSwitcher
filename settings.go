//go:build windows

package main

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// BindingModel implements walk.ReflectTableModel for displaying bindings.
type BindingModel struct {
	walk.ReflectTableModelBase
	items []Binding
}

func NewBindingModel(bindings []Binding) *BindingModel {
	return &BindingModel{items: bindings}
}

func (m *BindingModel) Items() interface{} {
	// Convert to a slice of display structs for the table
	type row struct {
		Name        string
		Hotkey      string
		ExeName     string
		MultiWindow string
	}
	rows := make([]row, len(m.items))
	for i, b := range m.items {
		mw := "Most Recent"
		if b.MultiWindow == "cycle" {
			mw = "Cycle"
		}
		rows[i] = row{
			Name:        b.Name,
			Hotkey:      b.Hotkey.Format(),
			ExeName:     b.ExeName,
			MultiWindow: mw,
		}
	}
	return rows
}

// ShowSettingsWindow displays the settings dialog with binding management.
func ShowSettingsWindow(owner walk.Form, bindings []Binding, onSave func([]Binding)) {
	// Make a copy of bindings to work with
	working := make([]Binding, len(bindings))
	copy(working, bindings)

	var dlg *walk.Dialog
	var tv *walk.TableView
	model := NewBindingModel(working)

	var refreshTable func()
	refreshTable = func() {
		model.items = working
		model.PublishRowsReset()
	}

	_, err := Dialog{
		AssignTo: &dlg,
		Title:    "AutoSwitcher Settings",
		MinSize:  Size{Width: 600, Height: 400},
		Layout:   VBox{},
		Children: []Widget{
			TableView{
				AssignTo:         &tv,
				AlternatingRowBG: true,
				ColumnsOrderable: true,
				Model:            model,
				Columns: []TableViewColumn{
					{Title: "Name", Width: 120},
					{Title: "Hotkey", Width: 100},
					{Title: "Exe Name", Width: 150},
					{Title: "Multi-Window", Width: 100},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{
						Text: "Add",
						OnClicked: func() {
							b := Binding{MultiWindow: "most_recent"}
							if ShowBindingEditor(dlg, &b) {
								working = append(working, b)
								refreshTable()
							}
						},
					},
					PushButton{
						Text: "Edit",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx < 0 || idx >= len(working) {
								return
							}
							b := working[idx]
							if ShowBindingEditor(dlg, &b) {
								working[idx] = b
								refreshTable()
							}
						},
					},
					PushButton{
						Text: "Delete",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx < 0 || idx >= len(working) {
								return
							}
							name := working[idx].Name
							if walk.MsgBox(dlg, "Delete Binding",
								"Delete binding '"+name+"'?",
								walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdNo {
								return
							}
							working = append(working[:idx], working[idx+1:]...)
							refreshTable()
						},
					},
					PushButton{
						Text: "Move Up",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx <= 0 || idx >= len(working) {
								return
							}
							working[idx], working[idx-1] = working[idx-1], working[idx]
							refreshTable()
							_ = tv.SetCurrentIndex(idx - 1)
						},
					},
					PushButton{
						Text: "Move Down",
						OnClicked: func() {
							idx := tv.CurrentIndex()
							if idx < 0 || idx >= len(working)-1 {
								return
							}
							working[idx], working[idx+1] = working[idx+1], working[idx]
							refreshTable()
							_ = tv.SetCurrentIndex(idx + 1)
						},
					},
					HSpacer{},
					PushButton{
						Text: "Save & Close",
						OnClicked: func() {
							onSave(working)
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

	if err != nil {
		return
	}
}
