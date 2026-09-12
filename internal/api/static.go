package api

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	for _, rel := range []string{"internal/api/web/dist", "web/dist"} {
		root := rel
		if base != "" {
			root = filepath.Join(base, rel)
		}
		if hasUIAssets(root) {
			return root
		}
	}
	return ""
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
	info, err := os.Stat(filepath.Join(root, "assets"))
	return err == nil && info.IsDir()
}

func spaFileServer(files http.FileSystem) http.Handler {
	fileServer := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" || !fileExists(files, p) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			r.URL.Path = "/"
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
