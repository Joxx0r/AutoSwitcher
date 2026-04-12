//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const taskName = "AutoSwitcher"

// IsAutostartEnabled checks if the AutoSwitcher scheduled task exists.
func IsAutostartEnabled() bool {
	cmd := exec.Command("schtasks", "/query", "/tn", taskName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// SetAutostart creates or deletes the scheduled task for autostart at logon.
// The task runs with highest privileges to avoid UAC prompts.
func SetAutostart(enabled bool) error {
	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("getting executable path: %w", err)
		}

		cmd := exec.Command("schtasks",
			"/create",
			"/tn", taskName,
			"/tr", exePath,
			"/sc", "onlogon",
			"/rl", "highest",
			"/f", // force overwrite if exists
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("creating scheduled task: %s: %w", string(output), err)
		}
		return nil
	}

	// Delete the task
	cmd := exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("deleting scheduled task: %s: %w", string(output), err)
	}
	return nil
}
