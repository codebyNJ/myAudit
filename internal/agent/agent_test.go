package agent

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"myaudit/internal/sandbox"
)

func TestIsolateBuildsDockerCommand(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "runs", "abc")
	abs, _ := filepath.Abs(dir)
	cmd := Options{Model: "haiku", Isolate: true, Allow: []string{"Read"}}.
		command(context.Background(), sandbox.Workspace{Dir: dir}, "do X")
	got := strings.Join(cmd.Args, " ")
	for _, want := range []string{
		"docker run", "--rm", "-i", "-v " + abs + ":/work", "-w /work",
		"-e CLAUDE_CODE_OAUTH_TOKEN", "myaudit-sandbox", "claude",
		"--add-dir /work", "-p",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("docker cmd missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "do X") {
		t.Fatalf("task text must not appear in argv (should go via stdin): %s", got)
	}
}

func TestDirectCommandRunsInWorkspace(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "runs", "xyz")
	abs, _ := filepath.Abs(dir)
	cmd := Options{Model: "haiku"}.command(context.Background(), sandbox.Workspace{Dir: dir}, "t")
	if cmd.Dir != abs {
		t.Fatalf("direct command should run in ws dir, got %q", cmd.Dir)
	}
	if !strings.HasSuffix(cmd.Args[0], "claude") {
		t.Fatalf("direct command should invoke claude, got %v", cmd.Args[0])
	}
}

func TestCommandDeliversTaskViaStdin(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "runs", "stdin-test")
	task := `a "quoted" task with ` + "`backticks`" + ` and {"json":"braces"}` + "\nand a newline"
	cmd := Options{Model: "haiku"}.command(context.Background(), sandbox.Workspace{Dir: dir}, task)

	if cmd.Stdin == nil {
		t.Fatal("cmd.Stdin must be set to deliver the task — -p mode with no positional prompt reads from stdin")
	}
	got, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatalf("reading cmd.Stdin: %v", err)
	}
	if string(got) != task {
		t.Fatalf("stdin content mismatch:\n got:  %q\n want: %q", got, task)
	}

	for _, arg := range cmd.Args {
		if strings.Contains(arg, task) {
			t.Fatalf("task text must not appear in any argv element (defeats the stdin fix): %q", arg)
		}
	}
}

func TestCommandUsesConfiguredBin(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "runs", "bin-test")
	cmd := Options{Bin: "custom-claude"}.command(context.Background(), sandbox.Workspace{Dir: dir}, "t")
	if !strings.HasSuffix(cmd.Args[0], "custom-claude") {
		t.Fatalf("should invoke configured Bin, got %v", cmd.Args[0])
	}
}

func TestCommandDefaultsBinToClaude(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "runs", "bin-default-test")
	cmd := Options{}.command(context.Background(), sandbox.Workspace{Dir: dir}, "t")
	if !strings.HasSuffix(cmd.Args[0], "claude") {
		t.Fatalf("should default to claude, got %v", cmd.Args[0])
	}
}

func TestArgsBuilder(t *testing.T) {
	got := strings.Join(Options{
		Model: "claude-haiku-4-5-20251001",
		Allow: []string{"Read", "Write", "Bash(npm:*)"},
		Deny:  []string{"Bash(rm:*)", "WebFetch"},
	}.Args("/ws"), " ")

	for _, want := range []string{
		"-p",
		"--output-format json",
		"--add-dir /ws",
		"--permission-mode acceptEdits",
		"--setting-sources project",
		"--model claude-haiku-4-5-20251001",
		"--allowedTools Read Write Bash(npm:*)",
		"--disallowedTools Bash(rm:*) WebFetch",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("args missing %q\n got: %s", want, got)
		}
	}
}

func TestArgsPNeverCarriesAValue(t *testing.T) {
	args := Options{}.Args("/ws")
	for i, a := range args {
		if a == "-p" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				t.Fatalf("-p must be a bare flag (task goes via stdin), but next arg is %q", args[i+1])
			}
			return
		}
	}
	t.Fatal("-p flag not found in args")
}

func TestArgsRepairUsesResume(t *testing.T) {
	got := strings.Join(Options{SessionID: "abc-123", Resume: true}.Args("/ws"), " ")
	if !strings.Contains(got, "--resume abc-123") {
		t.Fatalf("repair should --resume: %s", got)
	}
	first := strings.Join(Options{SessionID: "abc-123"}.Args("/ws"), " ")
	if !strings.Contains(first, "--session-id abc-123") {
		t.Fatalf("first run should --session-id: %s", first)
	}
}

func TestParseSuccess(t *testing.T) {
	r := parseEnvelope([]byte(`{"result":"Created project module","is_error":false,"subtype":"success","total_cost_usd":0.0736,"usage":{"input_tokens":26,"output_tokens":549}}`))
	if !r.OK {
		t.Fatalf("expected OK, err=%q", r.Err)
	}
	if r.CostUSD != 0.0736 || r.Tokens != 575 || r.Summary != "Created project module" {
		t.Fatalf("bad parse: %+v", r)
	}
}

func TestPoliciesRenderAndAreSafe(t *testing.T) {

	la, ld := PolicyFor(Live)
	live := strings.Join(Options{Allow: la, Deny: ld}.Args("/ws"), " ")
	for _, want := range []string{"Read", "Write", "Bash"} {
		if !strings.Contains(live, want) {
			t.Fatalf("Live policy should allow %s: %s", want, live)
		}
	}
	for _, bad := range []string{"WebFetch", "WebSearch"} {
		if !strings.Contains(live, bad) {
			t.Fatalf("Live policy should deny %s: %s", bad, live)
		}
	}

	ra, _ := PolicyFor(ReadOnly)
	for _, a := range ra {
		if a == "Write" || a == "Edit" || a == "MultiEdit" || strings.HasPrefix(a, "Bash") {
			t.Fatalf("ReadOnlyAllow must be read-only: %v", ra)
		}
	}
}

func TestParseError(t *testing.T) {
	r := parseEnvelope([]byte(`{"result":"","is_error":true,"subtype":"error_max_turns","total_cost_usd":0.01}`))
	if r.OK {
		t.Fatal("is_error must yield !OK")
	}
	if r.Err != "error_max_turns" {
		t.Fatalf("err should carry subtype, got %q", r.Err)
	}
}
