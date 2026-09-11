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
	"time"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/proc"
	"github.com/codebyNJ/myAudit/internal/sandbox"
)

type Result struct {
	OK      bool
	Summary string
	CostUSD float64
	Tokens  int
	Err     string
}

var ReadOnlyAllow = []string{
	"Read", "Glob", "Grep",
}

var ReadOnlyDeny = []string{
	"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash", "WebFetch", "WebSearch",
}

var LiveAllow = []string{
	"Read", "Glob", "Grep", "Write", "Edit", "MultiEdit", "Bash",
}

var LiveDeny = []string{
	"WebFetch", "WebSearch",
}

type Mode int

const (
	ReadOnly Mode = iota
	Live
)

func PolicyFor(m Mode) (allow, deny []string) {
	if m == Live {
		return LiveAllow, LiveDeny
	}
	return ReadOnlyAllow, ReadOnlyDeny
}

type Options struct {
	Model          string
	PermissionMode string
	Allow          []string
	Deny           []string
	SessionID      string
	Resume         bool
	Isolate        bool
	Image          string
	Bin            string

	OnStep func(step string)
}

func (o Options) bin() string {
	if o.Bin != "" {
		return o.Bin
	}
	return "claude"
}

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

		docker := []string{"run", "--rm", "-i", "--name", name, "-v", dir + ":/work", "-w", "/work", "-e", "CLAUDE_CODE_OAUTH_TOKEN", img, o.bin()}
		docker = append(docker, o.Args("/work")...)
		c = exec.CommandContext(ctx, "docker", docker...)

		c.Cancel = func() error {
			_ = exec.Command("docker", "rm", "-f", name).Run()
			if c.Process != nil {
				return c.Process.Kill()
			}
			return nil
		}
	} else {
		c = exec.CommandContext(ctx, o.bin(), o.Args(dir)...)
		c.Dir = dir

		proc.SetGroup(c)
		c.Cancel = func() error { return proc.KillTree(c) }
	}
	c.Stdin = strings.NewReader(task)
	c.WaitDelay = 10 * time.Second
	return c
}

func (o Options) Args(wsDir string) []string {
	pm := o.PermissionMode
	if pm == "" {
		pm = "acceptEdits"
	}

	format := "json"
	args := []string{"-p"}
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
		"--setting-sources", "project",
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

func Run(ctx context.Context, ws sandbox.Workspace, task string, opt Options) (Result, error) {
	cmd := opt.command(ctx, ws, task)
	cmd.Env = agentEnv()
	if opt.OnStep != nil {
		return runStreaming(cmd, opt.OnStep)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if len(out) > 0 {
		return parseEnvelope(out), nil
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
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
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
