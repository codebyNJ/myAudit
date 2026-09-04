package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateRunEndpoint(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	body := bytes.NewBufferString(`{"project":"acme-saas","landing":true,"google_auth":true}`)
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
	if len(runs) != 1 || runs[0].Project != "acme-saas" {
		t.Fatalf("run not created: %+v", runs)
	}
}

func TestResolveCheckpointEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "implement", nil)
	s.Pool().Exec(ctx, `UPDATE nodes SET status='running' WHERE id=$1`, nid)
	cp, _ := s.RaiseCheckpoint(ctx, run, nid, "Google-only?", []string{"yes", "no"})

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/checkpoints/"+cp.String()+"/resolve",
		"application/json", bytes.NewBufferString(`{"answer":"yes"}`))
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

func TestOpenAPIAndSchemaEndpoints(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	var routes []map[string]string
	resp, _ := http.Get(srv.URL + "/api/openapi")
	json.NewDecoder(resp.Body).Decode(&routes)
	if len(routes) < 3 || routes[0]["path"] != "/v1/auth/login" {
		t.Fatalf("openapi: %+v", routes)
	}
	var ents []map[string]any
	resp2, _ := http.Get(srv.URL + "/api/schema")
	json.NewDecoder(resp2.Body).Decode(&ents)
	if len(ents) < 3 || ents[0]["name"] != "users" {
		t.Fatalf("schema: %+v", ents)
	}
}

func TestCreateRunRequiresProject(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/api/runs", "application/json", bytes.NewBufferString(`{}`))
	if resp.StatusCode != 400 {
		t.Fatalf("missing project should 400, got %d", resp.StatusCode)
	}
}
