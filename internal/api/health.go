package api

import (
	"context"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/store"
)

type providerStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

func registerHealth(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		claudeOK, claudeVer := probe("claude", "--version")
		opencodeOK, opencodeVer := probe("opencode", "--version")
		gitOK, _ := probe("git", "--version")

		active := resolveProvider(r.Context(), s)
		agentOK := claudeOK
		if active == agent.ProviderOpenCode {
			agentOK = opencodeOK
		}
		ready := gitOK && agentOK

		msg := ""
		switch {
		case !gitOK:
			msg = "git not found on PATH — myAudit needs it to snapshot the workspace."
		case active == agent.ProviderOpenCode && !opencodeOK:
			msg = "OpenCode CLI not found. Install it and run `opencode auth login`, then reload."
		case active == agent.ProviderClaude && !claudeOK:
			msg = "Claude Code CLI not found. Install it and run `claude login`, then reload."
		}

		writeJSON(w, map[string]any{
			"ready":         ready,
			"git":           gitOK,
			"agentProvider": active,
			"providers": map[string]providerStatus{
				agent.ProviderClaude:   {Installed: claudeOK, Version: claudeVer},
				agent.ProviderOpenCode: {Installed: opencodeOK, Version: opencodeVer},
			},
			"claude":        claudeOK,
			"claudeVersion": claudeVer,
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
