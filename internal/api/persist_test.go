package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotesEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	// PUT then GET
	req, _ := http.NewRequest("PUT", srv.URL+"/api/runs/"+run.String()+"/notes", bytes.NewBufferString(`{"content":"hello notes"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("PUT notes failed: %v %v", err, resp.StatusCode)
	}
	var out struct {
		Content string `json:"content"`
	}
	r2, _ := http.Get(srv.URL + "/api/runs/" + run.String() + "/notes")
	json.NewDecoder(r2.Body).Decode(&out)
	if out.Content != "hello notes" {
		t.Fatalf("notes roundtrip: %q", out.Content)
	}
}

func TestSettingsEndpoint(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	req, _ := http.NewRequest("PUT", srv.URL+"/api/settings", bytes.NewBufferString(`{"model_tier":"sonnet-5"}`))
	resp, _ := http.DefaultClient.Do(req)
	var merged map[string]any
	json.NewDecoder(resp.Body).Decode(&merged)
	if merged["model_tier"] != "sonnet-5" {
		t.Fatalf("settings merge: %+v", merged)
	}
	// GET reflects it
	r2, _ := http.Get(srv.URL + "/api/settings")
	var got map[string]any
	json.NewDecoder(r2.Body).Decode(&got)
	if got["model_tier"] != "sonnet-5" {
		t.Fatalf("settings get: %+v", got)
	}
	s.PutSettings(context.Background(), map[string]any{"model_tier": "opus-4.8"})
}

func TestSteerEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/api/runs/"+run.String()+"/steer", "application/json", bytes.NewBufferString(`{"message":"add rate limiting"}`))
	if resp.StatusCode != 202 {
		t.Fatalf("steer want 202, got %d", resp.StatusCode)
	}
	var n int
	s.DB().QueryRowContext(ctx, `SELECT count(*) FROM events WHERE run_id=? AND kind='steer'`, run).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 steer event, got %d", n)
	}
}
