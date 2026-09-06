package store

import (
	"context"
	"testing"
)

func TestNotesRoundtrip(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")

	if n, err := s.GetNotes(ctx, run); err != nil || n != "" {
		t.Fatalf("empty notes want '', got %q err=%v", n, err)
	}
	if err := s.PutNotes(ctx, run, "first note"); err != nil {
		t.Fatal(err)
	}
	if err := s.PutNotes(ctx, run, "updated note"); err != nil {
		t.Fatal(err)
	}
	n, err := s.GetNotes(ctx, run)
	if err != nil || n != "updated note" {
		t.Fatalf("want 'updated note', got %q err=%v", n, err)
	}
}

func TestSettingsMerge(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()

	cur, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cur["model_tier"] != "haiku-4.5" {
		t.Fatalf("default model_tier: %v", cur["model_tier"])
	}
	merged, err := s.PutSettings(ctx, map[string]any{"model_tier": "sonnet-5"})
	if err != nil {
		t.Fatal(err)
	}
	if merged["model_tier"] != "sonnet-5" || merged["theme"] != "dark" {
		t.Fatalf("merge failed: %+v", merged)
	}
	// reset for other test runs
	s.PutSettings(ctx, map[string]any{"model_tier": "haiku-4.5"})
}
