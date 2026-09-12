package opencode

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

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/proc"
	"github.com/codebyNJ/myAudit/internal/sandbox"
)

type Options struct {
	Model  string
	Bin    string
	OnStep func(step string)
}

func (o Options) bin() string {
	if o.Bin != "" {
		return o.Bin
	}
	return "opencode"
}

func agentForMode(m agent.Mode) string {
	if m == agent.Live {
		return "build"
	}
	return "plan"
}

func (o Options) command(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode) *exec.Cmd {
	dir := ws.Dir
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	args := []string{"run", "--format", "json", "--dir", dir}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	args = append(args, "--agent", agentForMode(mode))
	if mode == agent.Live {
		args = append(args, "--auto")
	}
	args = append(args, task)

	c := exec.CommandContext(ctx, o.bin(), args...)
	c.Dir = dir
	proc.SetGroup(c)
	c.Cancel = func() error { return proc.KillTree(c) }
	c.WaitDelay = 10 * time.Second
	return c
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

type streamState struct {
	text   strings.Builder
	cost   float64
	tokens int
	done   bool
	ok     bool
	errMsg string
}

func parseEvent(line []byte, st *streamState, onStep func(string)) {
	var ev struct {
		Type string `json:"type"`
		Part struct {
			Type   string  `json:"type"`
			Text   string  `json:"text"`
			Tool   string  `json:"tool"`
			Reason string  `json:"reason"`
			Cost   float64 `json:"cost"`
			Tokens struct {
				Input  int `json:"input"`
				Output int `json:"output"`
			} `json:"tokens"`
			State struct {
				Status string `json:"status"`
				Title  string `json:"title"`
				Input  struct {
					Command string `json:"command"`
					Path    string `json:"path"`
					Pattern string `json:"pattern"`
				} `json:"input"`
			} `json:"state"`
		} `json:"part"`
		Error struct {
			Name string `json:"name"`
			Data struct {
				Message string `json:"message"`
			} `json:"data"`
		} `json:"error"`
	}
	if json.Unmarshal(line, &ev) != nil {
		return
	}
	switch ev.Type {
	case "text":
		if ev.Part.Text != "" {
			st.text.WriteString(ev.Part.Text)
		}
	case "tool_use":
		if onStep == nil || ev.Part.State.Status != "completed" {
			return
		}
		step := describeTool(ev.Part.Tool, ev.Part.State.Title, ev.Part.State.Input.Command, ev.Part.State.Input.Path, ev.Part.State.Input.Pattern)
		if step != "" {
			onStep(step)
		}
	case "step_finish":
		if ev.Part.Reason != "" && ev.Part.Reason != "stop" {
			return
		}
		st.done = true
		st.ok = true
		st.cost = ev.Part.Cost
		st.tokens = ev.Part.Tokens.Input + ev.Part.Tokens.Output
	case "error":
		st.done = true
		st.ok = false
		st.errMsg = ev.Error.Data.Message
		if st.errMsg == "" {
			st.errMsg = ev.Error.Name
		}
		if st.errMsg == "" {
			st.errMsg = "opencode error"
		}
	}
}

func describeTool(tool, title, command, path, pattern string) string {
	if title != "" {
		return title
	}
	switch tool {
	case "bash":
		return "$ " + strings.TrimSpace(command)
	case "read", "write", "edit":
		return tool + " " + path
	case "grep":
		return "grep " + pattern
	case "glob":
		return "glob " + pattern
	default:
		if tool != "" {
			return tool
		}
		return ""
	}
}

func runJSON(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, opt Options) (agent.Result, error) {
	cmd := opt.command(ctx, ws, task, mode)
	cmd.Env = agentEnv()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return agent.Result{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return agent.Result{}, err
	}

	st := &streamState{ok: true}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		parseEvent(sc.Bytes(), st, opt.OnStep)
	}
	waitErr := cmd.Wait()

	if !st.done && waitErr == nil && st.text.Len() > 0 {
		st.done = true
		st.ok = true
	}
	if !st.done {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && waitErr != nil {
			msg = waitErr.Error()
		}
		if msg == "" {
			msg = "no result from opencode stream"
		}
		return agent.Result{}, fmt.Errorf("opencode stream: %s", msg)
	}
	res := agent.Result{
		OK:      st.ok,
		Summary: strings.TrimSpace(st.text.String()),
		CostUSD: st.cost,
		Tokens:  st.tokens,
		Err:     st.errMsg,
	}
	if !st.ok {
		return res, nil
	}
	return res, nil
}

func Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, opt Options) (agent.Result, error) {
	return runJSON(ctx, ws, task, mode, opt)
}
