package config

import (
	"os"
	"strconv"
)

// Config holds the runtime configuration, loaded from env.
type Config struct {
	DBPath              string
	MaxConcurrentClaude int
}

// Load reads configuration from the environment. MYAUDIT_DB overrides the
// SQLite file path (default ./myaudit.db — this is a local, single-file IDE).
// MAX_CONCURRENT_CLAUDE defaults to 2 (bounded concurrency).
func Load() (Config, error) {
	path := os.Getenv("MYAUDIT_DB")
	if path == "" {
		path = "myaudit.db"
	}
	n, _ := strconv.Atoi(os.Getenv("MAX_CONCURRENT_CLAUDE"))
	if n == 0 {
		n = 2
	}
	return Config{DBPath: path, MaxConcurrentClaude: n}, nil
}
