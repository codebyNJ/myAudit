package worker

import (
	"context"
	"sync/atomic"
	"testing"
)

// TestCancelRunCancelsAllNodesOfARun exercises the `running` map directly at
// the package level: with bounded concurrency (MaxConcurrent > 1), two
// sibling nodes from the SAME run can be in flight at once. Both store their
// cancel func in `running`; CancelRun(runID) must invoke BOTH, not just
// whichever one happens to still occupy the map.
//
// Before the fix, `running` was keyed by run ID (map[runID]cancelFunc), so
// the second node's Store silently overwrote the first node's entry, and
// whichever node finished first ran `running.Delete(runID)` and deleted the
// OTHER node's still-in-flight cancel func too. This test fails against that
// code: only one of the two cancel funcs gets invoked.
func TestCancelRunCancelsAllNodesOfARun(t *testing.T) {
	const runID = "test-run-cancel-all-nodes"
	const nodeAID = "test-node-a"
	const nodeBID = "test-node-b"

	var aCancelled, bCancelled int32

	_, cancelA := context.WithCancel(context.Background())
	_, cancelB := context.WithCancel(context.Background())

	// Wrap so we can observe invocation independent of the real cancel funcs.
	wrappedA := func() { atomic.StoreInt32(&aCancelled, 1); cancelA() }
	wrappedB := func() { atomic.StoreInt32(&bCancelled, 1); cancelB() }

	running.Store(nodeAID, runningEntry{runID: runID, cancel: wrappedA})
	running.Store(nodeBID, runningEntry{runID: runID, cancel: wrappedB})
	t.Cleanup(func() {
		running.Delete(nodeAID)
		running.Delete(nodeBID)
	})

	CancelRun(runID)

	if atomic.LoadInt32(&aCancelled) != 1 {
		t.Fatal("expected node A's cancel func to be invoked")
	}
	if atomic.LoadInt32(&bCancelled) != 1 {
		t.Fatal("expected node B's cancel func to be invoked")
	}
}
