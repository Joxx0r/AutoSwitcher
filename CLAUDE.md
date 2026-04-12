# AutoSwitcher

## What This Is

A standalone Windows hotkey manager written in Go. Binds global hotkeys to focus existing application windows or launch them if not running. Replaces AutoHotkey (which is banned by anti-cheat vendors) using only standard Win32 APIs — no hooks, no DLL injection, no drivers.

## Quick Start

```bash
go generate ./...                              # embed manifest + version info
CGO_ENABLED=1 go build -o AutoSwitcher.exe .   # build (requires GCC for Walk)
.\AutoSwitcher.exe                             # run (tray icon appears)
```

Re-running kills the previous instance automatically — no manual cleanup needed during development.

## Architecture

Flat `package main` structure. All files in the project root.

### Core Logic (testable, no GUI)

| File | Purpose |
|------|---------|
| `keys.go` | Key/modifier parsing, formatting, constants. Platform-neutral (no build tag). |
| `validation.go` | `ValidateBinding()`, `ValidateModifiers()`. Platform-neutral. |
| `config.go` | Config/Binding types, JSON load/save, defaults. |
| `hotkey.go` | `HotkeyManager`, `ResolveHotkeyAction()` (pure decision logic), `HandleHotkey()` (thin executor). |
| `window.go` | `EnumWindows`, process matching, `FocusWindow`. Win32 but mockable via function variables. |
| `launcher.go` | `LaunchApp` — spawns processes. Mockable via function variable. |
| `autostart.go` | Task Scheduler integration (`schtasks.exe`). |

### GUI Layer (Walk, thin wrappers)

| File | Purpose |
|------|---------|
| `app.go` | `App` struct, message loop, WndProc override, tray/hotkey coordination. |
| `tray.go` | System tray icon, context menu, balloon notifications. |
| `settings.go` | Settings dialog with binding table (add/edit/delete). |
| `editor.go` | Binding editor dialog (hotkey recorder, exe picker). |

### Entry Point

| File | Purpose |
|------|---------|
| `main.go` | Single-instance mutex, IPC (kills old instance), logging, bootstrap. |

## Testing Architecture

### Action-Result Pattern

`ResolveHotkeyAction()` is a **pure function** that takes:
- A binding, list of matching windows, current foreground HWND, cycle state

And returns:
- A `HotkeyAction` struct describing what to do (focus, launch, notify, or nothing)
- Updated cycle state

`HandleHotkey()` is a thin executor that calls the pure function then performs the action. All Win32 dependencies are injected via function variables.

### Function Variables (Dependency Injection)

All Win32 side effects are indirected through package-level function variables:

```go
var findWindowsByExe = findWindowsByExeImpl   // window.go
var focusWindow = FocusWindowImpl             // window.go
var getForegroundHWND = GetForegroundHWNDImpl  // window.go
var launchApp = LaunchAppImpl                 // launcher.go
```

Tests swap these to inject mocks:
```go
findWindowsByExe = func(exe string) ([]WindowInfo, error) { return mockWindows, nil }
defer func() { findWindowsByExe = origFind }()
```

### Test Files

| File | Build Tag | Coverage |
|------|-----------|----------|
| `keys_test.go` | none | Key parsing, modifier parsing, formatting |
| `validation_test.go` | none | Binding validation, modifier validation |
| `config_test.go` | none | Config round-trips, corruption recovery, atomic save |
| `hotkey_dispatch_test.go` | `windows` | Dispatch logic (cycle, most_recent, launch, error paths) |

### Running Tests

```bash
go test -v ./...          # all tests (without admin manifest, runs unelevated)
go test -run TestResolve  # just dispatch logic
go test -run TestValidate # just validation
```

Note: do NOT have `rsrc_windows_amd64.syso` present when running tests — the admin manifest would require elevation. Tests are designed to run without it.

## Build System

- **CGO required** — Walk (GUI framework) uses CGo. `CGO_ENABLED=1` must be set.
- **go-winres** — Embeds the admin manifest and version info. Install: `go install github.com/tc-hib/go-winres@v0.3.3`
- **Manifest** — `winres/app.manifest` sets Common Controls v6 (required by Walk) and `asInvoker` execution level.
- **CI** — GitHub Actions on `windows-latest` with Go 1.25. Build, test, lint (golangci-lint v2), vet.

## Key Design Decisions

- **RegisterHotKey API** — Standard Win32, anti-cheat safe. No permanent `SetWindowsHookEx` or keyboard hooks. (A temporary `WH_KEYBOARD_LL` hook is used only during the hotkey recording dialog to capture Win+X and Alt+X combos; it is removed when the dialog closes.)
- **Walk for GUI** — Native Win32 look, single binary. Trade-off: requires CGo.
- **asInvoker manifest** — No UAC prompt during development. Task Scheduler autostart uses `/rl highest` for elevation when needed.
- **TerminateProcess for instance replacement** — More reliable than window messages for killing old instances during development iteration.
- **Session-scoped mutex** (`Local\AutoSwitcher`) — Allows multiple users on RDP/Fast User Switching.

## Config

Stored at `%APPDATA%\AutoSwitcher\config.json`. Log at `%APPDATA%\AutoSwitcher\autoswitcher.log`.
