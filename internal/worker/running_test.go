package worker

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestCancelRunCancelsAllNodesOfARun(t *testing.T) {
	const runID = "test-run-cancel-all-nodes"
	const nodeAID = "test-node-a"
	const nodeBID = "test-node-b"

	var aCancelled, bCancelled int32

	_, cancelA := context.WithCancel(context.Background())
	_, cancelB := context.WithCancel(context.Background())

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
