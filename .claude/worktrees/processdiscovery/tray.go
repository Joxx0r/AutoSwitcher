//go:build windows

package main

import (
	"log"

	"github.com/lxn/walk"
)

// TrayIcon manages the system tray icon and its context menu.
type TrayIcon struct {
	ni  *walk.NotifyIcon
	app *App
}

// NewTrayIcon creates the system tray icon with context menu.
func NewTrayIcon(mw *walk.MainWindow, app *App) (*TrayIcon, error) {
	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		return nil, err
	}

	t := &TrayIcon{ni: ni, app: app}

	// Set icon — use the application icon or a default
	icon, err := walk.NewIconFromResourceId(2)
	if err == nil {
		_ = ni.SetIcon(icon)
	} else {
		_ = ni.SetIcon(walk.IconInformation())
	}

	_ = ni.SetToolTip("AutoSwitcher")

	// Left-click opens settings
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			app.ShowSettings()
		}
	})

	// Build context menu
	if err := t.buildMenu(); err != nil {
		_ = ni.Dispose()
		return nil, err
	}

	_ = ni.SetVisible(true)

	return t, nil
}

func (t *TrayIcon) buildMenu() error {
	actions := t.ni.ContextMenu().Actions()

	// Settings
	settingsAction := walk.NewAction()
	_ = settingsAction.SetText("Settings")
	settingsAction.Triggered().Attach(func() {
		t.app.ShowSettings()
	})
	_ = actions.Add(settingsAction)

	// Separator
	_ = actions.Add(walk.NewSeparatorAction())

	// Enabled (checkbox)
	enabledAction := walk.NewAction()
	_ = enabledAction.SetText("Enabled")
	_ = enabledAction.SetCheckable(true)
	_ = enabledAction.SetChecked(true)
	enabledAction.Triggered().Attach(func() {
		enabled := enabledAction.Checked()
		t.app.SetEnabled(enabled)
	})
	_ = actions.Add(enabledAction)

	// Start with Windows (checkbox)
	autostartAction := walk.NewAction()
	_ = autostartAction.SetText("Start with Windows")
	_ = autostartAction.SetCheckable(true)
	_ = autostartAction.SetChecked(IsAutostartEnabled())
	autostartAction.Triggered().Attach(func() {
		checked := autostartAction.Checked()
		if err := SetAutostart(checked); err != nil {
			log.Printf("Autostart error: %v", err)
			t.ShowBalloon("Autostart Error", err.Error())
			_ = autostartAction.SetChecked(!checked) // revert
			return
		}
		t.app.config.Autostart = checked
		_ = SaveConfig(t.app.configPath, t.app.config)
	})
	_ = actions.Add(autostartAction)

	// Separator
	_ = actions.Add(walk.NewSeparatorAction())

	// Exit
	exitAction := walk.NewAction()
	_ = exitAction.SetText("Exit")
	exitAction.Triggered().Attach(func() {
		t.app.Exit()
	})
	_ = actions.Add(exitAction)

	return nil
}

// ShowBalloon displays a balloon notification from the tray icon.
func (t *TrayIcon) ShowBalloon(title, msg string) {
	if t.ni != nil {
		_ = t.ni.ShowCustom(title, msg, walk.IconInformation())
	}
}

// Dispose cleans up the tray icon.
func (t *TrayIcon) Dispose() {
	if t.ni != nil {
		_ = t.ni.Dispose()
		t.ni = nil
	}
}
