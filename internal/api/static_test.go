package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestMissingAssetReturns404(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, StaticHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/assets/index-stale-missing.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 404 {
		t.Fatalf("missing asset should 404, got %d body=%q", resp.StatusCode, string(body)[:min(80, len(body))])
	}
	if strings.HasPrefix(strings.ToLower(string(body)), "<!doctype") {
		t.Fatal("missing asset must not SPA-fallback to HTML")
	}
}

func TestSPAFallback(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, StaticHandler()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/settings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
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

// Writing a dist whose index.html names `bundle`, with `present` controlling
// whether that bundle is actually on disk.
func writeDist(t *testing.T, dir, bundle string, present bool) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html><html><head><script type="module" crossorigin src="/assets/` + bundle + `"></script></head><body><div id="root"></div></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	if present {
		if err := os.WriteFile(filepath.Join(dir, "assets", bundle), []byte("console.log(1)"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The failure this guards: index.html is tracked while the assets beside it
// are gitignored, so a checkout or stash restores a placeholder naming a
// bundle the last build deleted. Serving that gives a page whose own entry
// point 404s — indistinguishable from the app crashing.
func TestHasUIAssetsRejectsIndexPointingAtAMissingBundle(t *testing.T) {
	dir := writeDist(t, t.TempDir(), "index-GONE.js", false)
	// A different bundle *is* present, which is exactly the stale case: the
	// old glob check passed on this.
	if err := os.WriteFile(filepath.Join(dir, "assets", "index-FRESH.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasUIAssets(dir) {
		t.Fatal("a dist whose index.html names a missing bundle must not be served")
	}
}

func TestHasUIAssetsAcceptsASelfConsistentBuild(t *testing.T) {
	if !hasUIAssets(writeDist(t, t.TempDir(), "index-OK.js", true)) {
		t.Fatal("a build whose index.html matches its assets should be served")
	}
}

func TestHasUIAssetsRejectsAnIndexWithNoBundle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<div id="root"></div>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasUIAssets(dir) {
		t.Fatal("an index.html that loads no bundle is not a built UI")
	}
}
