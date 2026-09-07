//go:build !windows

// Package proc centralises spawning child processes that may themselves spawn
// servers, so cancelling one never orphans a process holding a port.
package proc

import (
	"os/exec"
	"syscall"
)

// SetGroup puts the child in its own process group so the whole group can be
// signalled at once.
func SetGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillTree kills the child and everything it spawned.
func KillTree(c *exec.Cmd) error {
	if c == nil || c.Process == nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
