package api

import (
	"runtime"
	"testing"
)

func TestResolveMaxConcurrentDynamicFloor(t *testing.T) {
	t.Setenv("MAX_CONCURRENT_AGENTS", "")
	t.Setenv("MAX_CONCURRENT_CLAUDE", "")
	got := resolveMaxConcurrent()
	want := runtime.NumCPU()
	if want < minConcurrentAgents {
		want = minConcurrentAgents
	}
	if got != want {
		t.Fatalf("dynamic default: got %d want %d (NumCPU=%d)", got, want, runtime.NumCPU())
	}
}

func TestResolveMaxConcurrentEnvOverride(t *testing.T) {
	t.Setenv("MAX_CONCURRENT_AGENTS", "8")
	t.Setenv("MAX_CONCURRENT_CLAUDE", "")
	if got := resolveMaxConcurrent(); got != 8 {
		t.Fatalf("env=8: got %d", got)
	}
}

func TestResolveMaxConcurrentEnvFlooredAtMin(t *testing.T) {
	t.Setenv("MAX_CONCURRENT_AGENTS", "2")
	if got := resolveMaxConcurrent(); got != minConcurrentAgents {
		t.Fatalf("env=2 should floor to %d, got %d", minConcurrentAgents, got)
	}
}
