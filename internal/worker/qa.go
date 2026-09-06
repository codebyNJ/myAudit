package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// liveTimeout bounds a single live node (QA run or dev fix): dep installs,
// servers, and suites can be slow, but a hung process must not wedge the loop.
// ponytail: fixed 20m ceiling; make it an env knob if a real target needs more.
const liveTimeout = 20 * time.Minute

// mapTimeout bounds the read-only map overview call (no Bash, just reading), so a
// hung model call on the critical path can't stall the run loop.
const mapTimeout = 5 * time.Minute

// This file holds the QA-led pipeline that mirrors a real org: map the product
// into modules, let QA (priority) find bugs + test gaps per module and file
// tickets with reproduce detail, then let the autonomous dev loop fix each
// ticket. The board is the living logger; notes are the running markdown report.

// doMap turns one repo into a board: it writes a high-level product map to notes
// and spawns one qa card per module (the dynamic fan-out). The structural module
// scan is deterministic and $0; the agent overview is best-effort (skipped/empty
// under the stub) and never blocks the fan-out.
func (d Deps) doMap(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	mods := scanModules(ws.Dir)

	// map is on the critical path of every run and gates the whole fan-out; bound
	// the read-only overview call so a hang can't wedge the (single) run loop.
	overview := ""
	mapCtx, cancel := context.WithTimeout(ctx, mapTimeout)
	if r, err := d.Agent.Run(mapCtx, ws, mapTask, agent.ReadOnly); err == nil {
		overview = strings.TrimSpace(r.Summary)
	}
	cancel()

	var b strings.Builder
	b.WriteString("# Audit map\n\n")
	if overview != "" {
		b.WriteString(overview + "\n\n")
	}
	b.WriteString("## Modules under QA\n")
	for _, m := range mods {
		b.WriteString(fmt.Sprintf("- **%s** — `%s`\n", m.Name, m.Path))
	}
	_ = d.Store.PutNotes(ctx, c.RunID, b.String())

	for _, m := range mods {
		id, err := d.Store.AddNodeFull(ctx, c.RunID, "qa", []uuid.UUID{c.ID},
			map[string]any{
				"module": m.Name, "path": m.Path,
				"title": "QA · " + m.Name,
				"tags":  []string{"qa", "module:" + m.Name},
			}, "pending")
		if err != nil {
			continue
		}
		nid := id
		d.Log.Log(ctx, event(c, "map.module", m.Name+" → qa card "+nid.String()[:6]))
	}
	d.complete(ctx, c, nodeOutput{Kind: "map", Summary: fmt.Sprintf("mapped %d module(s)", len(mods))})
	return true, nil
}

// qa is the QA role for one module: review its code for real bugs + best-practice
// misses, note the test cases that should exist, and file one bug ticket per
// finding (each blocked on this qa card, so dev can only start after QA is done).
func (d Deps) qa(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var sp struct {
		Module string `json:"module"`
		Path   string `json:"path"`
	}
	_ = json.Unmarshal(c.Spec, &sp)
	if sp.Module == "" {
		sp.Module, sp.Path = "(root)", "."
	}

	ctx, cancel := context.WithTimeout(ctx, liveTimeout)
	defer cancel()
	// Install deps once up front so the QA agent spends its budget running the
	// product, not on `npm install` (idempotent — skipped if already present).
	if did, msg := ensureInstalled(ctx, ws); did {
		d.Log.Log(ctx, event(c, "qa.install", msg))
	}

	r, ok := d.runAgent(ctx, c, ws, qaTask(sp.Module, sp.Path), agent.Live)
	if !ok {
		return true, nil
	}
	findings := parseFindings(r.Summary)
	filed := 0
	for _, f := range findings {
		sev := normSeverity(f.Severity)
		bug := store.Bug{
			Title:    f.Title,
			Name:     f.Title,
			File:     f.File,
			Severity: sev,
			Priority: map[string]string{"high": "P0", "medium": "P1", "low": "P2"}[sev],
			Detail:   f.Detail,
			Tags:     []string{"from:qa", "module:" + sp.Module, sev},
		}
		if bid, err := d.Store.CreateBug(ctx, c.RunID, bug, c.ID); err == nil {
			nid := bid
			d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "finding", Msg: "[" + sp.Module + "] " + f.Title})
			filed++
		}
	}
	cur, _ := d.Store.GetNotes(ctx, c.RunID)
	_ = d.Store.PutNotes(ctx, c.RunID, cur+
		fmt.Sprintf("\n\n## QA — %s (`%s`)\n\n", sp.Module, sp.Path)+
		findingsMarkdown(findings, r.Summary))
	d.complete(ctx, c, nodeOutput{
		Kind: "qa", Summary: fmt.Sprintf("%s: %d ticket(s) filed", sp.Module, filed),
		CostUSD: r.CostUSD, Tokens: r.Tokens,
	})
	return true, nil
}

// bug is the autonomous dev: read the ticket, apply a minimal root-cause fix,
// commit it, then verify. On a green suite the card auto-closes to done; a
// genuine test failure marks it failed; a suite that can't run (missing deps/
// runner) parks the fix in review for a human — never a false "fixed". Live
// dep-install + app boot land in the next checkpoint, which makes more suites
// actually runnable here.
func (d Deps) bug(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var b store.Bug
	_ = json.Unmarshal(c.Spec, &b)
	notes, _ := d.Store.GetNotes(ctx, c.RunID)

	ctx, cancel := context.WithTimeout(ctx, liveTimeout)
	defer cancel()

	r, ok := d.runAgent(ctx, c, ws, fixTask(b, notes), agent.Live)
	if !ok {
		return true, nil
	}
	diff, _ := ws.Diff(ctx)
	changed := changedFiles(diff)

	if strings.TrimSpace(diff) == "" {
		d.appendFix(ctx, c.RunID, b, "No code change was produced — the ticket may not be a real defect, or needs a human.", nil)
		d.finish(ctx, c, nodeOutput{Kind: "bug", Summary: "no change: " + b.Title, CostUSD: r.CostUSD, Tokens: r.Tokens}, "in_review")
		return true, nil
	}
	_ = ws.Commit(ctx, "fix: "+b.Title)

	state, out := classifyTests(ctx, ws)
	fixMsg := strings.TrimSpace(r.Summary)
	if fixMsg == "" {
		fixMsg = "Applied a fix."
	}
	out = firstN(out, 1200) // keep embedded command output readable in notes

	// One atomic write per outcome (finish) — a failed/in_review result can never
	// be lost to a follow-up update, so a broken fix never shows green.
	switch state {
	case testPass:
		d.appendFix(ctx, c.RunID, b, "✅ Fixed & verified (regression green).\n\n"+fixMsg, changed)
		d.finish(ctx, c, nodeOutput{Kind: "bug", Summary: "fixed: " + b.Title, Changed: changed, CostUSD: r.CostUSD, Tokens: r.Tokens}, "done")
	case testFail:
		d.appendFix(ctx, c.RunID, b, "❌ Fix did not pass regression:\n\n```\n"+out+"\n```", changed)
		d.finish(ctx, c, nodeOutput{Kind: "bug", Summary: "fix failed regression: " + b.Title, Changed: changed, CostUSD: r.CostUSD, Tokens: r.Tokens}, "failed")
	default: // testNotRunnable
		d.appendFix(ctx, c.RunID, b, "🟡 Fix ready — tests not runnable here, needs manual verify.\n\n"+fixMsg, changed)
		d.finish(ctx, c, nodeOutput{Kind: "bug", Summary: "fix ready (unverified): " + b.Title, Changed: changed, CostUSD: r.CostUSD, Tokens: r.Tokens}, "in_review")
	}
	return true, nil
}

// appendFix records the dev's work on a ticket into the running notes log.
func (d Deps) appendFix(ctx context.Context, run uuid.UUID, b store.Bug, msg string, changed []string) {
	cur, _ := d.Store.GetNotes(ctx, run)
	var sb strings.Builder
	sb.WriteString(cur)
	sb.WriteString("\n\n### Fix — " + b.Title + "\n\n" + msg + "\n")
	if len(changed) > 0 {
		sb.WriteString("\nFiles changed: ")
		sb.WriteString("`" + strings.Join(changed, "`, `") + "`\n")
	}
	_ = d.Store.PutNotes(ctx, run, sb.String())
}

// --- test classification (honest verify gate) ---

type testResult int

const (
	testNotRunnable testResult = iota // no runner, missing deps/binary — not a defect signal
	testPass
	testFail
)

// classifyTests runs the project's suite and distinguishes a genuine failure from
// a suite that simply can't run (the old false-positive: `vitest: command not
// found` is not a bug). ponytail: signature sniffing on output; extend the list
// if a runner reports "can't run" in a new way.
func classifyTests(ctx context.Context, ws sandbox.Workspace) (testResult, string) {
	name, args, ok := detectTestCmd(ws.Dir)
	if !ok {
		return testNotRunnable, "no test runner detected"
	}
	ensureInstalled(ctx, ws) // make the runner resolvable (idempotent)
	out, code, err := ws.Run(ctx, name, args...)
	if err != nil {
		return testNotRunnable, err.Error()
	}
	if code == 0 {
		return testPass, out
	}
	if code == 127 || looksNotRunnable(out) {
		return testNotRunnable, out
	}
	return testFail, out
}

// ensureInstalled installs project dependencies if a manifest is present and they
// look missing, so the test runner actually resolves (the fix for the old
// "vitest: command not found" false-positive). Best-effort + idempotent: returns
// whether it ran and a short message. Bounded by an inner timeout so a wedged
// install can't consume the whole node budget.
func ensureInstalled(ctx context.Context, ws sandbox.Workspace) (bool, string) {
	dir := ws.Dir
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if fileExists(filepath.Join(dir, "package.json")) && !dirExists(filepath.Join(dir, "node_modules")) {
		name, args := "npm", []string{"install", "--no-audit", "--no-fund"}
		if fileExists(filepath.Join(dir, "package-lock.json")) {
			args = []string{"ci", "--no-audit", "--no-fund"}
		}
		_, code, err := ws.Run(ctx, name, args...)
		if err != nil || code != 0 {
			return true, "npm install failed (continuing)"
		}
		return true, "npm install"
	}
	if fileExists(filepath.Join(dir, "requirements.txt")) {
		if _, _, err := ws.Run(ctx, "pip", "install", "-q", "-r", "requirements.txt"); err == nil {
			return true, "pip install"
		}
	}
	return false, ""
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

var notRunnableSigns = []string{
	"command not found", "not found", "cannot find module", "cannot find package",
	"no test specified", "missing script", "is not recognized", "enoent",
	"no tests found", "modulenotfounderror", "importerror",
}

func looksNotRunnable(out string) bool {
	l := strings.ToLower(out)
	for _, s := range notRunnableSigns {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

func normSeverity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "high" || s == "medium" || s == "low" {
		return s
	}
	return "medium"
}

// --- module scan ---

type module struct{ Name, Path string }

var codeExt = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".py": true,
	".rb": true, ".java": true, ".rs": true, ".php": true, ".vue": true, ".svelte": true,
	".c": true, ".cc": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
	".kt": true, ".swift": true, ".scala": true, ".clj": true, ".ex": true,
}

var skipModuleDir = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true, ".next": true,
	"target": true, ".venv": true, "venv": true, "__pycache__": true, "vendor": true,
	".svelte-kit": true, "coverage": true, ".gradle": true, ".vercel": true, ".turbo": true,
	".output": true, ".cache": true, "public": true, "static": true, "assets": true,
	"docs": true, ".github": true, ".idea": true, ".vscode": true,
}

// scanModules picks the codebase's top-level source directories as modules,
// descending one level into common container dirs (src/app/apps/packages) so a
// Next.js or monorepo layout splits sensibly. Falls back to the whole repo as a
// single "(root)" module. Capped so a huge repo doesn't explode the board.
func scanModules(root string) []module {
	var mods []module
	seen := map[string]bool{}
	add := func(name, rel string) {
		if seen[rel] {
			return
		}
		seen[rel] = true
		mods = append(mods, module{Name: name, Path: rel})
	}

	containers := map[string]bool{"src": true, "app": true, "apps": true, "packages": true}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || skipModuleDir[e.Name()] {
			continue
		}
		abs := filepath.Join(root, e.Name())
		if containers[e.Name()] {
			// descend one level; each child dir with code is a module
			for _, sub := range readDirs(abs) {
				if skipModuleDir[sub] {
					continue
				}
				if hasCode(filepath.Join(abs, sub)) {
					add(e.Name()+"/"+sub, e.Name()+"/"+sub)
				}
			}
			continue
		}
		if hasCode(abs) {
			add(e.Name(), e.Name())
		}
	}
	if len(mods) == 0 {
		return []module{{Name: "(root)", Path: "."}}
	}
	const cap = 10
	if len(mods) > cap {
		mods = mods[:cap]
	}
	return mods
}

func readDirs(dir string) []string {
	var out []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// hasCode reports whether dir contains any source file (stops at the first one).
func hasCode(dir string) bool {
	found := false
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if info.IsDir() {
			if skipModuleDir[info.Name()] && p != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if codeExt[strings.ToLower(filepath.Ext(p))] {
			found = true
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

// --- prompts ---

const mapTask = "Read this codebase and write a concise product map as markdown: " +
	"(1) what the product does and its scope, (2) its main modules/areas and what each is responsible for, " +
	"(3) the key user + backend flows a QA should exercise. Do NOT modify any files; your final message IS the map."

func qaTask(module, path string) string {
	return fmt.Sprintf(
		"You are the QA engineer for the module %q (path `%s`) of this codebase. You have a shell "+
			"(Bash) and the dependencies are installed. QA it like a real product: read the code, then "+
			"actually EXERCISE it — run the existing test suite, run the linter/build, and where practical "+
			"start the app or hit its backend to confirm real behavior. Prefer non-blocking commands; if you "+
			"start a server, background it, probe it, then kill it — never leave a process running or block. "+
			"Find real bugs, correctness issues, best-practice violations, AND important test cases that are "+
			"missing for this module. You may write NEW test files to prove a bug, but do not fix the code. "+
			"If this module has a visible UI and you can render it, save a screenshot as evidence to "+
			"`.myaudit/preview/%[1]s.png` (create the dir).\n\n"+
			"When done, your FINAL message must be ONLY a JSON array (no prose, no fences) of findings, each:\n"+
			`{"title":"<short one-line>","file":"<path:line>","severity":"high|medium|low","detail":"<the problem, why it matters, and exact steps to reproduce (commands/inputs) or the failing test output>"}`+
			"\nReturn [] if the module is genuinely clean. Order by severity (high first). Max 8.",
		module, path)
}

func fixTask(b store.Bug, notes string) string {
	var sb strings.Builder
	sb.WriteString("You are a senior engineer fixing this ticket in the codebase.\n\n")
	sb.WriteString("TICKET: " + b.Title + "\n")
	if b.File != "" {
		sb.WriteString("Location: " + b.File + "\n")
	}
	if b.Severity != "" {
		sb.WriteString("Severity: " + b.Severity + "\n")
	}
	if strings.TrimSpace(b.Detail) != "" {
		sb.WriteString("Details / reproduce:\n" + b.Detail + "\n")
	}
	sb.WriteString("\nYou have a shell (Bash) and installed dependencies. Fix the ROOT CAUSE with the smallest " +
		"correct change, matching the codebase's existing conventions. Do not introduce unrelated changes. If a " +
		"shared function is at fault, fix it there so all callers benefit. Then RUN the relevant test(s)/build to " +
		"confirm your fix holds and didn't break anything nearby. In your final message, briefly state WHAT you " +
		"changed and WHY, and the result of the check you ran.")
	return sb.String()
}
