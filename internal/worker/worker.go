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

// The live audit graph is import → map → qa → bug (see qa.go). The earlier
// understand/testgen/verify/review nodes were removed when the pipeline inverted
// to the QA-led flow; their shared helpers (parseFindings, findingsMarkdown,
// detectTestCmd, changedFiles, firstN) live below and are used by qa.go.

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

// --- review findings parsing (shared with qa.go) ---

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

func firstN(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
