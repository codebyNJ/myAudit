// Package worker runs one node of the build graph. Nodes dispatch by type:
// scaffold/config/finalize are deterministic ($0, no model); a feature node
// drives the Claude Code agent to add one domain resource, then verifies and
// captures the git diff. The Agent and Verifier are interfaces so tests inject
// fakes and prod injects the real claude-backed runner.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// Agent is the generation seam: real runs call the claude CLI, tests fake it.
type Agent interface {
	Run(ctx context.Context, ws sandbox.Workspace, task string) (agent.Result, error)
}

// Verifier checks a workspace after a feature node (e.g. build/boot). A nil
// error means the feature is good. When Deps.Verify is nil, verification is
// skipped (used by dev/dry runs).
type Verifier interface {
	Verify(ctx context.Context, ws sandbox.Workspace) error
}

// Deps are RunOnce's collaborators.
type Deps struct {
	Store         *store.Store
	Queue         *queue.Queue
	Log           *events.Logger
	Agent         Agent
	Verify        Verifier
	WorkspaceRoot string // runs live under <root>/<run-id>
	MaxRepairs    int    // bounded repair attempts on a failed feature before a checkpoint
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
	case "scaffold":
		return d.scaffold(ctx, c, ws)
	case "feature":
		return d.feature(ctx, c, ws)
	case "config", "finalize":
		return d.deterministic(ctx, c, ws)
	default:
		d.fail(ctx, c, "unknown node type: "+c.Type)
		return true, nil
	}
}

// scaffold produces the whole base app from the template — deterministically,
// no model.
func (d Deps) scaffold(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var sp struct {
		Project string `json:"project"`
	}
	_ = json.Unmarshal(c.Spec, &sp)
	name := sp.Project
	if name == "" {
		name = "app"
	}
	if _, err := sandbox.Scaffold(ctx, d.WorkspaceRoot, c.RunID.String(), name, slugify(name)); err != nil {
		d.fail(ctx, c, "scaffold: "+err.Error())
		return true, nil
	}
	d.complete(ctx, c, nodeOutput{Kind: "scaffold", Summary: "scaffolded " + name + " from template ($0)"})
	return true, nil
}

// deterministic handles config/finalize — currently pass-through commits that
// keep the graph moving with no model spend.
func (d Deps) deterministic(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	d.complete(ctx, c, nodeOutput{Kind: c.Type, Summary: c.Type + " (deterministic)"})
	return true, nil
}

// feature drives the agent to add one resource, verifies, repairs (bounded),
// and captures the diff.
func (d Deps) feature(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	var res store.Resource
	if err := json.Unmarshal(c.Spec, &res); err != nil || res.Name == "" {
		d.fail(ctx, c, "feature node has no valid resource spec")
		return true, nil
	}
	task := buildFeatureTask(res)

	var last agent.Result
	for attempt := 0; attempt <= d.MaxRepairs; attempt++ {
		t := task
		if attempt > 0 {
			t = task + "\n\nThe previous attempt failed verification with:\n" + last.Err + "\nFix it."
		}
		r, err := d.Agent.Run(ctx, ws, t)
		if err != nil {
			d.fail(ctx, c, "agent: "+err.Error())
			return true, nil
		}
		last = r
		if !r.OK {
			last.Err = r.Err
			continue // repair
		}
		// Verify (build/boot). nil verifier = accept.
		if d.Verify != nil {
			if verr := d.Verify.Verify(ctx, ws); verr != nil {
				last.Err = verr.Error()
				d.Log.Log(ctx, event(c, "verify.fail", verr.Error()))
				continue // repair
			}
			d.Log.Log(ctx, event(c, "verify.ok", "checks passed for "+res.Name))
		}
		// Success: capture diff, commit the node.
		diff, _ := ws.Diff(ctx)
		_ = ws.Commit(ctx, "feature: "+res.Name)
		d.complete(ctx, c, nodeOutput{
			Kind: "feature", Summary: r.Summary, CostUSD: r.CostUSD, Tokens: r.Tokens,
			Changed: changedFiles(diff),
		})
		return true, nil
	}

	// Out of repair budget → hand to a human instead of burning more tokens.
	q := fmt.Sprintf("feature %q failed verification after %d repair attempts: %s", res.Name, d.MaxRepairs, last.Err)
	if _, err := d.Store.RaiseCheckpoint(ctx, c.RunID, c.ID, q, nil); err != nil {
		return true, err
	}
	d.Log.Log(ctx, event(c, "checkpoint.raise", q))
	return true, nil
}

func (d Deps) complete(ctx context.Context, c *queue.ClaimedNode, out nodeOutput) {
	b, _ := json.Marshal(out)
	_ = d.Queue.Complete(ctx, c.ID, b)
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

// buildFeatureTask renders the agent instruction for one resource, leaning on
// the template's Item exemplar + CLAUDE.md conventions.
func buildFeatureTask(res store.Resource) string {
	var fields strings.Builder
	for i, f := range res.Fields {
		if i > 0 {
			fields.WriteString(", ")
		}
		typ := f.Type
		if typ == "" {
			typ = "string"
		}
		fields.WriteString(f.Name + " (" + typ + ")")
	}
	if fields.Len() == 0 {
		fields.WriteString("name (string)")
	}
	// Natural in tone, but names the reference locations so the agent doesn't burn
	// time discovering them — mirror Item, adapt for this resource, use judgment
	// on the details. (Open-ended "go study the repo" prompts explore for minutes.)
	return fmt.Sprintf(
		"Add a new feature to this app: a workspace-scoped %q resource with fields %s.\n\n"+
			"Mirror the existing \"Item\" feature as the pattern — it lives at backend/src/models/item.model.js, "+
			"backend/src/routes/v1/item/, and frontend/src/views/Item/, and is wired via backend/src/models/index.js, "+
			"backend/src/routes/v1/index.js, the deleteWorkspace cascade, and the frontend endpoints/servicesApi/routes/nav. "+
			"Adapt it for %q, following CLAUDE.md. Use your judgment on the details; don't add tests.",
		res.Name, fields.String(), res.Name)
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

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
