package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

// importedRunWithTicket imports a one-file repo, leaves an uncommitted file
// behind the way QA does, and queues one ready fix ticket.
func importedRunWithTicket(t *testing.T, s *store.Store) (run uuid.UUID, root string, ws sandbox.Workspace) {
	t.Helper()
	ctx := context.Background()
	run, _ = s.CreateRun(ctx, "proj")
	src := t.TempDir()
	// Not mkFile: it writes "x", which is exactly what recordingAgent writes,
	// and the fix would then change nothing.
	os.MkdirAll(filepath.Join(src, "moduleA"), 0o755)
	os.WriteFile(filepath.Join(src, "moduleA", "x.go"), []byte("package a\n"), 0o644)
	root = t.TempDir()
	ws, err := sandbox.Import(ctx, root, run.String(), src)
	if err != nil {
		t.Fatal(err)
	}
	mkFile(t, ws.Dir, "tests/proof_test.js") // QA's proof test, never committed by QA
	if _, err := s.AddNodeFull(ctx, run, "bug", nil,
		map[string]any{"title": "nil deref", "file": "moduleA/x.go"}, "ready"); err != nil {
		t.Fatal(err)
	}
	return run, root, ws
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestFixCommitHoldsOnlyTheFixAgentsChange(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_, root, ws := importedRunWithTicket(t, s)

	d := newDeps(s, &recordingAgent{result: agent.Result{OK: true, Summary: "guarded nil"}, writeFile: "moduleA/x.go"}, root)
	if _, err := RunOnce(ctx, d); err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(gitOut(t, ws.Dir, "show", "--name-only", "--format=", "HEAD")); got != "moduleA/x.go" {
		t.Fatalf("fix commit must hold only the fix, got:\n%s", got)
	}
	if got := gitOut(t, ws.Dir, "show", "--name-only", "--format=%s", "HEAD~1"); !strings.Contains(got, "tests/proof_test.js") {
		t.Fatalf("leftovers belong in their own commit before the fix, got:\n%s", got)
	}
}

func TestFixWithNoChangeStaysNoChange(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, root, _ := importedRunWithTicket(t, s)

	d := newDeps(s, &recordingAgent{result: agent.Result{OK: true, Summary: "looked, nothing to do"}}, root)
	if _, err := RunOnce(ctx, d); err != nil {
		t.Fatal(err)
	}

	bugs := nodesOfType(t, s, run, "bug")
	if bugs[0].Status != "in_review" || !strings.HasPrefix(bugs[0].Summary, "no change") {
		t.Fatalf("an agent that changed nothing must land in review as no change, got %s %q", bugs[0].Status, bugs[0].Summary)
	}
}
