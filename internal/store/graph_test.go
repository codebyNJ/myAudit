package store

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCreateGraphStoresResourceSpec(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()

	res := Resource{Name: "Project", Fields: []Field{{Name: "title", Type: "string"}, {Name: "status", Type: "string"}}}
	specs := []TaskSpec{
		{Key: "scaffold", Type: "scaffold"},
		{Key: "config", Type: "config", DepKeys: []string{"scaffold"}},
		{Key: "feature_Project", Type: "feature", DepKeys: []string{"config"}, Spec: res},
		{Key: "finalize", Type: "finalize", DepKeys: []string{"feature_Project"}},
	}
	_, ids, err := s.CreateGraph(ctx, "proj", specs)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 4 {
		t.Fatalf("want 4 nodes, got %d", len(ids))
	}
	var snap []byte
	if err := s.db.QueryRowContext(ctx, `SELECT input_snapshot FROM nodes WHERE id=?`, ids["feature_Project"]).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	var got Resource
	if err := json.Unmarshal(snap, &got); err != nil {
		t.Fatalf("spec did not round-trip as JSON: %v (%s)", err, snap)
	}
	if got.Name != "Project" || len(got.Fields) != 2 || got.Fields[0].Name != "title" {
		t.Fatalf("resource spec round-trip wrong: %+v", got)
	}
}

func TestCreateGraphWiresDeps(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()

	specs := []TaskSpec{
		{Key: "schema", Type: "implement"},
		{Key: "auth", Type: "implement", DepKeys: []string{"schema"}},
		{Key: "landing", Type: "implement", DepKeys: []string{"auth"}},
	}
	run, ids, err := s.CreateGraph(ctx, "acme-saas", specs)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("want 3 nodes, got %d", len(ids))
	}
	auth, err := s.GetNode(ctx, ids["auth"])
	if err != nil {
		t.Fatal(err)
	}
	if auth.RunID != run {
		t.Fatal("node not attached to run")
	}
	if len(auth.Deps) != 1 || auth.Deps[0] != ids["schema"] {
		t.Fatalf("auth should depend on schema, deps=%v", auth.Deps)
	}
}

func TestCreateGraphRejectsUnknownDep(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	_, _, err := s.CreateGraph(ctx, "x", []TaskSpec{
		{Key: "a", Type: "implement", DepKeys: []string{"ghost"}},
	})
	if err == nil {
		t.Fatal("unknown dep key must error")
	}
}
