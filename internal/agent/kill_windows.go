//go:build windows

package agent

import (
	"os/exec"
	"strconv"
)

// setProcessGroup is a no-op on Windows: there are no POSIX process groups, and
// the job-object equivalent isn't needed because killProcessTree walks children.
func setProcessGroup(c *exec.Cmd) {}

// killProcessTree kills the child and its descendants. taskkill /T is the
// Windows analogue of signalling a process group.
func killProcessTree(c *exec.Cmd) error {
	if c.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid))
	if err := kill.Run(); err != nil {
		return c.Process.Kill()
	}
	return nil
}
