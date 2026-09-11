package config

import "os"

type Config struct {
	DBPath string
}

func Load() (Config, error) {
	path := os.Getenv("MYAUDIT_DB")
	if path == "" {
		path = "myaudit.db"
	}
	return Config{DBPath: path}, nil
}
