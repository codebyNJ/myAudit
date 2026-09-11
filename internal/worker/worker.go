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

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/events"
	"github.com/codebyNJ/myAudit/internal/queue"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

type Agent interface {
	Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, onStep func(string)) (agent.Result, error)
}

type Deps struct {
	Store         *store.Store
	Queue         *queue.Queue
	Log           *events.Logger
	Agent         Agent
	WorkspaceRoot string
	MaxRepairs    int
	MaxConcurrent int
}

type nodeOutput struct {
	Kind    string          `json:"kind"`
	Summary string          `json:"summary,omitempty"`
	CostUSD float64         `json:"cost_usd,omitempty"`
	Tokens  int             `json:"tokens,omitempty"`
	Changed []string        `json:"changed,omitempty"`
	Flows   json.RawMessage `json:"flows,omitempty"`
}

type runningEntry struct {
	runID  string
	cancel context.CancelFunc
}

var running sync.Map

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

func (d Deps) ProcessClaimed(ctx context.Context, c *queue.ClaimedNode) error {
	nid := c.ID
	d.Log.Log(ctx, events.Event{RunID: c.RunID, NodeID: &nid, Kind: "node.start", Msg: c.Type})
	ws := sandbox.Workspace{Dir: filepath.Join(d.WorkspaceRoot, c.RunID.String())}

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

func (d Deps) runAgent(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace, task string, mode agent.Mode) (agent.Result, bool) {

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

type finding struct {
	Title      string `json:"title"`
	File       string `json:"file"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Confidence string `json:"confidence"`
	Detail     string `json:"detail"`
}

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*\]`)

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

func findingsMarkdown(fs []finding, raw string) string {
	if len(fs) == 0 {
		return raw
	}
	var b strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&b, "- **[%s]** %s", strings.ToUpper(f.Severity), f.Title)
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
