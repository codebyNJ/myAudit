package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func budgetRun(t *testing.T, s *Store, budget, spent float64) (run string) {
	t.Helper()
	ctx := context.Background()
	id, err := s.CreateRun(ctx, "x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNodeFull(ctx, id, "import", nil,
		map[string]any{"budget_usd": budget}, "done"); err != nil {
		t.Fatal(err)
	}
	qa, err := s.AddNodeFull(ctx, id, "qa", nil, nil, "done")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE nodes SET output=? WHERE id=?`,
		fmt.Sprintf(`{"cost_usd":%f}`, spent), qa); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE runs SET status='running' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	return id.String()
}

func nodeStatuses(t *testing.T, s *Store, run, typ string) []string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT status FROM nodes WHERE run_id=? AND type=?`, run, typ)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			t.Fatal(err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// A qa node still running when the budget landed can file tickets afterwards.
// Those must be parked too, or the cap is a one-shot suggestion.
func TestPauseOverBudgetParksWorkFiledAfterThePause(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run := budgetRun(t, s, 1.0, 2.5)

	paused, err := s.PauseOverBudget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(paused) != 1 {
		t.Fatalf("expected the over-budget run to be paused, got %v", paused)
	}

	// A late ticket arrives after the run was already paused.
	runID := paused[0]
	if _, err := s.AddNodeFull(ctx, runID, "bug", nil, nil, "pending"); err != nil {
		t.Fatal(err)
	}

	again, err := s.PauseOverBudget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("an already-paused run should not be reported as newly paused: %v", again)
	}
	for _, st := range nodeStatuses(t, s, run, "bug") {
		if st != "paused" {
			t.Fatalf("late ticket on a paused run should be parked, got %q", st)
		}
	}
}

func TestPauseOverBudgetLeavesRunsUnderBudgetAlone(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	budgetRun(t, s, 10.0, 0.5)
	paused, err := s.PauseOverBudget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(paused) != 0 {
		t.Fatalf("run under budget must keep going, got %v", paused)
	}
}

// Notes are appended by several nodes of a run at once; none may be lost.
func TestAppendNotesKeepsConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run, err := s.CreateRun(ctx, "x")
	if err != nil {
		t.Fatal(err)
	}

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if err := s.AppendNotes(ctx, run, fmt.Sprintf("\nfinding-%02d", i)); err != nil {
				t.Errorf("append %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got, err := s.GetNotes(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if !strings.Contains(got, fmt.Sprintf("finding-%02d", i)) {
			t.Fatalf("lost finding-%02d; notes = %q", i, got)
		}
	}
}
