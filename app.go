//go:build windows

package main

import (
	"fmt"
	"log"
	"strings"
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
	config       *Config
	configPath   string
	hotkeys      *HotkeyManager
	tray         *TrayIcon
	mw           *walk.MainWindow
	enabled      bool
	settingsOpen bool
	settingsDlg  uintptr // HWND of the open settings dialog
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

	// Hide the owner window — this is a tray-only app
	a.mw.SetVisible(false)

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
	a.hotkeys.RegisterAll(a.config.Bindings, false)

	// Reconcile autostart: make config authoritative over Task Scheduler state
	taskExists := IsAutostartEnabled()
	if a.config.Autostart && !taskExists {
		if err := SetAutostart(true); err != nil {
			log.Printf("Failed to restore autostart: %v", err)
		}
	} else if !a.config.Autostart && taskExists {
		if err := SetAutostart(false); err != nil {
			log.Printf("Failed to remove stale autostart task: %v", err)
		}
	}

	// Run the message loop
	a.mw.Run()
	return nil
}

// Reload atomically applies a new set of bindings: persists to disk, swaps
// live state, registers new hotkeys. Returns a ReloadResult describing any
// failure so the caller (the settings dialog) can surface it inline.
//
// Transactional semantics:
//
//   - SaveConfig runs first. On disk-write failure, NO live state is touched
//     and the on-disk file is unchanged. Caller gets SaveError and can retry.
//   - On registration failure (e.g. hotkey conflict, invalid key), the
//     previous bindings are rolled back: live HotkeyManager is restored,
//     a.config.Bindings is restored, and the on-disk file is rewritten with
//     the previous state. The dialog can then stay open and the user can
//     fix the conflict and retry — exactly the contract the UI implies.
//
// The input slice is deep-cloned, so the caller's working copy can keep
// being mutated without affecting live state.
func (a *App) Reload(newBindings []Binding) ReloadResult {
	cloned := cloneBindings(newBindings)

	// Snapshot for rollback.
	prevBindings := cloneBindings(a.config.Bindings)

	// Persist first. If this fails, the running app keeps its previous
	// hotkeys and the disk file is unchanged.
	trial := *a.config
	trial.Bindings = cloned
	if err := SaveConfig(a.configPath, &trial); err != nil {
		log.Printf("Reload: save failed, live state unchanged: %v", err)
		a.tray.ShowBalloon("AutoSwitcher", "Failed to save configuration")
		return ReloadResult{SaveError: err}
	}

	// Save succeeded — commit to live state.
	a.hotkeys.UnregisterAll()
	a.config.Bindings = cloned
	var result ReloadResult
	if a.enabled {
		result.RegistrationErrors = a.hotkeys.RegisterAll(a.config.Bindings, true)
	}

	// On any registration failure, roll back to the previous state so
	// the dialog's stay-open semantics are actually a transaction.
	if len(result.RegistrationErrors) > 0 {
		log.Printf("Reload: %d registration error(s), rolling back", len(result.RegistrationErrors))
		a.hotkeys.UnregisterAll()
		a.config.Bindings = prevBindings
		if a.enabled {
			// Best-effort re-register old bindings. If this fails (e.g.
			// another app grabbed a hotkey in the meantime), we log it
			// but still report the original errors as the user-visible
			// failure — the user's actionable info is the original.
			if rollbackErrs := a.hotkeys.RegisterAll(a.config.Bindings, true); len(rollbackErrs) > 0 {
				log.Printf("Reload: rollback re-register hit %d error(s)", len(rollbackErrs))
			}
		}
		// Restore disk file. If this fails, log it; the in-memory state
		// is still consistent and the user can fix and retry.
		prevTrial := *a.config
		prevTrial.Bindings = prevBindings
		if err := SaveConfig(a.configPath, &prevTrial); err != nil {
			log.Printf("Reload: rollback save failed: %v", err)
		}
		a.tray.ShowBalloon("AutoSwitcher", reloadSummary(len(a.config.Bindings), a.enabled, result))
		return result
	}

	a.tray.ShowBalloon("AutoSwitcher", reloadSummary(len(a.config.Bindings), a.enabled, result))
	log.Printf("Config reloaded with %d bindings (enabled=%v)", len(a.config.Bindings), a.enabled)
	return result
}

// reloadSummary builds the one-line tray balloon text for a Reload result.
// Pure function — exported (lowercase) for testing.
func reloadSummary(total int, enabled bool, result ReloadResult) string {
	if !enabled {
		return fmt.Sprintf("%d bindings saved (hotkeys disabled)", total)
	}
	active := total - len(result.RegistrationErrors)
	parts := []string{fmt.Sprintf("%d hotkeys active", active)}
	if len(result.RegistrationErrors) > 0 {
		parts = append(parts, fmt.Sprintf("%d registration error(s)", len(result.RegistrationErrors)))
	}
	return strings.Join(parts, ", ")
}

// SetEnabled enables or disables all hotkeys.
func (a *App) SetEnabled(enabled bool) {
	a.enabled = enabled
	if enabled {
		a.hotkeys.RegisterAll(a.config.Bindings, false)
		log.Println("Hotkeys enabled")
	} else {
		a.hotkeys.UnregisterAll()
		log.Println("Hotkeys disabled")
	}
}

// ShowSettings opens the settings window. If already open, brings it to front.
func (a *App) ShowSettings() {
	if a.settingsOpen {
		// Settings dialog is already open — bring it to front
		if a.settingsDlg != 0 {
			_ = FocusWindow(a.settingsDlg)
		}
		return
	}
	a.settingsOpen = true
	// Pass nil as the owner so Walk's WM_CLOSE handler doesn't
	// SetWindowPos(owner, SWP_SHOWWINDOW) on our hidden message-sink window.
	ShowSettingsWindow(nil, a.config.Bindings, func(bindings []Binding) ReloadResult {
		return a.Reload(bindings)
	}, func(hwnd uintptr) {
		a.settingsDlg = hwnd
	})
	a.settingsOpen = false
	a.settingsDlg = 0
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
