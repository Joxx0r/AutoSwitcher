//go:build windows

//go:generate go-winres make

package main

import (
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	maxLogSize = 10 * 1024 * 1024 // 10MB
	mutexName  = "Local\\AutoSwitcher"
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procRegisterWindowMsg = user32.NewProc("RegisterWindowMessageW")
	procFindWindow        = user32.NewProc("FindWindowW")
	procPostMessage       = user32.NewProc("PostMessageW")

	// wmShowSettings is a custom registered message for second-instance IPC.
	wmShowSettings uint32
)

func init() {
	msg, _ := windows.UTF16PtrFromString("AutoSwitcher_ShowSettings")
	ret, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(msg)))
	wmShowSettings = uint32(ret)
}

func main() {
	// Single-instance check via named mutex
	mutexNamePtr, _ := windows.UTF16PtrFromString(mutexName)
	_, err := windows.CreateMutex(nil, false, mutexNamePtr)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			// Another instance is running — tell it to show settings
			notifyExistingInstance()
			return
		}
		// Mutex creation failed for another reason — log and continue without protection
		log.Printf("WARNING: CreateMutex failed: %v — running without single-instance protection", err)
	}

	// Set up logging
	setupLogging()

	// Load config
	cfgPath, err := ConfigPath()
	if err != nil {
		log.Fatalf("config path: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Printf("WARNING: %v — using default config", err)
	}

	log.Printf("AutoSwitcher starting with %d bindings", len(cfg.Bindings))

	// Run the application
	app := NewApp(cfg, cfgPath)
	if err := app.Run(); err != nil {
		log.Fatalf("app error: %v", err)
	}
}

func notifyExistingInstance() {
	// Find the hidden window of the running instance by its window title
	// FindWindowW(lpClassName, lpWindowName) — pass nil class, match by title
	windowTitle, _ := windows.UTF16PtrFromString("AutoSwitcher_HiddenWindow")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(windowTitle)))
	if hwnd != 0 {
		_, _, _ = procPostMessage.Call(hwnd, uintptr(wmShowSettings), 0, 0)
	}
}

func setupLogging() {
	dir, err := ConfigDir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	logPath := filepath.Join(dir, "autoswitcher.log")

	// Truncate if too large
	if info, err := os.Stat(logPath); err == nil && info.Size() > maxLogSize {
		_ = os.Truncate(logPath, 0)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}
