package worker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/codebyNJ/myAudit/internal/sandbox"
)

// npm install scripts and test suites are the audited repo's code as much as
// the agent's Bash is; AGENT_ISOLATE has to cover them too.
func TestRepoRunUsesTheContainerWhenIsolated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as a stand-in for docker")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\necho \"docker $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AGENT_ISOLATE", "1")
	t.Setenv("AGENT_IMAGE", "img-x")

	dir := t.TempDir()
	out, code, err := repoRun(context.Background(), sandbox.Workspace{Dir: dir}, "npm", "test")
	if err != nil || code != 0 {
		t.Fatalf("run: %v code=%d", err, code)
	}
	abs, _ := filepath.Abs(dir)
	for _, want := range []string{"docker run --rm", "-v " + abs + ":/work", "-w /work", "img-x npm test"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
}
