package api

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/myAudit/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func postJSON(t *testing.T, url, body string, want int) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("POST %s: want %d got %d", url, want, resp.StatusCode)
	}
}

func putJSON(t *testing.T, url, body string, want int) {
	t.Helper()
	req, _ := http.NewRequest("PUT", url, bytes.NewBufferString(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("PUT %s: want %d got %d", url, want, resp.StatusCode)
	}
}
