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
	Kind      string          `json:"kind"`
	Summary   string          `json:"summary,omitempty"`
	CostUSD   float64         `json:"cost_usd,omitempty"`
	Tokens    int             `json:"tokens,omitempty"`
	Changed   []string        `json:"changed,omitempty"`
	Flows     json.RawMessage `json:"flows,omitempty"`
	CommitSHA string          `json:"commit_sha,omitempty"`
	PRURL     string          `json:"pr_url,omitempty"`
	PRStatus  string          `json:"pr_status,omitempty"`
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
			e.cancel()
			running.Delete(key)
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

	var err error
	switch c.Type {
	case "import":
		_, err = d.doImport(ctx, c, ws)
	case "map":
		_, err = d.doMap(ctx, c, ws)
	case "flows":
		_, err = d.doFlows(ctx, c, ws)
	case "qa":
		_, err = d.qa(ctx, c, ws)
	case "bug":
		_, err = d.bug(ctx, c, ws)
	default:
		d.fail(ctx, c, "unknown node type: "+c.Type)
		return nil
	}
	if ctx.Err() != nil {
		d.cancelled(ctx, c)
		return nil
	}
	return err
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
	git := sandbox.ProbeRepo(ctx, sp.RepoPath)
	if git.HasGit {
		g := map[string]any{
			"has_git": git.HasGit, "remote_url": git.RemoteURL,
			"default_branch": git.DefaultBranch, "head_sha": git.HeadSHA,
		}
		_ = d.Store.MergeNodeSnapshot(ctx, c.ID, map[string]any{"git": g})
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
			if ctx.Err() != nil {
				d.cancelled(ctx, c)
				return r, false
			}
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

func (d Deps) cancelled(ctx context.Context, c *queue.ClaimedNode) {
	b, _ := json.Marshal(nodeOutput{Kind: c.Type, Summary: "cancelled by user"})
	_ = d.Queue.Finish(ctx, c.ID, b, "cancelled")
	d.Log.Log(ctx, event(c, "node.cancelled", "stopped by user"))
}

func (d Deps) fail(ctx context.Context, c *queue.ClaimedNode, reason string) {
	if ctx.Err() != nil {
		d.cancelled(ctx, c)
		return
	}
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
	Title string `json:"title"`
	// Class separates a defect from a suggestion. The prompt has always asked
	// for "style/best-practice gaps" alongside real bugs, and everything came
	// back as a ticket with a P0/P1/P2 derived purely from severity — so a
	// naming nit competed for attention with a SQL injection.
	Class      string `json:"class"`
	File       string `json:"file"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Confidence string `json:"confidence"`
	Detail     string `json:"detail"`
}

// extractJSONArray returns the last well-formed JSON array in s. The agent is
// told its final message must be only the findings array, but prose often
// precedes it — and a greedy first-"[" to last-"]" match then spans that prose,
// parses as nothing, and drops every finding for the module without a word.
func extractJSONArray(s string) []byte {
	end := strings.LastIndexByte(s, ']')
	if end < 0 {
		return nil
	}
	for start := end; start >= 0; start-- {
		if s[start] != '[' {
			continue
		}
		if candidate := s[start : end+1]; json.Valid([]byte(candidate)) {
			return []byte(candidate)
		}
	}
	return nil
}

func parseFindings(s string) []finding {
	m := extractJSONArray(s)
	if m == nil {
		return nil
	}
	var fs []finding
	if json.Unmarshal(m, &fs) != nil {
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
