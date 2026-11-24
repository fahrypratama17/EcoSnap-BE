package config

import (
	"bufio"
	"errors"
	"os"

	"github.com/joho/godotenv"
)

const dotenvPath = ".env"

var errDotenv = errors.New("gagal memuat file .env")

// loadDotenv imports .env from the working directory into the process
// environment without overriding variables that are already set. Errors are
// deliberately generic so file contents never leak.
func loadDotenv() error {
	f, err := os.Open(dotenvPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errDotenv
	}
	defer f.Close()

	r := bufio.NewReader(f)
	if bom, err := r.Peek(3); err == nil && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		_, _ = r.Discard(3)
	}
	values, err := godotenv.Parse(r)
	if err != nil {
		return errDotenv
	}
	for k, v := range values {
		if _, ok := os.LookupEnv(k); ok {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return errDotenv
		}
	}
	return nil
}

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func Load() (Config, error) {
	if err := loadDotenv(); err != nil {
		return Config{}, err
	}
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
