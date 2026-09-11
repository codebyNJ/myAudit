package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/events"
	"github.com/codebyNJ/myAudit/internal/queue"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

type recordingAgent struct {
	calls     int
	lastMode  agent.Mode
	lastWSDir string
	result    agent.Result
	writeFile string
}

func (f *recordingAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, onStep func(string)) (agent.Result, error) {
	f.calls++
	f.lastMode = mode
	f.lastWSDir = ws.Dir
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
