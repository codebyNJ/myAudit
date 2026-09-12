package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestServesSPAShell(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, StaticHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("index should 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)

	if !strings.Contains(string(body), `id="root"`) || !strings.Contains(string(body), "<script") {
		t.Fatalf("expected SPA shell (root div + script), got:\n%s", string(body)[:min(200, len(body))])
	}
}

func TestServesBuiltAssetsFromDisk(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Skip("no go.mod found")
	}
	matches, err := filepath.Glob(filepath.Join(root, "web/dist/assets/index-*.js"))
	if err != nil || len(matches) == 0 {
		matches, err = filepath.Glob(filepath.Join(root, "internal/api/web/dist/assets/index-*.js"))
	}
	if err != nil || len(matches) == 0 {
		t.Skip("no built UI on disk")
	}
	assetURL := "/assets/" + filepath.Base(matches[0])
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, StaticHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + assetURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.HasPrefix(string(body), "<!doctype") {
		t.Fatal("JS asset path must not SPA-fallback to HTML when dist/assets exists on disk")
	}
}

func TestSPAFallback(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, StaticHandler()))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/settings")
	if resp.StatusCode != 200 {
		t.Fatalf("SPA fallback should 200, got %d", resp.StatusCode)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
