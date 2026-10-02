package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"path/filepath"
)

// ContainerCmd runs argv in image with dir mounted at /work: the AGENT_ISOLATE
// jail, for the agent CLI and for the repo's own commands alike. Killing the
// docker client does not stop its container, so cancelling removes it by name.
func ContainerCmd(ctx context.Context, dir, image string, dockerOpts []string, argv ...string) *exec.Cmd {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if image == "" {
		image = "myaudit-sandbox"
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	name := "myaudit-run-" + hex.EncodeToString(b)

	args := append([]string{"run", "--rm", "--name", name, "-v", dir + ":/work", "-w", "/work"}, dockerOpts...)
	args = append(append(args, image), argv...)
	c := exec.CommandContext(ctx, "docker", args...)
	c.Cancel = func() error {
		_ = exec.Command("docker", "rm", "-f", name).Run()
		if c.Process != nil {
			return c.Process.Kill()
		}
		return nil
	}
	return c
}
