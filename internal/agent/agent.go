// Package agent drives Claude Code (the `claude` CLI) in full agent mode against
// a scaffolded workspace. The agent edits files itself with its native
// Read/Write/Edit/Bash tools under an allow/deny policy; we read the outcome
// from the JSON envelope and the workspace git diff — there is no file contract.
package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

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

// Mode selects a node's tool policy: read-only comprehension (map overview) vs.
// the Bash-enabled live path (QA, dev fix, chat) that actually runs the product.
type Mode int

const (
	ReadOnly Mode = iota // map overview / chat questions: Read/Glob/Grep only
	Live                 // QA + dev fix: file tools + Bash to run the product
)

// PolicyFor returns the allow/deny tool lists for a mode.
func PolicyFor(m Mode) (allow, deny []string) {
	if m == Live {
		return LiveAllow, LiveDeny
	}
	return ReadOnlyAllow, ReadOnlyDeny
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
	// OnStep, if set, switches to streamed output: it's called with a short
	// human description of each tool the agent uses (e.g. "$ npm test", "Edit
	// x.ts") so the UI can show live progress instead of dead air.
	OnStep func(step string)
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
	var c *exec.Cmd
	if o.Isolate {
		img := o.Image
		if img == "" {
			img = "myaudit-sandbox"
		}
		name := "myaudit-run-" + uuid.NewString()[:8]
		docker := []string{"run", "--rm", "--name", name, "-v", dir + ":/work", "-w", "/work", "-e", "CLAUDE_CODE_OAUTH_TOKEN", img, "claude"}
		docker = append(docker, o.Args(task, "/work")...)
		c = exec.CommandContext(ctx, "docker", docker...)
		// Killing the `docker run` client alone leaves the container running, so on
		// cancel/timeout force-remove it by name.
		c.Cancel = func() error {
			_ = exec.Command("docker", "rm", "-f", name).Run()
			if c.Process != nil {
				return c.Process.Kill()
			}
			return nil
		}
	} else {
		c = exec.CommandContext(ctx, "claude", o.Args(task, dir)...)
		c.Dir = dir
		// Run claude in its own process group so that on cancel/timeout we can kill
		// the WHOLE group — including any dev server the live agent spawned via Bash
		// (otherwise it's orphaned and holds its port, breaking later modules).
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		c.Cancel = func() error {
			if c.Process != nil {
				_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
			}
			return nil
		}
	}
	c.WaitDelay = 10 * time.Second
	return c
}

// Args builds the claude command arguments (exposed for testing).
func (o Options) Args(task, wsDir string) []string {
	pm := o.PermissionMode
	if pm == "" {
		pm = "acceptEdits"
	}
	// Streamed mode (OnStep set) needs stream-json, which requires --verbose in -p.
	format := "json"
	args := []string{"-p", task}
	if o.OnStep != nil {
		format = "stream-json"
	}
	args = append(args, "--output-format", format)
	if o.OnStep != nil {
		args = append(args, "--verbose")
	}
	args = append(args,
		"--add-dir", wsDir,
		"--permission-mode", pm,
		"--setting-sources", "project", // template CLAUDE.md, not the dev's global one
	)
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
// When opt.OnStep is set it streams (stream-json), forwarding each tool use as a
// step; otherwise it uses the simple buffered json path.
func Run(ctx context.Context, ws sandbox.Workspace, task string, opt Options) (Result, error) {
	cmd := opt.command(ctx, ws, task)
	cmd.Stdin = nil // CRITICAL: -p mode blocks forever waiting on stdin EOF
	cmd.Env = agentEnv()
	if opt.OnStep != nil {
		return runStreaming(cmd, opt.OnStep)
	}
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

// runStreaming reads stream-json line-by-line: each assistant tool_use becomes an
// OnStep call; the final "result" line is parsed into the Result. Keeps the same
// error contract as Run (err only for launch/collect failures).
func runStreaming(cmd *exec.Cmd, onStep func(string)) (Result, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	var final Result
	var haveFinal bool
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // tool_result lines can be large
	for sc.Scan() {
		line := sc.Bytes()
		var ev struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			for _, step := range extractSteps(line) {
				onStep(step)
			}
		case "result":
			final = parseEnvelope(line)
			haveFinal = true
		}
	}
	waitErr := cmd.Wait()
	if !haveFinal {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && waitErr != nil {
			msg = waitErr.Error()
		}
		if msg == "" {
			msg = "no result from claude stream"
		}
		return Result{}, fmt.Errorf("claude stream: %s", msg)
	}
	return final, nil
}

// extractSteps pulls short tool-use descriptions out of one stream-json assistant
// line (e.g. "$ npm test", "Edit app/x.ts").
func extractSteps(line []byte) []string {
	var m struct {
		Message struct {
			Content []struct {
				Type  string         `json:"type"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &m) != nil {
		return nil
	}
	var out []string
	for _, c := range m.Message.Content {
		if c.Type == "tool_use" {
			out = append(out, describeTool(c.Name, c.Input))
		}
	}
	return out
}

func describeTool(name string, in map[string]any) string {
	str := func(k string) string { s, _ := in[k].(string); return s }
	first := func(s string) string {
		s = strings.TrimSpace(s)
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[:i]
		}
		return s
	}
	switch name {
	case "Bash":
		return "$ " + first(str("command"))
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit":
		return name + " " + str("file_path")
	case "Grep":
		return "grep " + str("pattern")
	case "Glob":
		return "glob " + str("pattern")
	default:
		return name
	}
}
