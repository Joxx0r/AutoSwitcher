//go:build windows

package main

import (
	"github.com/lxn/walk"
	decl "github.com/lxn/walk/declarative"
)

// BindingRow is the display representation of a binding for the table.
type BindingRow struct {
	Name        string
	Hotkey      string
	ExeName     string
	MultiWindow string
}

// BindingModel implements walk.TableModel for displaying bindings.
type BindingModel struct {
	walk.TableModelBase
	rows []BindingRow
}

func NewBindingModel(bindings []Binding) *BindingModel {
	m := &BindingModel{}
	m.updateFrom(bindings)
	return m
}

func (m *BindingModel) updateFrom(bindings []Binding) {
	m.rows = make([]BindingRow, len(bindings))
	for i, b := range bindings {
		mw := "Most Recent"
		if b.MultiWindow == "cycle" {
			mw = "Cycle"
		}
		m.rows[i] = BindingRow{
			Name:        b.Name,
			Hotkey:      b.Hotkey.Format(),
			ExeName:     b.ExeName,
			MultiWindow: mw,
		}
	}
}

func (m *BindingModel) RowCount() int {
	return len(m.rows)
}

func (m *BindingModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	r := m.rows[row]
	switch col {
	case 0:
		return r.Name
	case 1:
		return r.Hotkey
	case 2:
		return r.ExeName
	case 3:
		return r.MultiWindow
	}
	return ""
}

// ShowSettingsWindow displays the settings dialog with binding management.
// onSave is called when the user clicks Apply or Save & Close; it should
// persist the bindings and return a ReloadResult so the dialog can surface
// any registration or save failures without closing.
func ShowSettingsWindow(owner walk.Form, bindings []Binding, onSave func([]Binding) ReloadResult, onCreated func(hwnd uintptr)) {
	// Deep clone so the dialog can mutate working freely without ever
	// touching live state — Reload also clones again at commit time, but
	// being explicit at dialog open makes the isolation guarantee obvious.
	working := cloneBindings(bindings)

	var dlg *walk.Dialog
	var tv *walk.TableView
	model := NewBindingModel(working)

	refreshTable := func() {
		model.updateFrom(working)
		model.PublishRowsReset()
	}

	reportResult := func(result ReloadResult) {
		if !result.HasErrors() {
			return
		}
		msg := ""
		if len(result.RegistrationErrors) > 0 {
			msg += "The following bindings could not be registered:\n\n"
			for _, e := range result.RegistrationErrors {
				msg += "  • " + e.Error() + "\n"
			}
		}
		if result.SaveError != nil {
			if msg != "" {
				msg += "\n"
			}
			msg += "Failed to save configuration:\n  " + result.SaveError.Error()
		}
		walk.MsgBox(dlg, "Settings Errors", msg, walk.MsgBoxIconWarning)
	}

	_, _ = decl.Dialog{
		OnBoundsChanged: func() {
			if dlg != nil && onCreated != nil {
				onCreated(uintptr(dlg.Handle()))
				onCreated = nil
			}
		},
		AssignTo: &dlg,
		Title:    "AutoSwitcher Settings",
		MinSize:  decl.Size{Width: 600, Height: 400},
		Layout:   decl.VBox{},
		Children: []decl.Widget{
			decl.TableView{
				AssignTo:         &tv,
				AlternatingRowBG: true,
				Model:            model,
				Columns: []decl.TableViewColumn{
					{Title: "Name", Width: 120},
					{Title: "Hotkey", Width: 100},
					{Title: "Exe Name", Width: 150},
					{Title: "Multi-Window", Width: 100},
				},
			},
			decl.Composite{
				Layout: decl.HBox{},
				Children: []decl.Widget{
					decl.PushButton{
						Text: "Add",
						OnClicked: func() {
							b := Binding{MultiWindow: "most_recent"}
							if ShowBindingEditor(dlg, &b) {
								working = append(working, b)
								refreshTable()
							}
						},
					},
					decl.PushButton{
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
					decl.PushButton{
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
					decl.PushButton{
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
					decl.PushButton{
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
					decl.HSpacer{},
					decl.PushButton{
						Text: "Apply",
						OnClicked: func() {
							reportResult(onSave(working))
						},
					},
					decl.PushButton{
						Text: "Save && Close",
						OnClicked: func() {
							result := onSave(working)
							if result.HasErrors() {
								reportResult(result)
								return // keep dialog open so user can fix and retry
							}
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
}
