//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

func SetGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func KillTree(c *exec.Cmd) error {
	if c == nil || c.Process == nil {
		return nil
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
