package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"myaudit/internal/agent"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

func TestQALedFlow(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "moduleA"), 0o755)
	os.MkdirAll(filepath.Join(src, "lib"), 0o755)
	os.WriteFile(filepath.Join(src, "moduleA", "x.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(src, "lib", "y.go"), []byte("package lib\n"), 0o644)
	root := t.TempDir()
	if _, err := sandbox.Import(ctx, root, run.String(), src); err != nil {
		t.Fatal(err)
	}

	mapID, _ := s.AddNode(ctx, run, "map", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, mapID)

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

	if bugs[0].Status != "in_review" {
		t.Fatalf("unverifiable fix should land in review, got %s", bugs[0].Status)
	}
	notes, _ := s.GetNotes(ctx, run)
	if !strings.Contains(notes, "Fix —") {
		t.Fatalf("fix should be logged to notes; notes=%q", notes)
	}
}

func TestQAScopesWorkspaceToItsModule(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "moduleA"), 0o755)
	os.WriteFile(filepath.Join(src, "moduleA", "x.go"), []byte("package a\n"), 0o644)
	root := t.TempDir()
	if _, err := sandbox.Import(ctx, root, run.String(), src); err != nil {
		t.Fatal(err)
	}

	qaID, _ := s.AddNodeFull(ctx, run, "qa", nil,
		map[string]any{"module": "moduleA", "path": "moduleA"}, "ready")

	fake := &recordingAgent{result: agent.Result{OK: true, Summary: "[]"}}
	deps := newDeps(s, fake, root)
	if _, err := RunOnce(ctx, deps); err != nil {
		t.Fatal(err)
	}

	wantDir := filepath.Join(root, run.String(), "moduleA")
	wantAbs, _ := filepath.Abs(wantDir)
	gotAbs, _ := filepath.Abs(fake.lastWSDir)
	if gotAbs != wantAbs {
		t.Fatalf("qa should scope the workspace to its module:\n got:  %s\n want: %s", gotAbs, wantAbs)
	}
	_ = qaID
}

func TestClassifyTestsDistinguishesNotRunnable(t *testing.T) {

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

func TestScanModules(t *testing.T) {
	flat := t.TempDir()
	mkFile(t, flat, "main.go")
	if m, _ := scanModules(flat); len(m) != 1 || m[0].Path != "." {
		t.Fatalf("flat repo → single (root) module, got %+v", m)
	}

	multi := t.TempDir()
	mkFile(t, multi, "internal/x.go")
	mkFile(t, multi, "web/app.ts")
	mkFile(t, multi, "node_modules/dep/index.js")
	mkFile(t, multi, "docs/readme.md")
	mods, _ := scanModules(multi)
	got := modulePaths(mods)
	if !got["internal"] || !got["web"] {
		t.Fatalf("want internal+web modules, got %v", got)
	}
	if got["node_modules"] || got["docs"] {
		t.Fatalf("node_modules/docs must be excluded, got %v", got)
	}

	mono := t.TempDir()
	mkFile(t, mono, "src/a/a.go")
	mkFile(t, mono, "src/b/b.go")
	mods, _ = scanModules(mono)
	got = modulePaths(mods)
	if !got["src/a"] || !got["src/b"] {
		t.Fatalf("src container should descend one level, got %v", got)
	}
}

func TestScanModulesReportsDropped(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("svc-%02d", i)
		os.MkdirAll(filepath.Join(root, name), 0o755)
		os.WriteFile(filepath.Join(root, name, "main.go"), []byte("package main\n"), 0o644)
	}
	mods, dropped := scanModules(root)
	if len(mods) != 10 {
		t.Fatalf("expected 10 kept modules (the cap), got %d", len(mods))
	}
	if len(dropped) != 2 {
		t.Fatalf("expected 2 dropped modules (12 - cap of 10), got %d: %v", len(dropped), dropped)
	}
}

func TestCountFailLines(t *testing.T) {
	green, err := os.ReadFile("testdata/test_output_green.txt")
	if err != nil {
		t.Fatal(err)
	}
	if countFailLines(string(green)) != 0 {
		t.Fatalf("clean output should count 0 failures, got %d", countFailLines(string(green)))
	}
	red, err := os.ReadFile("testdata/test_output_red.txt")
	if err != nil {
		t.Fatal(err)
	}
	if countFailLines(string(red)) < 2 {
		t.Fatalf("red output should count multiple failures, got %d", countFailLines(string(red)))
	}
}

func TestQATaskIncludesNotesWhenPresent(t *testing.T) {
	got := qaTask("backend", "backend", "# Audit map\n\nThis product is a payment gateway.", "")
	if !strings.Contains(got, "payment gateway") {
		t.Fatalf("qaTask should include the notes content, got:\n%s", got)
	}
}

func TestQATaskOmitsNotesSectionWhenEmpty(t *testing.T) {
	got := qaTask("backend", "backend", "", "")
	if strings.Contains(got, "Prior analysis") {
		t.Fatalf("qaTask should not add an empty notes section, got:\n%s", got)
	}
}

func TestQATaskIncludesDetectedTestCommand(t *testing.T) {
	got := qaTask("backend", "backend", "", "go test ./...")
	if !strings.Contains(got, "go test ./...") {
		t.Fatalf("qaTask should surface the pre-detected test command, got:\n%s", got)
	}
}

func TestQATaskOmitsTestCommandHintWhenNoneDetected(t *testing.T) {
	got := qaTask("backend", "backend", "", "")
	if strings.Contains(got, "detected test command") {
		t.Fatalf("qaTask should not mention a test command hint when none was detected, got:\n%s", got)
	}
}

func TestQATaskFollowsStagedProcedure(t *testing.T) {
	got := qaTask("backend", "backend", "", "go test ./...")
	for _, want := range []string{
		"do not explore beyond this module's boundary",
		"Stop actively exploring",
		"write your findings now, even if incomplete",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("qaTask missing staged-procedure instruction %q, got:\n%s", want, got)
		}
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
