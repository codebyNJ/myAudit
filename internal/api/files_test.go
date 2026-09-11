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
)

func TestFileOpsAndSearch(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")

	root := filepath.Join("runs", run.String())
	os.MkdirAll(root, 0o755)
	t.Cleanup(func() { os.RemoveAll(root) })

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	base := srv.URL + "/api/runs/" + run.String()

	postJSON(t, base+"/file/new", `{"path":"src/app.go"}`, 201)

	putJSON(t, base+"/file", `{"path":"src/app.go","content":"package main\n// TODO: audit me\n"}`, 200)

	resp, _ := http.Get(base + "/search?q=TODO")
	var hits []searchHit
	json.NewDecoder(resp.Body).Decode(&hits)
	if len(hits) != 1 || hits[0].Path != "src/app.go" || hits[0].Line != 2 {
		t.Fatalf("search hits: %+v", hits)
	}

	postJSON(t, base+"/file/rename", `{"from":"src/app.go","to":"src/main.go"}`, 200)
	if _, err := os.Stat(filepath.Join(root, "src/main.go")); err != nil {
		t.Fatalf("rename target missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src/app.go")); err == nil {
		t.Fatal("old path should be gone after rename")
	}

	req, _ := http.NewRequest("DELETE", base+"/file?path=src/main.go", nil)
	dr, _ := http.DefaultClient.Do(req)
	if dr.StatusCode != 200 {
		t.Fatalf("delete status %d", dr.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(root, "src/main.go")); err == nil {
		t.Fatal("file should be deleted")
	}
}

func TestFileOpsRejectEscape(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(context.Background(), "proj")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	postJSON(t, srv.URL+"/api/runs/"+run.String()+"/file/new", `{"path":"../escape.txt"}`, 400)
}

func postJSON(t *testing.T, url, body string, want int) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("POST %s: want %d got %d", url, want, resp.StatusCode)
	}
}

func putJSON(t *testing.T, url, body string, want int) {
	t.Helper()
	req, _ := http.NewRequest("PUT", url, bytes.NewBufferString(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("PUT %s: want %d got %d", url, want, resp.StatusCode)
	}
}
