package publish

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/store"
)

func TestPublishFixRequiresGit(t *testing.T) {
	_, err := PublishFix(context.Background(), Options{
		RunID:     uuid.New(),
		NodeID:    uuid.New(),
		RepoPath:  "/tmp/x",
		CommitSHA: "abc123",
	})
	if err == nil {
		t.Fatal("expected error without git remote")
	}
}

func TestPublishFixRequiresCommit(t *testing.T) {
	_, err := PublishFix(context.Background(), Options{
		RunID:    uuid.New(),
		NodeID:   uuid.New(),
		RepoPath: "/tmp/x",
		Git:      &store.GitInfo{HasGit: true, RemoteURL: "https://github.com/o/r.git"},
	})
	if err == nil {
		t.Fatal("expected error without commit sha")
	}
}
