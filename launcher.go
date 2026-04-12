//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

const createNewProcessGroup = 0x00000200

var launchApp = LaunchAppImpl

// LaunchApp starts the given command in a detached process.
func LaunchApp(command string, args []string) error { return launchApp(command, args) }

// LaunchAppImpl is the real implementation of LaunchApp.
func LaunchAppImpl(command string, args []string) error {
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
	go func() { _ = cmd.Wait() }()

	return nil
}
