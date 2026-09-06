package config

import (
	"os"
	"testing"
)

func TestLoadReadsEnv(t *testing.T) {
	os.Setenv("MYAUDIT_DB", "/tmp/x.db")
	defer os.Unsetenv("MYAUDIT_DB")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DBPath != "/tmp/x.db" {
		t.Fatalf("path=%q", c.DBPath)
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("MYAUDIT_DB")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DBPath != "myaudit.db" {
		t.Fatalf("default path=%q", c.DBPath)
	}
}
