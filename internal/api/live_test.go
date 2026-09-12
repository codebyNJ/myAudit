package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewLogReturnsFileContent(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(t.Context(), "proj")

	root := filepath.Join("runs", run.String())
	if err := os.MkdirAll(filepath.Join(root, ".myaudit"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	if err := os.WriteFile(filepath.Join(root, ".myaudit", "preview.log"), []byte("hello from dev server\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/" + run.String() + "/preview/log")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from dev server\n" {
		t.Fatalf("got %q", body)
	}
}

func TestPreviewLogEmptyWhenNoFile(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(t.Context(), "proj")

	root := filepath.Join("runs", run.String())
	os.MkdirAll(root, 0o755)
	t.Cleanup(func() { os.RemoveAll(root) })

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/" + run.String() + "/preview/log")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "" {
		t.Fatalf("expected empty body for missing log, got %q", body)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPreviewLogTruncatesToLast64KB(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(t.Context(), "proj")

	root := filepath.Join("runs", run.String())
	os.MkdirAll(filepath.Join(root, ".myaudit"), 0o755)
	t.Cleanup(func() { os.RemoveAll(root) })

	big := make([]byte, 100*1024)
	for i := range big {
		big[i] = 'a'
	}
	copy(big[len(big)-5:], []byte("TAIL\n"))
	os.WriteFile(filepath.Join(root, ".myaudit", "preview.log"), big, 0o644)

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/" + run.String() + "/preview/log")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if len(body) > 64*1024 {
		t.Fatalf("expected body capped at 64KB, got %d bytes", len(body))
	}
	if body[len(body)-5:][0] != 'T' {
		t.Fatalf("expected the tail of the log to be preserved, got suffix %q", body[len(body)-10:])
	}
}

func TestPreviewRestartReturns202(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(t.Context(), "proj")

	root := filepath.Join("runs", run.String())
	os.MkdirAll(root, 0o755)
	t.Cleanup(func() { os.RemoveAll(root) })

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/runs/"+run.String()+"/preview/restart", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}
