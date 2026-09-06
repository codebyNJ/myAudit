package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web/dist
var webFS embed.FS

// StaticHandler serves the built React SPA (web/dist) with a fallback to
// index.html for client-side routes.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve real files; otherwise fall back to index.html (SPA).
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" || func() bool { _, err := fs.Stat(sub, p); return err != nil }() {
			// The SPA shell must never be cached, or a rebuilt UI (new hashed
			// bundle names) is masked by a stale index.html → blank/old page.
			// Hashed asset files are safe to cache; they aren't touched here.
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
