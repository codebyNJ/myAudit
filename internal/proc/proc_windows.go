//go:build windows

package proc

import (
	"os/exec"
	"strconv"
)

func SetGroup(c *exec.Cmd) {}

func KillTree(c *exec.Cmd) error {
	if c == nil || c.Process == nil {
		return nil
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid)).Run(); err != nil {
		return c.Process.Kill()
	}
	return nil
}
