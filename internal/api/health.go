package api

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

func registerHealth(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		claudeOK, claudeVer := probe("claude", "--version")
		gitOK, _ := probe("git", "--version")
		ready := claudeOK && gitOK
		msg := ""
		switch {
		case !claudeOK:
			msg = "Claude Code CLI not found. Install it and run `claude login`, then reload."
		case !gitOK:
			msg = "git not found on PATH — myAudit needs it to snapshot the workspace."
		}
		writeJSON(w, map[string]any{
			"ready":         ready,
			"claude":        claudeOK,
			"claudeVersion": claudeVer,
			"git":           gitOK,
			"message":       msg,
		})
	})
}

func probe(name string, args ...string) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return false, ""
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return true, line
}
