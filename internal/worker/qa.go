package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/events"
	"github.com/codebyNJ/myAudit/internal/queue"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

const liveTimeout = 20 * time.Minute

const mapTimeout = 5 * time.Minute

func (d Deps) doMap(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	mods, dropped := scanModules(ws.Dir)

	overview := ""
	mapCtx, cancel := context.WithTimeout(ctx, mapTimeout)
	if r, err := d.Agent.Run(mapCtx, ws, mapTask, agent.ReadOnly, nil); err == nil {
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
		fmt.Fprintf(&b, "- **%s** — `%s`\n", m.Name, m.Path)
	}
	if len(dropped) > 0 {
		fmt.Fprintf(&b, "\n_%d module(s) skipped (10-module cap reached): %s_\n",
			len(dropped), strings.Join(dropped, ", "))
	}
	if conv := readConventions(ws.Dir); conv != "" {
		b.WriteString(conv)
	}
	_ = d.Store.PutNotes(ctx, c.RunID, b.String())

	if _, err := d.Store.AddNodeFull(ctx, c.RunID, "flows", []uuid.UUID{c.ID},
		map[string]any{"title": "Flows · data & product", "tags": []string{"flows"}}, "pending"); err != nil {
		d.Log.Log(ctx, event(c, "node.error", "flows node: "+err.Error()))
	}

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

func (d Deps) qa(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var sp struct {
		Module string `json:"module"`
		Path   string `json:"path"`
	}
	_ = json.Unmarshal(c.Spec, &sp)
	if sp.Module == "" {
		sp.Module, sp.Path = "(root)", "."
	}

	moduleWS := sandbox.Workspace{Dir: filepath.Join(ws.Dir, sp.Path)}

	ctx, cancel := context.WithTimeout(ctx, liveTimeout)
	defer cancel()

	if did, msg := ensureInstalled(ctx, moduleWS); did {
		d.Log.Log(ctx, event(c, "qa.install", msg))
	}

	notes, _ := d.Store.GetNotes(ctx, c.RunID)
	testCmdHint := ""
	if name, args, ok := detectTestCmd(moduleWS.Dir); ok {
		testCmdHint = strings.TrimSpace(name + " " + strings.Join(args, " "))
	}
	r, ok := d.runAgent(ctx, c, moduleWS, qaTask(sp.Module, sp.Path, notes, testCmdHint), agent.Live)
	if !ok {
		return true, nil
	}
	findings := parseFindings(r.Summary)
	if len(findings) == 0 && strings.TrimSpace(r.Summary) != "" {
		d.Log.Log(ctx, event(c, "qa.unparsed", "no findings array in the agent reply — raw output kept in notes"))
	}
	filed := 0
	opts := d.Store.RunOptsFor(ctx, c.RunID)
	for _, f := range findings {
		sev := normSeverity(f.Severity)
		class := normClass(f.Class)
		tags := []string{"from:qa", "module:" + sp.Module, sev, "class:" + class, scopeTag(f.File, sp.Path)}
		bug := store.Bug{
			Title:      f.Title,
			Name:       f.Title,
			File:       f.File,
			Severity:   sev,
			Priority:   priorityFor(class, sev),
			Class:      class,
			Category:   f.Category,
			Confidence: normConfidence(f.Confidence),
			Detail:     f.Detail,
			Tags:       tags,
		}
		var bid uuid.UUID
		var err error
		if opts.AuditOnly {

			bid, err = d.Store.CreateBug(ctx, c.RunID, bug)
		} else {
			bid, err = d.Store.CreateBug(ctx, c.RunID, bug, c.ID)
		}
		if err == nil {
			nid := bid
			d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "finding", Msg: "[" + sp.Module + "] " + f.Title})
			filed++
		}
	}
	_ = d.Store.AppendNotes(ctx, c.RunID,
		fmt.Sprintf("\n\n## QA — %s (`%s`)\n\n", sp.Module, sp.Path)+
			findingsMarkdown(findings, r.Summary))
	d.complete(ctx, c, nodeOutput{
		Kind: "qa", Summary: fmt.Sprintf("%s: %d ticket(s) filed", sp.Module, filed),
		CostUSD: r.CostUSD, Tokens: r.Tokens,
	})
	return true, nil
}

func (d Deps) bug(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var b store.Bug
	_ = json.Unmarshal(c.Spec, &b)
	notes, _ := d.Store.GetNotes(ctx, c.RunID)

	ctx, cancel := context.WithTimeout(ctx, liveTimeout)
	defer cancel()

	baseFails := d.regressionBaseline(ctx, ws, c.RunID)

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
	if err := ws.Commit(ctx, "fix: "+b.Title); err != nil {
		d.appendFix(ctx, c.RunID, b, "❌ Fix applied but git commit failed: "+err.Error(), changed)
		d.finish(ctx, c, nodeOutput{Kind: "bug", Summary: "commit failed: " + b.Title, Changed: changed, CostUSD: r.CostUSD, Tokens: r.Tokens}, "in_review")
		return true, nil
	}
	commitSHA, err := ws.HeadSHA(ctx)
	if err != nil {
		d.appendFix(ctx, c.RunID, b, "❌ Fix committed but could not read commit SHA: "+err.Error(), changed)
		d.finish(ctx, c, nodeOutput{Kind: "bug", Summary: "commit sha failed: " + b.Title, Changed: changed, CostUSD: r.CostUSD, Tokens: r.Tokens}, "in_review")
		return true, nil
	}

	state, out := classifyTests(ctx, ws)
	fixMsg := strings.TrimSpace(r.Summary)
	if fixMsg == "" {
		fixMsg = "Applied a fix."
	}
	out = firstN(out, 1200)
	baseOut := nodeOutput{Kind: "bug", Changed: changed, CostUSD: r.CostUSD, Tokens: r.Tokens, CommitSHA: commitSHA}

	switch state {
	case testPass:
		d.appendFix(ctx, c.RunID, b, "✅ Fixed & verified (regression green).\n\n"+fixMsg, changed)
		baseOut.Summary = "fixed: " + b.Title
		d.finish(ctx, c, baseOut, "done")
	case testFail:

		if countFailLines(out) <= baseFails {
			d.appendFix(ctx, c.RunID, b, "🟡 Fix applied — the suite has pre-existing failures unrelated to this change (no new failures introduced). Needs manual verify.\n\n"+fixMsg, changed)
			baseOut.Summary = "fix applied (suite already red): " + b.Title
			d.finish(ctx, c, baseOut, "in_review")
			break
		}
		d.appendFix(ctx, c.RunID, b, "❌ Fix introduced new test failures:\n\n```\n"+out+"\n```", changed)
		baseOut.Summary = "fix failed regression: " + b.Title
		d.finish(ctx, c, baseOut, "failed")
	default:
		d.appendFix(ctx, c.RunID, b, "🟡 Fix ready — tests not runnable here, needs manual verify.\n\n"+fixMsg, changed)
		baseOut.Summary = "fix ready (unverified): " + b.Title
		d.finish(ctx, c, baseOut, "in_review")
	}
	return true, nil
}

func (d Deps) appendFix(ctx context.Context, run uuid.UUID, b store.Bug, msg string, changed []string) {
	var sb strings.Builder
	sb.WriteString("\n\n### Fix — " + b.Title + "\n\n" + msg + "\n")
	if len(changed) > 0 {
		sb.WriteString("\nFiles changed: ")
		sb.WriteString("`" + strings.Join(changed, "`, `") + "`\n")
	}
	_ = d.Store.AppendNotes(ctx, run, sb.String())
}

var baselineFails sync.Map // runID -> int
var baselineLocks sync.Map // runID -> *sync.Mutex

func (d Deps) regressionBaseline(ctx context.Context, ws sandbox.Workspace, run uuid.UUID) int {
	key := run.String()
	if v, ok := baselineFails.Load(key); ok {
		return v.(int)
	}
	v, _ := baselineLocks.LoadOrStore(key, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	if v, ok := baselineFails.Load(key); ok {
		return v.(int)
	}
	n := 0
	if state, out := classifyTests(ctx, ws); state == testFail {
		n = countFailLines(out)
	}
	baselineFails.Store(key, n)
	return n
}

func countFailLines(out string) int {
	n := 0
	for _, ln := range strings.Split(out, "\n") {
		l := strings.ToLower(ln)
		if strings.Contains(l, "fail") || strings.Contains(ln, "✕") || strings.Contains(ln, "✗") || strings.Contains(ln, "✖") {
			n++
		}
	}
	return n
}

type testResult int

const (
	testNotRunnable testResult = iota
	testPass
	testFail
)

func classifyTests(ctx context.Context, ws sandbox.Workspace) (testResult, string) {
	name, args, ok := detectTestCmd(ws.Dir)
	if !ok {
		return testNotRunnable, "no test runner detected"
	}
	_, imsg := ensureInstalled(ctx, ws)
	out, code, err := ws.Run(ctx, name, args...)
	if err != nil {
		return testNotRunnable, err.Error()
	}
	if code == 0 {
		return testPass, out
	}
	if code == 127 || looksNotRunnable(out) {

		if strings.Contains(imsg, "failed") {
			return testNotRunnable, "dependency install failed — tests could not run:\n" + imsg + "\n" + out
		}
		return testNotRunnable, out
	}
	return testFail, out
}

func ensureInstalled(ctx context.Context, ws sandbox.Workspace) (bool, string) {

	if os.Getenv("REAL_CLAUDE") == "" {
		return false, ""
	}
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

// normClass keeps the agent's own judgement of what it found. Anything
// unrecognised is treated as a bug, so a model that ignores the field cannot
// quietly downgrade a real defect into a note.
func normClass(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "improvement":
		return "improvement"
	case "style":
		return "style"
	case "question":
		return "question"
	default:
		return "bug"
	}
}

func normConfidence(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

// priorityFor stops a naming nit from arriving as a P0. Only defects get a
// priority derived from severity; everything else is triage-later by
// construction, whatever severity the agent claimed.
func priorityFor(class, sev string) string {
	if class != "bug" {
		return "P2"
	}
	return map[string]string{"high": "P0", "medium": "P1", "low": "P2"}[sev]
}

// scopeTag records whether a finding is actually about the module that was
// audited. Scope was previously enforced three ways with very different
// strength — the filesystem cwd, a line in the prompt, and a cosmetic tag —
// and a finding's file was never checked against any of them, so an agent that
// wandered filed a ticket labelled with the wrong module and nothing noticed.
// Findings outside the module are tagged, never dropped: cross-module problems
// are real and the module boundary is ours, not the codebase's.
func scopeTag(file, modulePath string) string {
	if file == "" || modulePath == "" || modulePath == "." {
		return "scope:in"
	}
	clean := strings.TrimPrefix(filepath.Clean(file), "./")
	if clean == modulePath || strings.HasPrefix(clean, modulePath+"/") {
		return "scope:in"
	}
	return "scope:adjacent"
}

func normSeverity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "high" || s == "medium" || s == "low" {
		return s
	}
	return "medium"
}

// conventionFiles are the places a repo states how it wants to be worked in.
// Order matters: the agent-facing ones first, since they are written for
// exactly this audience.
var conventionFiles = []string{"AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md", ".cursorrules"}

const conventionBudget = 6000

// readConventions pulls a repo's own stated conventions into the run notes.
//
// Nothing used to tell the agent how this codebase prefers to be written, so a
// pattern used deliberately and consistently — an error-handling idiom, a
// naming scheme, a file layout — came back as a finding. The notes blob is
// already passed into every qa and fix prompt, so this needs no new plumbing:
// putting the conventions there means every later node inherits them.
func readConventions(root string) string {
	var b strings.Builder
	for _, name := range conventionFiles {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(raw))
		if text == "" {
			continue
		}
		if len(text) > conventionBudget {
			// These can be long. The opening section is where a repo states its
			// rules; the rest is usually setup instructions.
			text = text[:conventionBudget] + "\n\n_(truncated)_"
		}
		fmt.Fprintf(&b, "\n### %s\n\n%s\n", name, text)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n## Repo conventions\n\nTreat these as intentional. A pattern the repo " +
		"follows consistently is a convention, not a defect.\n" + b.String()
}

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
	"tests": true, "test": true, "__tests__": true,
}

func scanModules(root string) (mods []module, dropped []string) {
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
		return []module{{Name: "(root)", Path: "."}}, nil
	}
	const cap = 10
	if len(mods) > cap {
		for _, m := range mods[cap:] {
			dropped = append(dropped, m.Name)
		}
		mods = mods[:cap]
	}
	return mods, dropped
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

const mapTask = "Read this codebase and write a concise product map as markdown: " +
	"(1) what the product does and its scope, (2) its main modules/areas and what each is responsible for, " +
	"(3) the key user + backend flows a QA should exercise. Do NOT modify any files; your final message IS the map."

func qaTask(module, path, notes, testCmdHint string) string {
	var sb strings.Builder
	if strings.TrimSpace(notes) != "" {
		sb.WriteString("Prior analysis + audit log for this codebase:\n\n" + notes + "\n\n")
	}
	if testCmdHint != "" {
		sb.WriteString("Detected test command for this module: `" + testCmdHint + "` — run it rather than guessing.\n\n")
	}
	fmt.Fprintf(&sb,
		"You are the QA engineer for the module %q (path `%s`) of this codebase. You have a shell "+
			"(Bash) and the dependencies are installed. Work through these steps IN ORDER, once each — "+
			"do not explore beyond this module's boundary and do not loop back to an earlier step:\n\n"+
			"1. Identify this module's entry point(s) and manifest (package.json, go.mod, etc.).\n"+
			"2. Check whether a test suite exists and what command runs it (a detected command may be "+
			"given below). Run it if present; note the result.\n"+
			"3. Read the module's main flow (routes/handlers/exported functions) — list what it does "+
			"before judging anything.\n"+
			"4. Now look for defects, in this order: broken/incorrect logic → missing error handling → "+
			"security issues → missing tests. Report anything that is not a defect as an "+
			"improvement or a style note, not as a bug (see `class` below).\n"+
			"5. Stop actively exploring once you've covered 1–4 once. Do not keep re-reading files.\n"+
			"6. write your findings now, even if incomplete — a partial finding list beats a truncated response.\n\n"+
			"WHILE the product is up and reachable (if you start a server), write `.myaudit/live.json` as "+
			`{"url":"http://localhost:<port>","title":"<what is running>"}`+
			" so the operator can watch it live, and delete that file immediately after you kill the server. "+
			"As you exercise the UI, save successive screenshots to `.myaudit/live/<step>.png` so the run is "+
			"watchable frame by frame. "+
			"Find real bugs, correctness issues, AND important test cases that are missing for this "+
			"module. You may write NEW test files to prove a bug, but do not fix the code. "+
			"Put new tests in a dedicated folder (`tests/` at repo root, or `__tests__/` for JS/TS) — not "+
			"alongside source files. "+
			"If this module has a visible UI and you can render it, save a screenshot as evidence to "+
			"`.myaudit/preview/%[1]s.png` (create the dir).\n\n"+
			"JUDGEMENT. Two things separate a useful report from a noisy one:\n"+
			"- Respect the module boundary. If a problem is real but lives outside `%[2]s`, still "+
			"report it and give its true path — do not silently reattribute it to this module.\n"+
			"- Treat a pattern the codebase uses consistently as intentional. If the repo always "+
			"handles errors, names things, or lays out files a certain way, that is a convention, "+
			"not a defect. Only flag it if it is actually causing harm, and say which convention it "+
			"contradicts. Repo conventions, where they were found, are in the analysis above.\n\n"+
			"When done, your FINAL message must be ONLY a JSON array (no prose, no fences) of findings, each:\n"+
			`{"title":"<short one-line>","file":"<path:line>","class":"bug|improvement|style|question","severity":"high|medium|low","confidence":"high|medium|low","category":"<one word, e.g. security, correctness, performance, testing>","detail":"<the problem, why it matters, and exact steps to reproduce (commands/inputs) or the failing test output>"}`+
			"\n\n`class` is what you found: `bug` = it is wrong and will misbehave; `improvement` = it "+
			"works but could be better; `style` = cosmetic or convention; `question` = you are not sure "+
			"and a human should look. `severity` is how much it matters IF real. `confidence` is how sure "+
			"you are that it is real — a high-severity guess is `high`/`low`, not `high`/`high`.\n"+
			"Return [] if the module is genuinely clean. Order by severity (high first). Max 8.",
		module, path)
	return sb.String()
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
