package store

import (
	"context"
	"testing"
)

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
