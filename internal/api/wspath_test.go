package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func workspace(t *testing.T, run uuid.UUID) string {
	t.Helper()
	root := filepath.Join("runs", run.String())
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	return root
}

func do(t *testing.T, method, url, body string) int {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("content-type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// "." cleans to the run root, so DELETE ?path=. used to RemoveAll the whole
// workspace — every fix in the audit, gone.
func TestDeleteRejectsWorkspaceRoot(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(context.Background(), "proj")
	root := workspace(t, run)
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	for _, p := range []string{".", "%2E", "src/..", ""} {
		if code := do(t, "DELETE", srv.URL+"/api/runs/"+run.String()+"/file?path="+p, ""); code != 400 {
			t.Fatalf("DELETE path=%q: want 400, got %d", p, code)
		}
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("workspace must still exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "keep.go")); err != nil {
		t.Fatalf("workspace contents must survive: %v", err)
	}
}

// An imported repo can carry a symlink pointing anywhere, and the agent can
// create one. Reading through it must not leave the workspace.
func TestWorkspacePathRefusesSymlinkEscape(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(context.Background(), "proj")
	root := workspace(t, run)

	outside := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(outside, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leak")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(root, "leakdir")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	base := srv.URL + "/api/runs/" + run.String()

	for _, path := range []string{"leak", "leakdir/id_rsa"} {
		if code := do(t, "GET", base+"/file?path="+path, ""); code != 400 {
			t.Fatalf("GET /file?path=%s: want 400, got %d", path, code)
		}
		if code := do(t, "GET", base+"/raw?path="+path, ""); code != 400 {
			t.Fatalf("GET /raw?path=%s: want 400, got %d", path, code)
		}
	}
	if code := do(t, "PUT", base+"/file", `{"path":"leak","content":"overwritten"}`); code != 400 {
		t.Fatalf("PUT through a symlink: want 400, got %d", code)
	}
	b, err := os.ReadFile(outside)
	if err != nil || string(b) != "PRIVATE KEY" {
		t.Fatalf("file outside the workspace was modified: %q (%v)", b, err)
	}
}

func TestWorkspacePathAllowsOrdinaryPaths(t *testing.T) {
	run := uuid.New()
	for _, p := range []string{"main.go", "src/app.ts", "./src/app.ts", "a/b/c.txt"} {
		if _, _, err := resolveWorkspacePath(run, p); err != nil {
			t.Fatalf("%q should be allowed, got %v", p, err)
		}
	}
	for _, p := range []string{"", ".", "..", "../x", "/etc/passwd", "src/../.."} {
		if _, _, err := resolveWorkspacePath(run, p); err == nil {
			t.Fatalf("%q should be refused", p)
		}
	}
}

// The node endpoints ignored {id}, so any node could be driven through any
// run's URL.
func TestNodeEndpointsRejectForeignNodes(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	mine, _ := s.CreateRun(ctx, "mine")
	theirs, _ := s.CreateRun(ctx, "theirs")
	victim, err := s.AddNodeFull(ctx, theirs, "bug", nil, map[string]any{"title": "x"}, "open")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	wrong := srv.URL + "/api/runs/" + mine.String() + "/nodes/" + victim.String()

	if code := do(t, "PATCH", wrong, `{"status":"done"}`); code != 404 {
		t.Fatalf("PATCH across runs: want 404, got %d", code)
	}
	if code := do(t, "POST", wrong+"/enqueue", ""); code != 404 {
		t.Fatalf("enqueue across runs: want 404, got %d", code)
	}
	if code := do(t, "POST", wrong+"/tags", `{"tags":["x"]}`); code != 404 {
		t.Fatalf("tags across runs: want 404, got %d", code)
	}

	n, err := s.GetNode(ctx, victim)
	if err != nil || n.Status != "open" {
		t.Fatalf("foreign node was modified: %+v (%v)", n, err)
	}
}

// Re-queuing an import would re-copy the source repo over a workspace that
// already holds fixes.
func TestEnqueueOnlyAcceptsBugTickets(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	imp, err := s.AddNodeFull(ctx, run, "import", nil, nil, "done")
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := s.AddNodeFull(ctx, run, "bug", nil, map[string]any{"title": "t"}, "open")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	base := srv.URL + "/api/runs/" + run.String() + "/nodes/"

	if code := do(t, "POST", base+imp.String()+"/enqueue", ""); code != 400 {
		t.Fatalf("enqueue import: want 400, got %d", code)
	}
	if code := do(t, "POST", base+ticket.String()+"/enqueue", ""); code != 202 {
		t.Fatalf("enqueue bug ticket: want 202, got %d", code)
	}
}

// Rejecting a review reverts a path from the import baseline. Without the
// shared path guard, "." reaches the run root and is handed straight to
// RevertFromBaseline — which on a real workspace runs `git checkout <baseline>
// -- .` and discards every agent change to a tracked file, not just the one
// being rejected. A merge resolution dropped that guard once; this test is here
// so it cannot happen quietly again.
func TestReviewRejectCannotRevertWholeWorkspace(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	root := workspace(t, run)

	fix := filepath.Join(root, "fixed.go")
	if err := os.WriteFile(fix, []byte("package main // the agent's fix"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	for _, p := range []string{".", "..", "/etc/passwd"} {
		body := `{"path":"` + p + `","status":"rejected"}`
		// The review status itself is still recorded; only the revert is skipped.
		if code := do(t, "POST", srv.URL+"/api/runs/"+run.String()+"/review", body); code != 200 {
			t.Fatalf("review path=%q: want 200, got %d", p, code)
		}
		if _, err := os.Stat(fix); err != nil {
			t.Fatalf("rejecting path=%q wiped workspace content: %v", p, err)
		}
	}
}
