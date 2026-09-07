package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LiveView is what the agent-screen viewer renders: a live URL when the agent
// currently has the product running, otherwise the most recent captured frame.
type LiveView struct {
	Status string `json:"status"` // "live" | "frames" | "idle"
	URL    string `json:"url,omitempty"`
	Frame  string `json:"frame,omitempty"`
	At     string `json:"at,omitempty"`
	Title  string `json:"title,omitempty"`
}

// liveTarget is the file the QA agent writes while it has the product running.
type liveTarget struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// reachable reports whether the host:port behind a URL is accepting connections,
// so a stale live.json (agent killed its server, or crashed) never leaves the
// viewer pointing at a dead page.
func reachable(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "https" {
			host = net.JoinHostPort(u.Hostname(), "443")
		} else {
			host = net.JoinHostPort(u.Hostname(), "80")
		}
	}
	c, err := net.DialTimeout("tcp", host, 400*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// newestFrame returns the most recently modified image under the run's .myaudit
// capture dirs, workspace-relative.
func newestFrame(root string) (string, time.Time) {
	var best string
	var bestAt time.Time
	for _, sub := range []string{".myaudit/live", ".myaudit/preview"} {
		dir := filepath.Join(root, sub)
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !isImage(p) {
				return nil
			}
			info, ierr := d.Info()
			if ierr != nil {
				return nil
			}
			if info.ModTime().After(bestAt) {
				bestAt = info.ModTime()
				if rel, rerr := filepath.Rel(root, p); rerr == nil {
					best = filepath.ToSlash(rel)
				}
			}
			return nil
		})
		if best != "" {
			break // prefer live captures over end-of-module previews
		}
	}
	return best, bestAt
}

func isImage(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

// liveHandler serves the agent-screen source for a run.
func liveHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	root := filepath.Join("runs", id.String())

	if b, rerr := os.ReadFile(filepath.Join(root, ".myaudit", "live.json")); rerr == nil {
		var t liveTarget
		if json.Unmarshal(b, &t) == nil && t.URL != "" && reachable(t.URL) {
			writeJSON(w, LiveView{Status: "live", URL: t.URL, Title: t.Title})
			return
		}
	}

	if frame, at := newestFrame(root); frame != "" {
		writeJSON(w, LiveView{Status: "frames", Frame: frame, At: at.UTC().Format(time.RFC3339)})
		return
	}

	writeJSON(w, LiveView{Status: "idle"})
}
