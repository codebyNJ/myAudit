// Package worker runs one node of an audit graph. Nodes dispatch by type:
// import copies the target repo into an isolated workspace ($0, deterministic);
// understand and review drive Claude Code read-only to analyze the code; testgen
// drives it to write one test; verify runs the tests deterministically. The
// Agent is an interface so tests inject a fake and prod injects the real
// claude-backed runner.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// Agent is the Claude Code seam: real runs call the claude CLI, tests fake it.
// mode selects the tool policy (read-only comprehension/review, write, or live
// Bash-enabled QA/dev).
type Agent interface {
	Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode) (agent.Result, error)
}

// Deps are RunOnce's collaborators.
type Deps struct {
	Store         *store.Store
	Queue         *queue.Queue
	Log           *events.Logger
	Agent         Agent
	WorkspaceRoot string // runs live under <root>/<run-id>
	MaxRepairs    int    // bounded agent retries before a checkpoint
}

// nodeOutput is what we persist per node (shown in the UI, summed for cost).
type nodeOutput struct {
	Kind    string   `json:"kind"`
	Summary string   `json:"summary,omitempty"`
	CostUSD float64  `json:"cost_usd,omitempty"`
	Tokens  int      `json:"tokens,omitempty"`
	Changed []string `json:"changed,omitempty"`
}

// RunOnce claims one ready node and processes it. Infra failures return an
// error; a failed node marks the node failed and returns (true, nil).
func RunOnce(ctx context.Context, d Deps) (bool, error) {
	c, err := d.Queue.Claim(ctx)
	if err != nil {
		return false, err
	}
	if c == nil {
		return false, nil
	}
	nid := c.ID
	d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "node.start", Msg: c.Type})
	ws := sandbox.Workspace{Dir: filepath.Join(d.WorkspaceRoot, c.RunID.String())}

	switch c.Type {
	case "import":
		return d.doImport(ctx, c, ws)
	case "map":
		return d.doMap(ctx, c, ws)
	case "qa":
		return d.qa(ctx, c, ws)
	case "bug":
		return d.bug(ctx, c, ws)
	case "understand":
		return d.understand(ctx, c, ws)
	case "testgen":
		return d.testgen(ctx, c, ws)
	case "verify":
		return d.verify(ctx, c, ws)
	case "review":
		return d.review(ctx, c, ws)
	default:
		d.fail(ctx, c, "unknown node type: "+c.Type)
		return true, nil
	}
}

// doImport copies the target repo into the run workspace with a git baseline.
func (d Deps) doImport(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var sp struct {
		RepoPath string `json:"repo_path"`
	}
	_ = json.Unmarshal(c.Spec, &sp)
	if sp.RepoPath == "" {
		d.fail(ctx, c, "import: no repo_path in spec")
		return true, nil
	}
	if _, err := sandbox.Import(ctx, d.WorkspaceRoot, c.RunID.String(), sp.RepoPath); err != nil {
		d.fail(ctx, c, "import: "+err.Error())
		return true, nil
	}
	d.complete(ctx, c, nodeOutput{Kind: "import", Summary: "imported " + filepath.Base(sp.RepoPath)})
	return true, nil
}

// understand drives a read-only agent to summarize the codebase and its flows,
// writing the result to the run's notes.
func (d Deps) understand(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	r, ok := d.runAgent(ctx, c, ws, understandTask, agent.ReadOnly)
	if !ok {
		return true, nil
	}
	_ = d.Store.PutNotes(ctx, c.RunID, "# Understanding\n\n"+r.Summary)
	d.Log.Log(ctx, event(c, "understand.done", "wrote understanding + flows to notes"))
	d.complete(ctx, c, nodeOutput{Kind: "understand", Summary: firstN(r.Summary, 140), CostUSD: r.CostUSD, Tokens: r.Tokens})
	return true, nil
}

// testgen drives the agent to write one test for an identified flow, grounded on
// the understanding notes, then captures the new file(s).
func (d Deps) testgen(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	notes, _ := d.Store.GetNotes(ctx, c.RunID)
	r, ok := d.runAgent(ctx, c, ws, testgenTask(notes), agent.Write)
	if !ok {
		return true, nil
	}
	diff, _ := ws.Diff(ctx)
	_ = ws.Commit(ctx, "testgen: add test")
	d.complete(ctx, c, nodeOutput{
		Kind: "testgen", Summary: r.Summary, CostUSD: r.CostUSD, Tokens: r.Tokens,
		Changed: changedFiles(diff),
	})
	return true, nil
}

// verify runs the project's tests. Inverting the base's RED gate: the code
// already exists, so a passing suite confirms behavior and a failing one is a
// finding (never a node failure).
func (d Deps) verify(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	name, args, ok := detectTestCmd(ws.Dir)
	if !ok {
		d.Log.Log(ctx, event(c, "verify.skip", "no test runner detected"))
		d.complete(ctx, c, nodeOutput{Kind: "verify", Summary: "no test runner detected"})
		return true, nil
	}
	out, code, err := ws.Run(ctx, name, args...)
	if err != nil {
		d.fail(ctx, c, "verify run: "+err.Error())
		return true, nil
	}
	if code == 0 {
		d.Log.Log(ctx, event(c, "verify.pass", "tests passed — behavior confirmed"))
		d.complete(ctx, c, nodeOutput{Kind: "verify", Summary: "tests passed"})
	} else {
		// A failing test on existing code is a finding → file a bug ticket.
		bug := store.Bug{
			Title:    "Test failure",
			Name:     "test failure",
			Severity: "high",
			Priority: "P1",
			Detail:   "The generated test failed against the current code:\n\n" + firstN(out, 1500),
			Tags:     []string{"from:verify", "test-failure"},
		}
		if bid, err := d.Store.CreateBug(ctx, c.RunID, bug); err == nil {
			nid := bid
			d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "finding", Msg: "test failure: " + firstLine(out)})
		}
		d.complete(ctx, c, nodeOutput{Kind: "verify", Summary: "tests failed — bug filed"})
	}
	return true, nil
}

// review drives a read-only agent to flag bugs and best-practice misses, then
// files one bug ticket per finding (tagged with severity) and records a summary
// in the notes.
func (d Deps) review(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	r, ok := d.runAgent(ctx, c, ws, reviewTask, agent.ReadOnly)
	if !ok {
		return true, nil
	}
	findings := parseFindings(r.Summary)
	for _, f := range findings {
		sev := strings.ToLower(f.Severity)
		if sev != "high" && sev != "medium" && sev != "low" {
			sev = "medium"
		}
		bug := store.Bug{
			Title:    f.Title,
			Name:     f.Title,
			File:     f.File,
			Severity: sev,
			Priority: map[string]string{"high": "P0", "medium": "P1", "low": "P2"}[sev],
			Detail:   f.Detail,
			Tags:     []string{"from:review", sev},
		}
		if bid, err := d.Store.CreateBug(ctx, c.RunID, bug); err == nil {
			nid := bid
			d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "finding", Msg: f.Title})
		}
	}
	cur, _ := d.Store.GetNotes(ctx, c.RunID)
	_ = d.Store.PutNotes(ctx, c.RunID, cur+"\n\n# Findings (review)\n\n"+findingsMarkdown(findings, r.Summary))
	d.complete(ctx, c, nodeOutput{
		Kind: "review", Summary: fmt.Sprintf("%d finding(s) filed", len(findings)),
		CostUSD: r.CostUSD, Tokens: r.Tokens,
	})
	return true, nil
}

// runAgent runs the agent with bounded retries. On an infra error it fails the
// node; on repeated agent-level failure it raises a checkpoint. Returns the
// result and whether the caller should proceed.
func (d Deps) runAgent(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace, task string, mode agent.Mode) (agent.Result, bool) {
	var last agent.Result
	for attempt := 0; attempt <= d.MaxRepairs; attempt++ {
		t := task
		if attempt > 0 && last.Err != "" {
			t = task + "\n\nThe previous attempt failed with:\n" + last.Err + "\nTry again."
		}
		r, err := d.Agent.Run(ctx, ws, t, mode)
		if err != nil {
			d.fail(ctx, c, "agent: "+err.Error())
			return r, false
		}
		last = r
		if r.OK {
			return r, true
		}
	}
	q := fmt.Sprintf("%s failed after %d attempts: %s", c.Type, d.MaxRepairs, last.Err)
	if _, err := d.Store.RaiseCheckpoint(ctx, c.RunID, c.ID, q, nil); err == nil {
		d.Log.Log(ctx, event(c, "checkpoint.raise", q))
	}
	return last, false
}

func (d Deps) complete(ctx context.Context, c *queue.ClaimedNode, out nodeOutput) {
	d.finish(ctx, c, out, "done")
}

// finish persists a node's output + explicit terminal status in one write, then
// logs cost + node.end. Used directly by the bug handler so failed / in_review
// outcomes are atomic (no done-then-override race).
func (d Deps) finish(ctx context.Context, c *queue.ClaimedNode, out nodeOutput, status string) {
	b, _ := json.Marshal(out)
	if err := d.Queue.Finish(ctx, c.ID, b, status); err != nil {
		e := event(c, "node.error", "persist status: "+err.Error())
		e.Level = "error"
		d.Log.Log(ctx, e)
	}
	if out.CostUSD > 0 {
		d.Log.Log(ctx, event(c, "agent.cost", fmt.Sprintf("$%.4f (%d tokens)", out.CostUSD, out.Tokens)))
	}
	d.Log.Log(ctx, event(c, "node.end", out.Summary))
}

func (d Deps) fail(ctx context.Context, c *queue.ClaimedNode, reason string) {
	_ = d.Queue.Fail(ctx, c.ID)
	e := event(c, "node.fail", reason)
	e.Level = "error"
	d.Log.Log(ctx, e)
}

func event(c *queue.ClaimedNode, kind, msg string) events.Event {
	nid := c.ID
	return events.Event{RunID: c.RunID, NodeID: &nid, Kind: kind, Msg: msg}
}

// --- prompts ---

const understandTask = "Read this codebase and explain, as concise markdown: " +
	"(1) what the project does, (2) its high-level architecture and main components, " +
	"(3) the key user and data flows you can identify — name the real files each flow touches. " +
	"Do NOT modify any files; your final message IS the analysis."

const reviewTask = "Review this codebase for real bugs, correctness issues, and clear " +
	"best-practice violations. Do NOT modify any files.\n\n" +
	"Return ONLY a JSON array (no prose, no markdown fences) of findings, each:\n" +
	`{"title":"<short one-line>","file":"<path:line>","severity":"high|medium|low","detail":"<problem + why + fix>"}` +
	"\nReturn [] if there are no real issues. Order by severity (high first). Max 10."

func testgenTask(notes string) string {
	ctx := ""
	if strings.TrimSpace(notes) != "" {
		ctx = "Here is an understanding of this codebase:\n\n" + notes + "\n\n"
	}
	return ctx + "Pick the single most important flow and write ONE focused test that " +
		"exercises it, in the project's existing test style/framework. Create a NEW test " +
		"file; do NOT modify existing source files. Keep it runnable."
}

// --- review findings parsing ---

type finding struct {
	Title    string `json:"title"`
	File     string `json:"file"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*\]`)

// parseFindings extracts the JSON findings array from the model's reply, which
// may be wrapped in prose or ```json fences. Returns nil if none parse.
func parseFindings(s string) []finding {
	m := jsonArrayRe.FindString(s)
	if m == "" {
		return nil
	}
	var fs []finding
	if json.Unmarshal([]byte(m), &fs) != nil {
		return nil
	}
	out := fs[:0]
	for _, f := range fs {
		if strings.TrimSpace(f.Title) != "" {
			out = append(out, f)
		}
	}
	return out
}

// findingsMarkdown renders findings for the notes; falls back to the raw reply
// when parsing produced nothing (so no analysis is ever lost).
func findingsMarkdown(fs []finding, raw string) string {
	if len(fs) == 0 {
		return raw
	}
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(fmt.Sprintf("- **[%s]** %s", strings.ToUpper(f.Severity), f.Title))
		if f.File != "" {
			b.WriteString(" — `" + f.File + "`")
		}
		b.WriteString("\n")
		if f.Detail != "" {
			b.WriteString("  " + f.Detail + "\n")
		}
	}
	return b.String()
}

// --- test-runner detection ---

// detectTestCmd picks a test command from the workspace's project markers.
// ponytail: naive marker sniffing — the calibration knob for real repos. Extend
// the table (or read package.json scripts) when a target needs something else.
func detectTestCmd(dir string) (string, []string, bool) {
	switch {
	case fileExists(filepath.Join(dir, "package.json")):
		return "npm", []string{"test", "--silent"}, true
	case fileExists(filepath.Join(dir, "go.mod")):
		return "go", []string{"test", "./..."}, true
	case fileExists(filepath.Join(dir, "pytest.ini")), fileExists(filepath.Join(dir, "pyproject.toml")):
		return "pytest", []string{"-q"}, true
	}
	return "", nil, false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// --- small helpers ---

var diffFileRe = regexp.MustCompile(`(?m)^diff --git a/(\S+) b/`)

func changedFiles(diff string) []string {
	m := diffFileRe.FindAllStringSubmatch(diff, -1)
	out := make([]string, 0, len(m))
	for _, g := range m {
		out = append(out, g[1])
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func firstN(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
