//go:build !windows

package agent

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so the whole group
// can be signalled at once.
func setProcessGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree kills the child AND everything it spawned (a dev server the
// live agent started via Bash, for example) by signalling the process group.
func killProcessTree(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
