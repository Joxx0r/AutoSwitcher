//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

const createNewProcessGroup = 0x00000200

// LaunchApp starts the given command in a detached process.
func LaunchApp(command string, args []string) error {
	if command == "" {
		return fmt.Errorf("no launch command specified")
	}

	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching %s: %w", command, err)
	}

	// Detach — don't wait for the process
	go cmd.Wait()

	return nil
}
