package config

import (
	"os"
	"testing"
)

func TestLoadReadsEnv(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://x")
	os.Setenv("MAX_CONCURRENT_CLAUDE", "3")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != "postgres://x" {
		t.Fatalf("url=%q", c.DatabaseURL)
	}
	if c.MaxConcurrentClaude != 3 {
		t.Fatalf("n=%d", c.MaxConcurrentClaude)
	}
}

func TestLoadFailsWithoutURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	if _, err := Load(); err == nil {
		t.Fatal("want error")
	}
}
