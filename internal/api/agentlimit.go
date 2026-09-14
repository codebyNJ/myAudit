package api

import (
	"context"
	"os"
	"runtime"
	"strconv"
)

const minConcurrentAgents = 4

// resolveMaxConcurrent picks parallel agent slots: env override (floored at 4),
// else max(4, NumCPU()).
func resolveMaxConcurrent() int {
	if n, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_AGENTS")); err == nil && n > 0 {
		return max(n, minConcurrentAgents)
	}
	if n, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_CLAUDE")); err == nil && n > 0 {
		return max(n, minConcurrentAgents)
	}
	n := runtime.NumCPU()
	if n < minConcurrentAgents {
		n = minConcurrentAgents
	}
	return n
}

type agentLimiter struct {
	ch chan struct{}
}

var globalAgentLimit *agentLimiter

func setAgentLimit(n int) {
	if n < 1 {
		n = 1
	}
	globalAgentLimit = &agentLimiter{ch: make(chan struct{}, n)}
	for i := 0; i < n; i++ {
		globalAgentLimit.ch <- struct{}{}
	}
}

func acquireAgent(ctx context.Context) error {
	if globalAgentLimit == nil {
		return nil
	}
	select {
	case <-globalAgentLimit.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseAgent() {
	if globalAgentLimit == nil {
		return
	}
	globalAgentLimit.ch <- struct{}{}
}
