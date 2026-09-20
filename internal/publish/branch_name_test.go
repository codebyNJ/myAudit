package publish

import (
	"testing"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/sandbox"
)

// The branch the UI shows for a ticket is created in the sandbox at fix time;
// the branch `Push PR` creates in the developer's clone is built here. They are
// the same name by construction, and this is the test that keeps it that way —
// if this file starts formatting its own string again, this fails.
func TestPublishUsesTheSharedBranchName(t *testing.T) {
	run := uuid.MustParse("fcc9ddf7-379c-40e9-9f12-710e6918a2dd")
	node := uuid.MustParse("9b0fe6e7-ec6f-4a9f-a32e-b879750511f3")

	want := sandbox.BranchName(run.String(), node.String())
	if want != "myaudit/fcc9ddf7/9b0fe6e7" {
		t.Fatalf("shared helper changed shape: %q", want)
	}
	if got := branchFor(run, node); got != want {
		t.Fatalf("publish branch = %q, sandbox label = %q", got, want)
	}
}
