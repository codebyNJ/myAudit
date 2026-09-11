//go:build integration

package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myaudit/internal/sandbox"
)

func TestRealFeatureAddProject(t *testing.T) {
	if os.Getenv("TEMPLATE_PATH") == "" {
		t.Skip("TEMPLATE_PATH required (real template, no mocks)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	ws, err := sandbox.Scaffold(ctx, t.TempDir(), "feat1", "Lumen", "lumen")
	if err != nil {
		t.Fatal(err)
	}

	task := "Add a workspace-scoped backend feature module 'Project' with fields " +
		"name (string, required) and status (string). Follow the existing Item module EXACTLY as the pattern: " +
		"create backend/src/models/project.model.js and backend/src/routes/v1/project/{project.route.js,project.controller.js,project.validations.js}, " +
		"register the model in backend/src/models/index.js and the routes in backend/src/routes/v1/index.js, " +
		"and add Project cleanup to the deleteWorkspace cascade. Do NOT add tests. Match CLAUDE.md conventions."

	res, err := Run(ctx, ws, task, Options{
		Model: "claude-haiku-4-5-20251001",
		Allow: []string{"Read", "Write", "Edit", "Glob", "Grep", "Bash(cat:*)", "Bash(ls:*)", "Bash(mkdir:*)"},
		Deny:  []string{"Bash(rm:*)", "Bash(sudo:*)", "Bash(curl:*)", "Bash(wget:*)", "WebFetch", "WebSearch"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("agent OK=%v cost=$%.4f tokens=%d", res.OK, res.CostUSD, res.Tokens)
	if !res.OK {
		t.Fatalf("agent failed: %s", res.Err)
	}

	for _, p := range []string{
		"backend/src/models/project.model.js",
		"backend/src/routes/v1/project/project.route.js",
		"backend/src/routes/v1/project/project.controller.js",
		"backend/src/routes/v1/project/project.validations.js",
	} {
		if _, err := os.Stat(filepath.Join(ws.Dir, p)); err != nil {
			t.Fatalf("expected generated file missing: %s", p)
		}
	}

	idx, _ := os.ReadFile(filepath.Join(ws.Dir, "backend/src/routes/v1/index.js"))
	if !strings.Contains(strings.ToLower(string(idx)), "project") {
		t.Fatalf("project routes not registered in v1/index.js:\n%s", idx)
	}

	if out, code, _ := ws.Run(ctx, "node", "--check", "backend/src/models/project.model.js"); code != 0 {
		t.Fatalf("project.model.js is not valid JS: %s", out)
	}
	t.Logf("diff stat:")
	if d, _ := ws.Diff(ctx); d != "" {
		lines := strings.Split(d, "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "diff --git") {
				t.Logf("  %s", strings.TrimPrefix(l, "diff --git "))
			}
		}
	}
}
