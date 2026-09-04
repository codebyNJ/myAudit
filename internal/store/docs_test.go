package store

import (
	"context"
	"testing"
)

func TestPutAndGetBrief(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	s.Pool().Exec(ctx, `DELETE FROM doc_briefs WHERE provider='elevenlabs'`)

	in := Brief{
		Provider: "elevenlabs", Version: "v1", SourceHash: "abc",
		Summary:   "Text-to-speech + realtime voice API.",
		Auth:      "xi-api-key header",
		Endpoints: []string{"POST /v1/text-to-speech/{voice_id}"},
		Example:   "curl ...",
	}
	if err := s.PutBrief(ctx, in); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetBrief(ctx, "elevenlabs", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("brief should be found")
	}
	if got.Auth != "xi-api-key header" || len(got.Endpoints) != 1 || got.SourceHash != "abc" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestGetBriefMiss(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	_, ok, err := s.GetBrief(ctx, "nope", "v9")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("missing brief should return ok=false")
	}
}
