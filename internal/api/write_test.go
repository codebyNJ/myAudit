package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/myAudit/internal/store"
)

func TestCreateRunImportsRepo(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	repo := filepath.Join(t.TempDir(), "myrepo")
	os.MkdirAll(repo, 0o755)

	payload, _ := json.Marshal(map[string]string{"repo_path": repo})
	body := bytes.NewBuffer(payload)
	resp, err := http.Post(srv.URL+"/api/runs", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if out.ID == "" {
		t.Fatal("expected run id")
	}
	runs, _ := s.ListRuns(context.Background(), 10)
	if len(runs) != 1 || runs[0].Project != "myrepo" {
		t.Fatalf("run not created from repo basename: %+v", runs)
	}
}

func TestCreateRunRequiresRepoPath(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/api/runs", "application/json", bytes.NewBufferString(`{}`))
	if resp.StatusCode != 400 {
		t.Fatalf("missing repo_path should 400, got %d", resp.StatusCode)
	}
}

func TestCreateRunRejectsMissingDir(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/api/runs", "application/json", bytes.NewBufferString(`{"repo_path":"/no/such/dir/xyz"}`))
	if resp.StatusCode != 400 {
		t.Fatalf("nonexistent dir should 400, got %d", resp.StatusCode)
	}
}

func TestResolveCheckpointEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "understand", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='running' WHERE id=?`, nid)
	cp, _ := s.RaiseCheckpoint(ctx, run, nid, "which flow?", []string{"login", "signup"})

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/checkpoints/"+cp.String()+"/resolve",
		"application/json", bytes.NewBufferString(`{"answer":"login"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	n, _ := s.GetNode(ctx, nid)
	if n.Status != "ready" {
		t.Fatalf("node should be requeued to ready, got %s", n.Status)
	}
}

func TestPatchNodeStatusInReviewToDone(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	bid, err := s.CreateBug(ctx, run, store.Bug{
		Title: "Leaky handler", File: "api.go:10", Severity: "medium", Priority: "P1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetNodeStatus(ctx, bid, "in_review"); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	patchJSON(t, srv.URL+"/api/runs/"+run.String()+"/nodes/"+bid.String(),
		`{"status":"done"}`, 200)

	resp, err := http.Get(srv.URL + "/api/runs/" + run.String() + "/board")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("board: want 200 got %d", resp.StatusCode)
	}
	var cards []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cards); err != nil {
		t.Fatal(err)
	}
	for _, c := range cards {
		if c.ID == bid.String() {
			if c.Status != "done" {
				t.Fatalf("want done, got %s", c.Status)
			}
			return
		}
	}
	t.Fatal("bug card not found on board")
}
