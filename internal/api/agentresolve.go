package api

import (
	"context"
	"os"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/agent/claude"
	"github.com/codebyNJ/myAudit/internal/agent/opencode"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

var tierToModel = map[string]string{
	"opus-4.8":  "claude-opus-4-8",
	"sonnet-5":  "claude-sonnet-5",
	"haiku-4.5": "claude-haiku-4-5-20251001",
}

const (
	defaultModel         = "claude-haiku-4-5-20251001"
	defaultOpenCodeModel = "anthropic/claude-haiku-4-5"
)

func resolveProvider(ctx context.Context, s *store.Store) string {
	if p := os.Getenv("AGENT_PROVIDER"); p == agent.ProviderClaude || p == agent.ProviderOpenCode {
		return p
	}
	if s != nil {
		if st, err := s.GetSettings(ctx); err == nil {
			if p, _ := st["agent_provider"].(string); p == agent.ProviderClaude || p == agent.ProviderOpenCode {
				return p
			}
		}
	}
	return agent.ProviderClaude
}

func resolveModel(ctx context.Context, s *store.Store) string {
	if m := os.Getenv("CLAUDE_MODEL"); m != "" {
		return m
	}
	if s != nil {
		if st, err := s.GetSettings(ctx); err == nil {
			if tier, _ := st["model_tier"].(string); tier != "" {
				if id := tierToModel[tier]; id != "" {
					return id
				}
			}
		}
	}
	return defaultModel
}

func resolveOpenCodeModel(ctx context.Context, s *store.Store) string {
	if m := os.Getenv("OPENCODE_MODEL"); m != "" {
		return m
	}
	if s != nil {
		if st, err := s.GetSettings(ctx); err == nil {
			if m, _ := st["opencode_model"].(string); m != "" {
				return m
			}
		}
	}
	return defaultOpenCodeModel
}

func runAgent(
	ctx context.Context,
	s *store.Store,
	ws sandbox.Workspace,
	task string,
	mode agent.Mode,
	isolate bool,
	image string,
	claudeBin string,
	opencodeBin string,
	onStep func(string),
) (agent.Result, error) {
	switch resolveProvider(ctx, s) {
	case agent.ProviderOpenCode:
		return opencode.Run(ctx, ws, task, mode, opencode.Options{
			Model:  resolveOpenCodeModel(ctx, s),
			Bin:    opencodeBin,
			OnStep: onStep,
		})
	default:
		allow, deny := agent.PolicyFor(mode)
		return claude.Run(ctx, ws, task, claude.Options{
			Model:   resolveModel(ctx, s),
			Allow:   allow,
			Deny:    deny,
			Isolate: isolate,
			Image:   image,
			Bin:     claudeBin,
			OnStep:  onStep,
		})
	}
}
