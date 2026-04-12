//go:build windows

//go:generate go-winres make

package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
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

	// Custom registered message for second-instance IPC.
	wmShowSettings uint32
)

func init() {
	msg, _ := windows.UTF16PtrFromString("AutoSwitcher_ShowSettings")
	ret, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(msg)))
	wmShowSettings = uint32(ret)
}

func main() {
	// Single-instance check via named mutex.
	// CreateMutex returns a valid handle even when the mutex already exists,
	// so we must check GetLastError for ERROR_ALREADY_EXISTS.
	mutexNamePtr, _ := windows.UTF16PtrFromString(mutexName)
	handle, err := windows.CreateMutex(nil, false, mutexNamePtr)
	if err == windows.ERROR_ALREADY_EXISTS {
		// Another instance is running — shut it down and take over
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		shutdownExistingInstance()
		// Retry acquiring the mutex after the old instance exits
		handle, err = windows.CreateMutex(nil, false, mutexNamePtr)
		if err != nil && err != windows.ERROR_ALREADY_EXISTS {
			log.Printf("WARNING: CreateMutex retry failed: %v", err)
		}
	}
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		log.Printf("WARNING: CreateMutex failed: %v — running without single-instance protection", err)
	}
	_ = handle // keep handle alive for process lifetime

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

func shutdownExistingInstance() {
	// Find the old instance's window and get its process ID
	windowTitle, _ := windows.UTF16PtrFromString("AutoSwitcher_HiddenWindow")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(windowTitle)))
	if hwnd == 0 {
		time.Sleep(500 * time.Millisecond)
		return
	}

	// Get the PID of the old instance
	var pid uint32
	_, _, _ = procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return
	}

	// Open the process and terminate it
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		log.Printf("WARNING: could not open old instance (PID %d): %v", pid, err)
		return
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	if err := windows.TerminateProcess(handle, 0); err != nil {
		log.Printf("WARNING: could not terminate old instance (PID %d): %v", pid, err)
		return
	}

	// Wait for the process to actually exit
	_, _ = windows.WaitForSingleObject(handle, 5000)
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
