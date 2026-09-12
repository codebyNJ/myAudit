package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["agentProvider"] != "claude" {
		t.Fatalf("expected default agentProvider claude, got %v", body["agentProvider"])
	}
	providers, ok := body["providers"].(map[string]any)
	if !ok {
		t.Fatal("missing providers map")
	}
	if _, ok := providers["claude"]; !ok {
		t.Fatal("missing claude provider status")
	}
	if _, ok := providers["opencode"]; !ok {
		t.Fatal("missing opencode provider status")
	}
}
