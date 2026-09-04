package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds the orchestrator's runtime configuration, loaded from env.
type Config struct {
	DatabaseURL         string
	MaxConcurrentClaude int
}

// Load reads configuration from the environment. DATABASE_URL is required;
// MAX_CONCURRENT_CLAUDE defaults to 2 (bounded concurrency).
func Load() (Config, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return Config{}, errors.New("DATABASE_URL required")
	}
	n, _ := strconv.Atoi(os.Getenv("MAX_CONCURRENT_CLAUDE"))
	if n == 0 {
		n = 2
	}
	return Config{DatabaseURL: url, MaxConcurrentClaude: n}, nil
}
