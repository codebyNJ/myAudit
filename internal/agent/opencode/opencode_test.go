package opencode

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/sandbox"
)

func replayFile(path string, onStep func(string)) agent.Result {
	st := &streamState{ok: true}
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parseEvent(sc.Bytes(), st, onStep)
	}
	if !st.done && st.text.Len() > 0 {
		st.done = true
		st.ok = true
	}
	return agent.Result{
		OK:      st.ok,
		Summary: strings.TrimSpace(st.text.String()),
		CostUSD: st.cost,
		Tokens:  st.tokens,
		Err:     st.errMsg,
	}
}

func TestParseSuccessStream(t *testing.T) {
	r := replayFile("testdata/stream_success.jsonl", nil)
	if !r.OK {
		t.Fatalf("expected OK, got %+v", r)
	}
	if !strings.Contains(r.Summary, "SQL injection") {
		t.Fatalf("summary: %q", r.Summary)
	}
	if r.CostUSD != 0.001 || r.Tokens != 679 {
		t.Fatalf("usage: %+v", r)
	}
}

func TestParseErrorStream(t *testing.T) {
	r := replayFile("testdata/stream_error.jsonl", nil)
	if r.OK {
		t.Fatal("expected error result")
	}
	if r.Err != "Rate limit exceeded" {
		t.Fatalf("err: %q", r.Err)
	}
}

func TestParseToolStream(t *testing.T) {
	var steps []string
	r := replayFile("testdata/stream_tool.jsonl", func(s string) { steps = append(steps, s) })
	if !r.OK || r.Summary != "done" {
		t.Fatalf("result: %+v", r)
	}
	if len(steps) != 1 || steps[0] != "Print hello to stdout" {
		t.Fatalf("steps: %v", steps)
	}
}

func TestCommandArgs(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "runs", "abc")
	opt := Options{Model: "anthropic/claude-haiku-4-5"}
	cmd := opt.command(context.Background(), sandbox.Workspace{Dir: dir}, "audit this", agent.Live)
	got := strings.Join(cmd.Args, " ")
	for _, want := range []string{"run", "--format", "json", "--agent", "build", "--auto", "--model", "anthropic/claude-haiku-4-5"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	cmd = opt.command(context.Background(), sandbox.Workspace{Dir: dir}, "audit", agent.ReadOnly)
	got = strings.Join(cmd.Args, " ")
	if strings.Contains(got, "--auto") {
		t.Fatal("read-only should not use --auto")
	}
	if !strings.Contains(got, "--agent plan") {
		t.Fatalf("expected plan agent: %s", got)
	}
}
