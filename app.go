//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const wmHotkey = 0x0312

var procSetWindowTextW = user32.NewProc("SetWindowTextW")

// App ties all subsystems together.
type App struct {
	config     *Config
	configPath string
	hotkeys    *HotkeyManager
	tray       *TrayIcon
	mw         *walk.MainWindow
	enabled    bool
}

// NewApp creates a new App instance.
func NewApp(config *Config, configPath string) *App {
	return &App{
		config:     config,
		configPath: configPath,
		enabled:    true,
	}
}

// Run starts the application: creates the hidden window, tray icon, registers hotkeys,
// and enters the Win32 message loop.
func (a *App) Run() error {
	var err error

	// Create hidden main window (required by Walk for message loop and tray icon ownership)
	a.mw, err = walk.NewMainWindow()
	if err != nil {
		return err
	}
	defer a.mw.Dispose()

	// Set window text so second instance can find us via FindWindow
	windowTitle, _ := windows.UTF16PtrFromString("AutoSwitcher_HiddenWindow")
	_, _, _ = procSetWindowTextW.Call(uintptr(a.mw.Handle()), uintptr(unsafe.Pointer(windowTitle)))

	// Override WndProc to intercept WM_HOTKEY and our custom IPC message.
	// We need to declare origWndProc before the callback references it.
	var origWndProc uintptr
	newProc := syscall.NewCallback(func(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case wmHotkey:
			if a.enabled {
				a.hotkeys.HandleHotkey(int32(wParam))
			}
			return 0
		case uint32(wmShowSettings):
			a.ShowSettings()
			return 0
		}
		return win.CallWindowProc(origWndProc, hwnd, msg, wParam, lParam)
	})
	origWndProc = win.SetWindowLongPtr(a.mw.Handle(), win.GWL_WNDPROC, newProc)

	// Create tray icon
	a.tray, err = NewTrayIcon(a.mw, a)
	if err != nil {
		return err
	}
	defer a.tray.Dispose()

	// Register hotkeys
	a.hotkeys = NewHotkeyManager(uintptr(a.mw.Handle()), a.tray.ShowBalloon)
	a.hotkeys.RegisterAll(a.config.Bindings)

	// Run the message loop
	a.mw.Run()
	return nil
}

// Reload unregisters all hotkeys, updates config, re-registers, and saves.
func (a *App) Reload(newBindings []Binding) {
	a.hotkeys.UnregisterAll()
	a.config.Bindings = newBindings
	if a.enabled {
		a.hotkeys.RegisterAll(a.config.Bindings)
	}
	if err := SaveConfig(a.configPath, a.config); err != nil {
		log.Printf("Error saving config: %v", err)
		a.tray.ShowBalloon("Config Error", "Failed to save configuration")
	}
	log.Printf("Config reloaded with %d bindings", len(a.config.Bindings))
}

// SetEnabled enables or disables all hotkeys.
func (a *App) SetEnabled(enabled bool) {
	a.enabled = enabled
	if enabled {
		a.hotkeys.RegisterAll(a.config.Bindings)
		log.Println("Hotkeys enabled")
	} else {
		a.hotkeys.UnregisterAll()
		log.Println("Hotkeys disabled")
	}
}

// ShowSettings opens the settings window.
func (a *App) ShowSettings() {
	ShowSettingsWindow(a.mw, a.config.Bindings, func(bindings []Binding) {
		a.Reload(bindings)
	})
}

// Exit cleanly shuts down the application.
func (a *App) Exit() {
	a.hotkeys.UnregisterAll()
	if a.tray != nil {
		a.tray.Dispose()
		a.tray = nil
	}
	walk.App().Exit(0)
}
