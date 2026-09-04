package store

import (
	"context"
	"testing"
)

func TestCreateRunAndNode(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, err := s.CreateRun(ctx, "acme-saas")
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.AddNode(ctx, run, "implement", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetNode(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "implement" || got.Status != "pending" {
		t.Fatalf("%+v", got)
	}
}
