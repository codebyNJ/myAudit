package config

import (
	"os"
	"testing"
)

func TestLoadReadsEnv(t *testing.T) {
	os.Setenv("MYAUDIT_DB", "/tmp/x.db")
	os.Setenv("MAX_CONCURRENT_CLAUDE", "3")
	defer os.Unsetenv("MYAUDIT_DB")
	defer os.Unsetenv("MAX_CONCURRENT_CLAUDE")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DBPath != "/tmp/x.db" {
		t.Fatalf("path=%q", c.DBPath)
	}
	if c.MaxConcurrentClaude != 3 {
		t.Fatalf("n=%d", c.MaxConcurrentClaude)
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("MYAUDIT_DB")
	os.Unsetenv("MAX_CONCURRENT_CLAUDE")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DBPath != "myaudit.db" {
		t.Fatalf("default path=%q", c.DBPath)
	}
	if c.MaxConcurrentClaude != 2 {
		t.Fatalf("default n=%d", c.MaxConcurrentClaude)
	}
}
