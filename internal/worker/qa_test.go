package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
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

func TestScanModulesCoversLooseFiles(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "server.js")
	mkFile(t, root, "lib/a.js")
	mkFile(t, root, "src/App.tsx")
	mkFile(t, root, "src/components/B.tsx")
	mkFile(t, root, ".agents/tool.py")
	mkFile(t, root, "src/.hidden/y.ts")

	mods, _ := scanModules(root)
	byName := map[string]module{}
	for _, m := range mods {
		byName[m.Name] = m
	}
	if m, ok := byName["(root files)"]; !ok || m.Path != "." || !slices.Equal(m.Files, []string{"server.js"}) {
		t.Fatalf("code at the repo root needs a module, got %+v", mods)
	}
	if m, ok := byName["src (top-level files)"]; !ok || m.Path != "src" || !slices.Equal(m.Files, []string{"App.tsx"}) {
		t.Fatalf("a container's own files need a module, got %+v", mods)
	}
	got := modulePaths(mods)
	if !got["lib"] || !got["src/components"] {
		t.Fatalf("directory modules must remain, got %v", got)
	}
	if got[".agents"] || got["src/.hidden"] {
		t.Fatalf("hidden directories are not product modules, got %v", got)
	}
}

// The repo's entry point usually lives at the root; the cap must not drop it.
func TestScanModulesKeepsRootFilesWhenCapped(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "main.go")
	for i := 0; i < 12; i++ {
		mkFile(t, root, fmt.Sprintf("svc-%02d/main.go", i))
	}
	mods, dropped := scanModules(root)
	if len(mods) != 10 || len(dropped) != 3 {
		t.Fatalf("want 10 kept / 3 dropped, got %d / %d", len(mods), len(dropped))
	}
	if mods[0].Name != "(root files)" {
		t.Fatalf("root files must survive the cap, got first module %+v", mods[0])
	}
}

func TestQALooseFilesModuleIsScopedToItsFiles(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, run.String(), "src"), 0o755)
	if _, err := s.AddNodeFull(ctx, run, "qa", nil, map[string]any{
		"module": "src (top-level files)", "path": "src", "files": []string{"App.tsx", "api.ts"},
	}, "ready"); err != nil {
		t.Fatal(err)
	}

	fake := &recordingAgent{result: agent.Result{OK: true, Summary: "[]"}}
	if _, err := RunOnce(ctx, newDeps(s, fake, root)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastTask, "audit only these files directly in `src`: App.tsx, api.ts") {
		t.Fatalf("loose-files QA must be told its files, got:\n%.400s", fake.lastTask)
	}
}

func TestAuditContextKeepsMapAndConventionsOnly(t *testing.T) {
	notes := "# Audit map\n\nthe map\n\n## Repo conventions\n\nuse tabs" +
		qaSectionMarker + "api (`api`)\n\n- **[HIGH]** finding A" +
		"\n\n### Fix — finding A\n\nfixed it"
	got := auditContext(notes)
	if !strings.Contains(got, "the map") || !strings.Contains(got, "use tabs") {
		t.Fatalf("map and conventions must stay, got:\n%s", got)
	}
	if strings.Contains(got, "finding A") || strings.Contains(got, "fixed it") {
		t.Fatalf("QA sections and fix logs must go, got:\n%s", got)
	}
}

// Notes are editable in the UI; without the QA heading there is nothing to cut.
func TestAuditContextFallsBackToWholeNotes(t *testing.T) {
	edited := "# My own notes\n\nno QA headings here"
	if got := auditContext(edited); got != edited {
		t.Fatalf("want the notes unchanged, got:\n%s", got)
	}
}

// QA nodes run in parallel and append their sections as they finish, so a
// later module's prompt used to carry every earlier module's findings.
func TestQAPromptOmitsEarlierModulesFindings(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, run.String(), "b"), 0o755)
	s.PutNotes(ctx, run, "# Audit map\n\nthe map"+qaSectionMarker+"a (`a`)\n\n- **[HIGH]** module a's finding")
	if _, err := s.AddNodeFull(ctx, run, "qa", nil, map[string]any{"module": "b", "path": "b"}, "ready"); err != nil {
		t.Fatal(err)
	}

	fake := &recordingAgent{result: agent.Result{OK: true, Summary: "[]"}}
	if _, err := RunOnce(ctx, newDeps(s, fake, root)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastTask, "the map") || strings.Contains(fake.lastTask, "module a's finding") {
		t.Fatalf("QA prompt must carry the map and not other modules' findings, got:\n%.600s", fake.lastTask)
	}
}
