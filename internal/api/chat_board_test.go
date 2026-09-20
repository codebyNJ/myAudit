package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/codebyNJ/myAudit/internal/store"
)

// Chat could read any file and recall the audit's notes, but could not see the
// tickets — so "what's still open?" was unanswerable (#23).
func TestBoardSummaryListsFindings(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	s.CreateBug(ctx, run, store.Bug{Title: "SQL injection in search", File: "api.go:12", Severity: "high", Priority: "P0"})
	s.CreateBug(ctx, run, store.Bug{Title: "naming is inconsistent", Severity: "low", Priority: "P2"})

	got := boardSummary(ctx, s, run)
	for _, want := range []string{"SQL injection in search", "api.go:12", "high", "naming is inconsistent"} {
		if !strings.Contains(got, want) {
			t.Errorf("board summary missing %q, got:\n%s", want, got)
		}
	}
	// A finding with no file must not render an empty column.
	if strings.Contains(got, "|  |") {
		t.Errorf("empty column rendered instead of a placeholder:\n%s", got)
	}
}

func TestBoardSummaryEmptyWithoutFindings(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	if got := boardSummary(ctx, s, run); got != "" {
		t.Fatalf("want no board section when there are no findings, got:\n%s", got)
	}
}

// A long audit must not push the notes out of the prompt.
func TestBoardSummaryTruncates(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	for i := 0; i < boardContextLimit+5; i++ {
		s.CreateBug(ctx, run, store.Bug{Title: fmt.Sprintf("finding %d", i), Severity: "low", Priority: "P2"})
	}

	got := boardSummary(ctx, s, run)
	if strings.Count(got, "| finding ") > boardContextLimit {
		t.Fatalf("listed more than %d findings", boardContextLimit)
	}
	if !strings.Contains(got, "and 5 more") {
		t.Fatalf("truncation not reported:\n%s", got[max(0, len(got)-200):])
	}
}
