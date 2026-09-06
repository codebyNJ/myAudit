package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// recordingAgent fakes the Claude Code seam: records calls + the readOnly flag,
// optionally writes a file into the workspace, returns a configured Result.
type recordingAgent struct {
	calls     int
	lastMode  agent.Mode
	result    agent.Result
	writeFile string
}

func (f *recordingAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode) (agent.Result, error) {
	f.calls++
	f.lastMode = mode
	if f.writeFile != "" {
		p := filepath.Join(ws.Dir, f.writeFile)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return f.result, nil
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func newDeps(s *store.Store, a Agent, root string) Deps {
	return Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent: a, WorkspaceRoot: root, MaxRepairs: 2,
	}
}

// --- pure-unit tests ---

func TestChangedFiles(t *testing.T) {
	d := "diff --git a/src/x.js b/src/x.js\n@@\n+a\ndiff --git a/src/y.js b/src/y.js\n@@\n+b\n"
	f := changedFiles(d)
	if len(f) != 2 || f[0] != "src/x.js" || f[1] != "src/y.js" {
		t.Fatalf("changedFiles=%v", f)
	}
}

func TestParseFindings(t *testing.T) {
	// model output wrapped in prose + a json fence (the realistic case)
	raw := "Here are the issues:\n```json\n" +
		`[{"title":"SQL injection","file":"db.go:10","severity":"high","detail":"unsanitized"},` +
		`{"title":"no timeout","file":"http.go:5","severity":"low","detail":"add ctx"}]` +
		"\n```\n"
	fs := parseFindings(raw)
	if len(fs) != 2 || fs[0].Title != "SQL injection" || fs[1].Severity != "low" {
		t.Fatalf("parse: %+v", fs)
	}
	if parseFindings("no json here") != nil {
		t.Fatal("non-json should parse to nil")
	}
	if len(parseFindings("[]")) != 0 {
		t.Fatal("empty array → no findings")
	}
}

func TestDetectTestCmd(t *testing.T) {
	node := t.TempDir()
	os.WriteFile(filepath.Join(node, "package.json"), []byte("{}"), 0o644)
	if name, _, ok := detectTestCmd(node); !ok || name != "npm" {
		t.Fatalf("npm: %s ok=%v", name, ok)
	}
	golang := t.TempDir()
	os.WriteFile(filepath.Join(golang, "go.mod"), []byte("module x"), 0o644)
	if name, _, ok := detectTestCmd(golang); !ok || name != "go" {
		t.Fatalf("go: %s ok=%v", name, ok)
	}
	if _, _, ok := detectTestCmd(t.TempDir()); ok {
		t.Fatal("empty dir should have no runner")
	}
}

// (map/qa/bug dispatch is covered end-to-end in qa_test.go's TestQALedFlow.)
