# AutoSwitcher — Specification

## Overview

**AutoSwitcher** is a standalone Windows application written in Go that provides global hotkey bindings to focus existing application windows or launch them if not running. It replaces tools like AutoHotkey which are banned by anti-cheat vendors.

### Problem Statement

Developers working across many applications (IDE, terminal, browser, Unreal Editor, Discord) need fast window switching beyond Alt+Tab. Existing solutions like AutoHotkey are flagged/banned by anti-cheat systems. AutoSwitcher solves this by being a clean, standalone binary using only standard Win32 APIs — no hooks, no DLL injection, no drivers.

---

## Functional Requirements

### 1. Global Hotkey Registration

- Use the **Win32 `RegisterHotKey` API** for all hotkey registration
  - No `SetWindowsHookEx`, no low-level keyboard hooks, no drivers
  - This is the safest approach for anti-cheat compatibility
- Supported key combinations:
  - Any modifier combo (`Ctrl`, `Alt`, `Shift`, `Win`) + any key
  - Function keys `F1`–`F24` (with or without modifiers)
- Hotkeys must work globally — regardless of which application has focus

### 2. Window Matching

- Match target windows by **executable name** (e.g., `Discord.exe`, `Code.exe`)
- Use `EnumWindows` + `GetWindowThreadProcessId` + process snapshot to find matching windows
- Only match visible, top-level windows (skip hidden/tool windows)

### 3. Focus Behavior

When a hotkey is pressed:

1. **Window found, not focused** → Bring it to foreground (`SetForegroundWindow` / `SwitchToThisWindow`). If minimized, restore it first.
2. **Window found, already focused** → Do nothing (no toggle/minimize).
3. **No matching window found** → Launch the configured command (if a launch command is set for this binding).
4. **No matching window, no launch command** → Show a toast notification: "No window found for [binding name]"

### 4. Multi-Window Handling

When multiple windows match the same executable, behavior is **configurable per binding**:

- **Focus most recent**: Always bring the most recently used window to front
- **Cycle**: Each press cycles to the next window of that executable (round-robin)

### 5. Application Launching

- Each binding can optionally specify a **launch command** (executable path + arguments)
- If the target app is not running and a launch command is configured, execute it
- Launch via `os/exec` with the configured command
- After launch, the next hotkey press should find and focus the newly created window

---

## Configuration

### Storage

- Config file: `%APPDATA%\AutoSwitcher\config.json`
- Created with defaults on first run if it doesn't exist

### Schema

```json
{
  "version": 1,
  "autostart": true,
  "bindings": [
    {
      "name": "VS Code",
      "hotkey": {
        "modifiers": ["win"],
        "key": "1"
      },
      "exe_name": "Code.exe",
      "launch_command": "C:\\Users\\...\\AppData\\Local\\Programs\\Microsoft VS Code\\Code.exe",
      "launch_args": [],
      "multi_window": "most_recent"
    },
    {
      "name": "Unreal Editor",
      "hotkey": {
        "modifiers": ["win"],
        "key": "2"
      },
      "exe_name": "UnrealEditor.exe",
      "launch_command": "",
      "launch_args": [],
      "multi_window": "cycle"
    }
  ]
}
```

### Live Editing

- Changes made through the settings window apply **immediately** without restart
- On save: unregister old hotkeys, re-register new ones, persist to disk

---

## User Interface

### System Tray Icon

- Displays an icon in the Windows system tray
- Left-click: open settings window
- Right-click context menu:
  - **Settings** — open settings window
  - **Enabled** — checkbox to temporarily disable all hotkeys
  - **Start with Windows** — toggle autostart
  - **Exit** — quit the application

### Settings Window

A small, focused window containing:

- **Binding list/table** showing all configured hotkeys:
  - Columns: Name, Hotkey, Exe Name, Launch Command, Multi-Window Mode
  - Row selection for edit/delete
- **Add** button → opens binding editor dialog
- **Edit** button → opens binding editor for selected row
- **Delete** button → removes selected binding (with confirmation)
- **Close** button → saves and closes

### Binding Editor Dialog

Fields:

- **Name**: Text field (display name for the binding)
- **Hotkey**: Recorder field — user presses desired key combo and it's captured
- **Executable Name**: Text field (e.g., `Code.exe`)
- **Launch Command**: File picker + text field for the executable path
- **Launch Arguments**: Text field for optional command-line arguments
- **Multi-Window Mode**: Dropdown — "Focus Most Recent" / "Cycle Through"

---

## System Integration

### Autostart

- Toggle in tray menu and settings
- Implemented via Windows Registry: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
- Stores the path to the AutoSwitcher executable

### Elevation

- Binary compiled with an **admin manifest** (`requireAdministrator`)
- Required to `SetForegroundWindow` on elevated/admin windows (e.g., Unreal Editor)
- UAC prompt will appear on launch (acceptable trade-off)

### Error Handling

- **Windows toast notifications** for user-facing errors:
  - Hotkey registration conflict (another app has the hotkey)
  - Launch command failed (exe not found, permission denied)
  - No window found (and no launch command configured)
- **Log file** at `%APPDATA%\AutoSwitcher\autoswitcher.log` for debugging
  - Log all hotkey registrations, window matches, launches, and errors

---

## Technical Approach

### Language & Build

- **Go** — compiled to a single standalone `.exe`
- Admin manifest embedded via Go resource compilation (`rsrc` or `goversioninfo`)
- No CGo if possible — pure Go with `syscall` / `golang.org/x/sys/windows` for Win32 calls

### Key Win32 APIs

| API | Purpose |
|-----|---------|
| `RegisterHotKey` / `UnregisterHotKey` | Global hotkey registration |
| `GetMessage` | Message loop for hotkey events |
| `EnumWindows` | Iterate all top-level windows |
| `GetWindowThreadProcessId` | Map window → process |
| `CreateToolhelp32Snapshot` | Map PID → executable name |
| `SetForegroundWindow` | Bring window to front |
| `ShowWindow` | Restore minimized windows |
| `IsIconic` | Check if window is minimized |
| `IsWindowVisible` | Filter hidden windows |
| `Shell_NotifyIcon` | System tray icon |

### GUI Framework

- Settings window and dialogs: use a lightweight Go GUI library
  - Options: [`walk`](https://github.com/lxn/walk) (native Win32), [`wails`](https://wails.io/) (webview-based), or raw Win32 via syscall
  - Recommendation: `walk` for native look and minimal binary size

### Toast Notifications

- Use `Shell_NotifyIcon` balloon tips or Windows 10+ toast notification API
- Fallback: tray balloon notifications (simpler, widely compatible)

---

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Hotkey conflict with another app | Show toast: "Hotkey [combo] is already in use by another application" |
| Target app crashes after launch | Next hotkey press finds no window, re-launches |
| Multiple monitors | Focus window wherever it is (no monitor-specific logic) |
| Minimized window | Restore (`SW_RESTORE`) then focus |
| Window on different virtual desktop | Not supported in v1 (single desktop assumption) |
| Config file corrupted/missing | Recreate with empty bindings, show toast notification |
| App already running (second instance) | Detect via named mutex, bring existing instance's settings window to front |

---

## Out of Scope (v1)

- Window tiling, resizing, or layout management
- Macro scripting or multi-step actions
- Key remapping
- Multi-monitor window placement/movement
- Virtual desktop support
- Window title matching (exe name only in v1)

### Potential Future Additions (v2+)

- Window title pattern matching (regex/substring)
- Virtual desktop awareness
- Window tiling/snapping
- Per-monitor binding profiles
- Import/export configuration
- Binding groups/profiles (e.g., "gaming" vs "dev" mode)

---

## Success Criteria

1. **Core loop works**: Press hotkey → correct window is focused within <100ms
2. **Launch works**: Press hotkey with no matching window → app launches
3. **Multi-window cycle**: Repeated presses cycle through all windows of the target exe
4. **Settings window**: Can add, edit, delete bindings with immediate effect
5. **Tray icon**: Present, responsive, shows context menu
6. **Autostart**: App starts on Windows login when enabled
7. **Admin windows**: Can focus Unreal Editor and other elevated windows
8. **Anti-cheat safe**: Does not trigger EAC, BattlEye, or Vanguard detections
9. **Single binary**: `go build` produces one `.exe` with no external dependencies at runtime
10. **Stability**: Runs for hours without memory leaks or becoming unresponsive
