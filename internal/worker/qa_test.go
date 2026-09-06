package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"myaudit/internal/agent"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// TestQALedFlow drives the whole reshaped pipeline with a fake agent:
// map fans out per-module qa cards, qa files a bug ticket blocked on its module
// (QA-over-dev), promotion unblocks it, and the autonomous dev applies a fix that
// lands in review because the suite isn't runnable here. No real model, no cost.
func TestQALedFlow(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	// Build a real workspace (two code modules + git baseline) via import.
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "moduleA"), 0o755)
	os.MkdirAll(filepath.Join(src, "lib"), 0o755)
	os.WriteFile(filepath.Join(src, "moduleA", "x.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(src, "lib", "y.go"), []byte("package lib\n"), 0o644)
	root := t.TempDir()
	if _, err := sandbox.Import(ctx, root, run.String(), src); err != nil {
		t.Fatal(err)
	}

	// A ready map node (import already happened above).
	mapID, _ := s.AddNode(ctx, run, "map", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, mapID)

	// 1) map → spawns one qa card per module.
	if _, err := RunOnce(ctx, newDeps(s, &recordingAgent{result: agent.Result{OK: true, Summary: "product map"}}, root)); err != nil {
		t.Fatal(err)
	}
	qaCards := nodesOfType(t, s, run, "qa")
	if len(qaCards) < 2 {
		t.Fatalf("map should fan out ≥2 qa cards, got %d", len(qaCards))
	}
	for _, q := range qaCards {
		if q.Status != "pending" {
			t.Fatalf("qa card should start pending, got %s", q.Status)
		}
	}

	// 2) promote + run one qa card → files a bug ticket, blocked on the qa card.
	q := newDeps(s, &recordingAgent{result: agent.Result{OK: true,
		Summary: `[{"title":"nil deref in handler","file":"moduleA/x.go:1","severity":"high","detail":"reproduce: call with empty body"}]`}}, root)
	if _, err := q.Queue.PromoteReady(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := RunOnce(ctx, q); err != nil {
		t.Fatal(err)
	}
	bugs := nodesOfType(t, s, run, "bug")
	if len(bugs) != 1 {
		t.Fatalf("qa should file exactly 1 bug, got %d", len(bugs))
	}
	if bugs[0].Status != "pending" {
		t.Fatalf("bug must be blocked (pending) until its QA is done, got %s", bugs[0].Status)
	}

	// 3) drain the board: remaining qa cards run first (QA over dev), then the
	// bug is unblocked and the autonomous dev fixes it.
	bugDeps := newDeps(s, &recordingAgent{result: agent.Result{OK: true, Summary: "guarded the nil case"}, writeFile: "moduleA/x.go"}, root)
	for i := 0; i < 10; i++ {
		bugDeps.Queue.PromoteReady(ctx)
		did, err := RunOnce(ctx, bugDeps)
		if err != nil {
			t.Fatal(err)
		}
		if !did {
			break
		}
	}
	bugs = nodesOfType(t, s, run, "bug")
	// No test runner in this workspace → fix can't be verified → parks in review,
	// never a false "fixed" (the old vitest-not-found false-positive is gone).
	if bugs[0].Status != "in_review" {
		t.Fatalf("unverifiable fix should land in review, got %s", bugs[0].Status)
	}
	notes, _ := s.GetNotes(ctx, run)
	if !strings.Contains(notes, "Fix —") {
		t.Fatalf("fix should be logged to notes; notes=%q", notes)
	}
}

func TestClassifyTestsDistinguishesNotRunnable(t *testing.T) {
	// The core of the false-positive fix: "command not found" is not a defect.
	if !looksNotRunnable("sh: vitest: command not found") {
		t.Fatal("vitest-not-found must be classified not-runnable")
	}
	if looksNotRunnable("Expected 2 but received 3") {
		t.Fatal("a real assertion failure must NOT be not-runnable")
	}
}

func nodesOfType(t *testing.T, s *store.Store, run uuid.UUID, typ string) []store.NodeDetail {
	t.Helper()
	cards, err := s.NodeDetailsForRun(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.NodeDetail
	for _, c := range cards {
		if c.Type == typ {
			out = append(out, c)
		}
	}
	return out
}

// classifyTests decides "fixed vs broken vs unverifiable" — the core of the
// auto-close guarantee. Exercise all three branches with a real tiny go module.
func TestClassifyTests(t *testing.T) {
	ctx := context.Background()

	if state, _ := classifyTests(ctx, sandbox.Workspace{Dir: t.TempDir()}); state != testNotRunnable {
		t.Fatalf("empty workspace should be not-runnable, got %v", state)
	}

	pass := t.TempDir()
	writeGoModule(t, pass, "func TestX(t *testing.T){}")
	if state, out := classifyTests(ctx, sandbox.Workspace{Dir: pass}); state != testPass {
		t.Fatalf("passing suite should be testPass, got %v out=%s", state, out)
	}

	fail := t.TempDir()
	writeGoModule(t, fail, "func TestX(t *testing.T){ t.Fatal(\"boom\") }")
	if state, _ := classifyTests(ctx, sandbox.Workspace{Dir: fail}); state != testFail {
		t.Fatalf("failing assertion should be testFail, got %v", state)
	}
}

// scanModules shapes the whole board fan-out; a regression here mis-splits every
// repo. Cover flat, multi-dir (with exclusions), and container-descent layouts.
func TestScanModules(t *testing.T) {
	flat := t.TempDir()
	mkFile(t, flat, "main.go")
	if m := scanModules(flat); len(m) != 1 || m[0].Path != "." {
		t.Fatalf("flat repo → single (root) module, got %+v", m)
	}

	multi := t.TempDir()
	mkFile(t, multi, "internal/x.go")
	mkFile(t, multi, "web/app.ts")
	mkFile(t, multi, "node_modules/dep/index.js") // skip-dir
	mkFile(t, multi, "docs/readme.md")            // no code → excluded
	got := modulePaths(scanModules(multi))
	if !got["internal"] || !got["web"] {
		t.Fatalf("want internal+web modules, got %v", got)
	}
	if got["node_modules"] || got["docs"] {
		t.Fatalf("node_modules/docs must be excluded, got %v", got)
	}

	mono := t.TempDir()
	mkFile(t, mono, "src/a/a.go")
	mkFile(t, mono, "src/b/b.go")
	got = modulePaths(scanModules(mono))
	if !got["src/a"] || !got["src/b"] {
		t.Fatalf("src container should descend one level, got %v", got)
	}
}

func writeGoModule(t *testing.T, dir, testBody string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module t\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "t_test.go"), []byte("package t\nimport \"testing\"\n"+testBody+"\n"), 0o644)
}

func mkFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("x"), 0o644)
}

func modulePaths(mods []module) map[string]bool {
	m := map[string]bool{}
	for _, x := range mods {
		m[x.Path] = true
	}
	return m
}
