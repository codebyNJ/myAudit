package api

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed all:web/dist
var webFS embed.FS

func StaticHandler() http.Handler {
	if root := builtDistOnDisk(); root != "" {
		return spaFileServer(http.Dir(root))
	}
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	return spaFileServer(http.FS(sub))
}

// builtDistOnDisk returns a Vite output dir when assets/ exists on disk.
// Phase 3 stopped committing build artifacts; after `make ui-build` this lets
// a running `go run` serve the fresh bundle without recompiling the binary.
func builtDistOnDisk() string {
	if root := strings.TrimSpace(os.Getenv("MYAUDIT_WEB_ROOT")); root != "" && hasUIAssets(root) {
		return root
	}
	base := repoRoot()
	var best string
	var bestMod time.Time
	for _, rel := range []string{"web/dist", "internal/api/web/dist"} {
		root := rel
		if base != "" {
			root = filepath.Join(base, rel)
		}
		if !hasUIAssets(root) {
			continue
		}
		mod := uiBundleModTime(root)
		if best == "" || mod.After(bestMod) {
			best = root
			bestMod = mod
		}
	}
	return best
}

func uiBundleModTime(root string) time.Time {
	fi, err := os.Stat(filepath.Join(root, "index.html"))
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// entryScriptRe pulls the bundle that index.html actually loads.
var entryScriptRe = regexp.MustCompile(`src="(/assets/[^"]+\.js)"`)

// hasUIAssets reports whether root holds a *self-consistent* built UI.
//
// Checking that some index-*.js exists is not enough. index.html and the
// assets beside it can disagree: internal/api/web/dist/index.html is tracked
// while everything else in that directory is gitignored, so any checkout,
// branch switch or stash restores a placeholder naming a bundle that was
// deleted by the last build. Because that placeholder was just written, its
// mtime is newest, and builtDistOnDisk preferred it — serving a page whose own
// entry point 404s, which looks exactly like the app crashing.
func hasUIAssets(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil || !strings.Contains(string(data), `id="root"`) {
		return false
	}
	m := entryScriptRe.FindSubmatch(data)
	if m == nil {
		return false
	}
	entry := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(string(m[1]), "/")))
	if _, err := os.Stat(entry); err != nil {
		slog.Warn("ignoring stale UI build: index.html references a bundle that is not there",
			"dir", root, "bundle", string(m[1]), "hint", "run `make ui-build`")
		return false
	}
	return true
}

func spaFileServer(files http.FileSystem) http.Handler {
	fileServer := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case p == "":
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			r.URL.Path = "/"
		case !fileExists(files, p):
			// Missing hashed bundles must 404 — SPA HTML as JS/CSS causes a blank screen.
			if strings.HasPrefix(p, "assets/") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			r.URL.Path = "/"
		case p == "index.html":
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func fileExists(files http.FileSystem, p string) bool {
	f, err := files.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	return err == nil && !st.IsDir()
}
