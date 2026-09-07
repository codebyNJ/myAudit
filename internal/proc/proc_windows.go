//go:build windows

// Package proc centralises spawning child processes that may themselves spawn
// servers, so cancelling one never orphans a process holding a port.
package proc

import (
	"os/exec"
	"strconv"
)

// SetGroup is a no-op on Windows: there are no POSIX process groups, and
// KillTree walks the child tree instead.
func SetGroup(c *exec.Cmd) {}

// KillTree kills the child and its descendants (taskkill /T is the Windows
// analogue of signalling a process group).
func KillTree(c *exec.Cmd) error {
	if c == nil || c.Process == nil {
		return nil
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid)).Run(); err != nil {
		return c.Process.Kill()
	}
	return nil
}
