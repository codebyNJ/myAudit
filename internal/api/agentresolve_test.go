package api

import (
	"context"
	"testing"

	"github.com/codebyNJ/myAudit/internal/agent"
)

func TestResolveProvider(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()

	if p := resolveProvider(ctx, s); p != agent.ProviderClaude {
		t.Fatalf("default provider should be claude, got %s", p)
	}
	if _, err := s.PutSettings(ctx, map[string]any{"agent_provider": "opencode"}); err != nil {
		t.Fatal(err)
	}
	if p := resolveProvider(ctx, s); p != agent.ProviderOpenCode {
		t.Fatalf("settings should select opencode, got %s", p)
	}
	t.Setenv("AGENT_PROVIDER", "claude")
	if p := resolveProvider(ctx, s); p != agent.ProviderClaude {
		t.Fatalf("env must override settings, got %s", p)
	}
}

func TestResolveOpenCodeModel(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()

	t.Setenv("OPENCODE_MODEL", "")
	if m := resolveOpenCodeModel(ctx, s); m != defaultOpenCodeModel {
		t.Fatalf("default opencode model: got %s", m)
	}
	if _, err := s.PutSettings(ctx, map[string]any{"opencode_model": "openai/gpt-4o"}); err != nil {
		t.Fatal(err)
	}
	if m := resolveOpenCodeModel(ctx, s); m != "openai/gpt-4o" {
		t.Fatalf("settings model: got %s", m)
	}
	t.Setenv("OPENCODE_MODEL", "anthropic/claude-sonnet-4")
	if m := resolveOpenCodeModel(ctx, s); m != "anthropic/claude-sonnet-4" {
		t.Fatalf("env must override settings, got %s", m)
	}
}
