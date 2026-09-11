package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotesEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

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

	r2, _ := http.Get(srv.URL + "/api/settings")
	var got map[string]any
	json.NewDecoder(r2.Body).Decode(&got)
	if got["model_tier"] != "sonnet-5" {
		t.Fatalf("settings get: %+v", got)
	}
	s.PutSettings(context.Background(), map[string]any{"model_tier": "opus-4.8"})
}

func TestMutatingEndpointsReturnEmptyOrJSON(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	post := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Post(srv.URL+path, "application/json", bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, strings.TrimSpace(string(b))
	}

	for _, path := range []string{
		"/api/runs/" + run.String() + "/cancel",
		"/api/runs/" + run.String() + "/flows",
	} {
		code, body := post(path)
		if code >= 400 {
			continue
		}
		if body == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Errorf("%s returned %d with a non-JSON, non-empty body %q", path, code, body)
		}
	}
}
