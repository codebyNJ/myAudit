package store

import (
	"context"
	"os"
	"testing"
)

func testURL(t *testing.T) string {
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("set TEST_DATABASE_URL (docker compose up -d)")
	}
	return u
}

func TestOpenAndPing(t *testing.T) {
	s, err := Open(context.Background(), testURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}
