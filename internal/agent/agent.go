// Package agent drives Claude Code (the `claude` CLI) in full agent mode against
// a scaffolded workspace. The agent edits files itself with its native
// Read/Write/Edit/Bash tools under an allow/deny policy; we read the outcome
// from the JSON envelope and the workspace git diff — there is no file contract.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"myaudit/internal/sandbox"
)

// Result summarizes one agent run. OK means claude completed without error; the
// worker still judges real success via verify (build/boot) + git diff.
type Result struct {
	OK      bool
	Summary string  // envelope .result
	CostUSD float64 // .total_cost_usd
	Tokens  int     // input+output tokens
	Err     string  // failure reason when !OK
}

// DefaultAllow is the tool policy for the write node (testgen): native file
// tools only. Bash is deliberately excluded — it's the one tool that can escape
// the workspace via `../..`, and writing a test file needs Write/Edit, not a
// shell. verify runs the tests separately (deterministically), so the agent
// never needs Bash.
var DefaultAllow = []string{
	"Read", "Glob", "Grep", "Write", "Edit", "MultiEdit",
}

// DefaultDeny blocks Bash (workspace-escape + destructive surface) and network.
var DefaultDeny = []string{
	"Bash", "WebFetch", "WebSearch",
}

// ReadOnlyAllow is the policy for comprehension/review nodes: read the imported
// code, never mutate it, never shell out. Native Read/Glob/Grep are confined to
// the workspace + --add-dir, so there is no way up into the host repo.
var ReadOnlyAllow = []string{
	"Read", "Glob", "Grep",
}

// ReadOnlyDeny blocks all mutation, shell, and network for read-only nodes.
var ReadOnlyDeny = []string{
	"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash", "WebFetch", "WebSearch",
}

// LiveAllow is the policy for the live QA + dev-fix nodes: the agent may edit
// files AND run the product via Bash — install deps, launch servers, run the test
// suites and e2e, capture output. This deliberately relaxes the no-Bash
// confinement (the user opted into full live QA); the blast radius is the run's
// own workspace copy, and AGENT_ISOLATE=1 can additionally jail it in a container.
var LiveAllow = []string{
	"Read", "Glob", "Grep", "Write", "Edit", "MultiEdit", "Bash",
}

// LiveDeny keeps the model's own web tools off (Bash still reaches the network for
// package installs — that's expected and needed to run real projects).
var LiveDeny = []string{
	"WebFetch", "WebSearch",
}

// Mode selects a node's tool policy. It replaces a bare read-only bool so we can
// express the third capability tier (Live: Bash-enabled) the QA/dev loop needs.
type Mode int

const (
	ReadOnly Mode = iota // comprehension/review: Read/Glob/Grep only
	Write                // legacy write node (testgen): native file tools, no Bash
	Live                 // QA + dev fix: file tools + Bash to run the product
)

// PolicyFor returns the allow/deny tool lists for a mode.
func PolicyFor(m Mode) (allow, deny []string) {
	switch m {
	case Live:
		return LiveAllow, LiveDeny
	case Write:
		return DefaultAllow, DefaultDeny
	default:
		return ReadOnlyAllow, ReadOnlyDeny
	}
}

// Options configure the claude invocation.
type Options struct {
	Model          string   // e.g. "claude-haiku-4-5-20251001"
	PermissionMode string   // default "acceptEdits"
	Allow          []string // --allowedTools entries (e.g. "Bash(npm:*)")
	Deny           []string // --disallowedTools entries
	SessionID      string   // set for repair continuity
	Resume         bool     // true → --resume SessionID (continue), else --session-id
	Isolate        bool     // run claude inside a docker container (blast-radius isolation)
	Image          string   // container image when Isolate (default "myaudit-sandbox")
}

// command builds the exec.Cmd, either running claude directly (cwd = workspace)
// or inside a container with the workspace bind-mounted at /work. In the
// container, claude auths via CLAUDE_CODE_OAUTH_TOKEN (from `claude setup-token`)
// since the host keychain isn't reachable.
func (o Options) command(ctx context.Context, ws sandbox.Workspace, task string) *exec.Cmd {
	dir := ws.Dir
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if o.Isolate {
		img := o.Image
		if img == "" {
			img = "myaudit-sandbox"
		}
		docker := []string{"run", "--rm", "-v", dir + ":/work", "-w", "/work", "-e", "CLAUDE_CODE_OAUTH_TOKEN", img, "claude"}
		docker = append(docker, o.Args(task, "/work")...)
		return exec.CommandContext(ctx, "docker", docker...)
	}
	c := exec.CommandContext(ctx, "claude", o.Args(task, dir)...)
	c.Dir = dir
	return c
}

// Args builds the claude command arguments (exposed for testing).
func (o Options) Args(task, wsDir string) []string {
	pm := o.PermissionMode
	if pm == "" {
		pm = "acceptEdits"
	}
	args := []string{
		"-p", task,
		"--output-format", "json",
		"--add-dir", wsDir,
		"--permission-mode", pm,
		"--setting-sources", "project", // template CLAUDE.md, not the dev's global one
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	if o.SessionID != "" {
		if o.Resume {
			args = append(args, "--resume", o.SessionID)
		} else {
			args = append(args, "--session-id", o.SessionID)
		}
	}
	// Variadic flags go last so they don't swallow later flags.
	if len(o.Allow) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, o.Allow...)
	}
	if len(o.Deny) > 0 {
		args = append(args, "--disallowedTools")
		args = append(args, o.Deny...)
	}
	return args
}

// agentEnv is the process env for the agent, with myAudit's own server vars
// scrubbed. Critical: the Live agent runs the target app's dev server via Bash;
// if it inherited PORT (myAudit's own port) the app would bind — and fight for —
// that exact port, taking down the audit server. MYAUDIT_DB is dropped so the
// agent can't see or touch our database path.
func agentEnv() []string {
	drop := map[string]bool{"PORT": true, "MYAUDIT_DB": true}
	src := os.Environ()
	out := make([]string, 0, len(src))
	for _, kv := range src {
		k, _, _ := strings.Cut(kv, "=")
		if drop[k] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// envelope is Claude Code's --output-format json shape (shared with internal/claude).
type envelope struct {
	Result  string  `json:"result"`
	IsError bool    `json:"is_error"`
	Subtype string  `json:"subtype"`
	CostUSD float64 `json:"total_cost_usd"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func parseEnvelope(b []byte) Result {
	var e envelope
	if err := json.Unmarshal(b, &e); err != nil {
		return Result{Err: "parse envelope: " + err.Error()}
	}
	r := Result{
		Summary: e.Result,
		CostUSD: e.CostUSD,
		Tokens:  e.Usage.InputTokens + e.Usage.OutputTokens,
		OK:      !e.IsError,
	}
	if e.IsError {
		r.Err = e.Subtype
		if r.Err == "" {
			r.Err = "claude reported is_error"
		}
	}
	return r
}

// Run invokes claude in the workspace and returns the parsed Result. err is only
// for failures to launch/collect the process; agent-level failures are in Result.
func Run(ctx context.Context, ws sandbox.Workspace, task string, opt Options) (Result, error) {
	cmd := opt.command(ctx, ws, task)
	cmd.Stdin = nil // CRITICAL: -p mode blocks forever waiting on stdin EOF
	cmd.Env = agentEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if len(out) > 0 {
		return parseEnvelope(out), nil // envelope present even on non-zero exit
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Result{}, fmt.Errorf("claude exec: %s", msg)
	}
	return Result{}, fmt.Errorf("claude produced no output")
}
