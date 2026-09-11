// Package worker runs one node of the audit graph. Nodes dispatch by type:
// import copies the target repo into an isolated workspace ($0, deterministic);
// map reads it and fans out one qa card per module; qa drives Claude Code live
// (Bash) to exercise a module and file bug tickets; bug drives it to fix a ticket
// and verify the fix (see qa.go). The Agent is an interface so tests inject a
// fake and prod injects the real claude-backed runner.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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
	Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, onStep func(string)) (agent.Result, error)
}

// Deps are RunOnce's collaborators.
type Deps struct {
	Store         *store.Store
	Queue         *queue.Queue
	Log           *events.Logger
	Agent         Agent
	WorkspaceRoot string // runs live under <root>/<run-id>
	MaxRepairs    int    // bounded agent retries before a checkpoint
	MaxConcurrent int    // ready nodes claimed+dispatched per tick; 0 or 1 = serial (current behavior)
}

// nodeOutput is what we persist per node (shown in the UI, summed for cost).
type nodeOutput struct {
	Kind    string   `json:"kind"`
	Summary string   `json:"summary,omitempty"`
	CostUSD float64  `json:"cost_usd,omitempty"`
	Tokens  int      `json:"tokens,omitempty"`
	Changed []string `json:"changed,omitempty"`
	Flows   json.RawMessage `json:"flows,omitempty"`
}

// runningEntry pairs an in-flight node's cancel func with the run it belongs
// to, so CancelRun can find every node for a run even when several run
// concurrently (bounded-concurrency => possibly more than one node per run).
type runningEntry struct {
	runID  string
	cancel context.CancelFunc
}

// running maps a node id to its runningEntry, so a cancel request can
// interrupt every in-flight node belonging to that run.
var running sync.Map

// CancelRun interrupts every in-flight node of a run, if any. New nodes are stopped
// separately by marking the run's queued nodes cancelled in the store.
func CancelRun(runID string) {
	running.Range(func(key, v any) bool {
		e, ok := v.(runningEntry)
		if ok && e.runID == runID {
			running.Delete(key)
			e.cancel()
		}
		return true
	})
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
	return true, d.ProcessClaimed(ctx, c)
}

// ProcessClaimed dispatches an already-claimed node by type. Split out of
// RunOnce so a batch-claiming caller (TickAll's bounded-concurrency path)
// can claim N nodes up front via Queue.ClaimN and dispatch each one here,
// concurrently, without re-implementing the claim step.
func (d Deps) ProcessClaimed(ctx context.Context, c *queue.ClaimedNode) error {
	nid := c.ID
	d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "node.start", Msg: c.Type})
	ws := sandbox.Workspace{Dir: filepath.Join(d.WorkspaceRoot, c.RunID.String())}

	// Make this node's work cancelable so CancelRun can kill the in-flight agent
	// (and, via the agent's process-group Cancel, any dev server it spawned).
	ctx, cancel := context.WithCancel(ctx)
	nodeKey := c.ID.String()
	running.Store(nodeKey, runningEntry{runID: c.RunID.String(), cancel: cancel})
	defer func() { running.Delete(nodeKey); cancel() }()

	switch c.Type {
	case "import":
		_, err := d.doImport(ctx, c, ws)
		return err
	case "map":
		_, err := d.doMap(ctx, c, ws)
		return err
	case "flows":
		_, err := d.doFlows(ctx, c, ws)
		return err
	case "qa":
		_, err := d.qa(ctx, c, ws)
		return err
	case "bug":
		_, err := d.bug(ctx, c, ws)
		return err
	default:
		d.fail(ctx, c, "unknown node type: "+c.Type)
		return nil
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
	if !hasCode(ws.Dir) {
		d.fail(ctx, c, "No source files found to audit in "+filepath.Base(sp.RepoPath))
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
	// Stream the agent's tool use as live "agent.step" events (deduped + truncated)
	// so a running card shows what it's doing instead of dead air for minutes.
	var lastStep string
	onStep := func(step string) {
		step = firstN(step, 140)
		if step == "" || step == lastStep {
			return
		}
		lastStep = step
		d.Log.Log(ctx, event(c, "agent.step", step))
	}
	var last agent.Result
	for attempt := 0; attempt <= d.MaxRepairs; attempt++ {
		t := task
		if attempt > 0 && last.Err != "" {
			t = task + "\n\nThe previous attempt failed with:\n" + last.Err + "\nTry again."
		}
		r, err := d.Agent.Run(ctx, ws, t, mode, onStep)
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
	// Persist the reason as the node's summary (not just an event) so the board
	// card shows WHY it failed — e.g. "claude exec: not logged in" — without the
	// user having to drill into the activity log.
	b, _ := json.Marshal(nodeOutput{Kind: c.Type, Summary: reason})
	_ = d.Queue.Finish(ctx, c.ID, b, "failed")
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
	Title      string `json:"title"`
	File       string `json:"file"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Confidence string `json:"confidence"`
	Detail     string `json:"detail"`
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
