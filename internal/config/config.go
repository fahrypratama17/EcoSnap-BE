package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func Load() (Config, error) {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, errors.New("database url kosong")
	}

	return Config{HTTPAddr: addr, DatabaseURL: dbURL}, nil
}
