package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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

func hasUIAssets(root string) bool {
	matches, err := filepath.Glob(filepath.Join(root, "assets", "index-*.js"))
	if err != nil || len(matches) == 0 {
		return false
	}
	data, err := os.ReadFile(filepath.Join(root, "index.html"))
	return err == nil && strings.Contains(string(data), `id="root"`)
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
