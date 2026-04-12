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
	icon, _ := walk.NewIconFromResourceId(2)
	if icon != nil {
		ni.SetIcon(icon)
	} else {
		// Fallback to a stock icon
		ni.SetIcon(walk.IconInformation())
	}

	ni.SetToolTip("AutoSwitcher")

	// Left-click opens settings
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			app.ShowSettings()
		}
	})

	// Build context menu
	if err := t.buildMenu(); err != nil {
		ni.Dispose()
		return nil, err
	}

	ni.SetVisible(true)

	return t, nil
}

func (t *TrayIcon) buildMenu() error {
	// Settings
	settingsAction := walk.NewAction()
	settingsAction.SetText("Settings")
	settingsAction.Triggered().Attach(func() {
		t.app.ShowSettings()
	})
	t.ni.ContextMenu().Actions().Add(settingsAction)

	// Separator
	t.ni.ContextMenu().Actions().Add(walk.NewSeparatorAction())

	// Enabled (checkbox)
	enabledAction := walk.NewAction()
	enabledAction.SetText("Enabled")
	enabledAction.SetCheckable(true)
	enabledAction.SetChecked(true)
	enabledAction.Triggered().Attach(func() {
		enabled := enabledAction.Checked()
		t.app.SetEnabled(enabled)
	})
	t.ni.ContextMenu().Actions().Add(enabledAction)

	// Start with Windows (checkbox)
	autostartAction := walk.NewAction()
	autostartAction.SetText("Start with Windows")
	autostartAction.SetCheckable(true)
	autostartAction.SetChecked(t.app.config.Autostart)
	autostartAction.Triggered().Attach(func() {
		checked := autostartAction.Checked()
		if err := SetAutostart(checked); err != nil {
			log.Printf("Autostart error: %v", err)
			t.ShowBalloon("Autostart Error", err.Error())
			autostartAction.SetChecked(!checked) // revert
			return
		}
		t.app.config.Autostart = checked
		_ = SaveConfig(t.app.configPath, t.app.config)
	})
	t.ni.ContextMenu().Actions().Add(autostartAction)

	// Separator
	t.ni.ContextMenu().Actions().Add(walk.NewSeparatorAction())

	// Exit
	exitAction := walk.NewAction()
	exitAction.SetText("Exit")
	exitAction.Triggered().Attach(func() {
		t.app.Exit()
	})
	t.ni.ContextMenu().Actions().Add(exitAction)

	return nil
}

// ShowBalloon displays a balloon notification from the tray icon.
func (t *TrayIcon) ShowBalloon(title, msg string) {
	if t.ni != nil {
		t.ni.ShowCustom(title, msg, walk.IconInformation())
	}
}

// Dispose cleans up the tray icon.
func (t *TrayIcon) Dispose() {
	if t.ni != nil {
		t.ni.Dispose()
	}
}
