package config

import "os"

// Config holds the runtime configuration, loaded from env.
type Config struct {
	DBPath string
}

// Load reads configuration from the environment. MYAUDIT_DB overrides the
// SQLite file path (default ./myaudit.db — this is a local, single-file IDE).
func Load() (Config, error) {
	path := os.Getenv("MYAUDIT_DB")
	if path == "" {
		path = "myaudit.db"
	}
	return Config{DBPath: path}, nil
}
