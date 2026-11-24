package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolate switches to an empty temp dir and guarantees the given keys are
// unset during the test and restored afterwards (Load uses os.Setenv, which
// t.Setenv cleanup does not undo).
func isolate(t *testing.T, keys ...string) string {
	t.Helper()
	for _, k := range keys {
		old, had := os.LookupEnv(k)
		t.Cleanup(func() {
			if had {
				os.Setenv(k, old)
			} else {
				os.Unsetenv(k)
			}
		})
		os.Unsetenv(k)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func writeEnv(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

var allKeys = []string{"DATABASE_URL", "HTTP_ADDR", "GOOGLE_APPLICATION_CREDENTIALS", "DERIVED_URL", "EARLY_KEY"}

func TestLoadImportsDotenvIntoProcessEnv(t *testing.T) {
	dir := isolate(t, allKeys...)
	writeEnv(t, dir, "# comment\nexport DATABASE_URL=\"postgres://u:p@h/db\"\nHTTP_ADDR=0.0.0.0:9000\n"+
		"GOOGLE_APPLICATION_CREDENTIALS=\"C:/Users/My Name/firebase key.json\"\nDERIVED_URL=${DATABASE_URL}?sslmode=require\n")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://u:p@h/db" || cfg.HTTPAddr != "0.0.0.0:9000" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if got := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); got != "C:/Users/My Name/firebase key.json" {
		t.Fatalf("credentials = %q", got)
	}
	if got := os.Getenv("DERIVED_URL"); got != "postgres://u:p@h/db?sslmode=require" {
		t.Fatalf("interpolation = %q", got)
	}
}

func TestLoadProcessEnvWinsIncludingEmpty(t *testing.T) {
	dir := isolate(t, allKeys...)
	writeEnv(t, dir, "DATABASE_URL=file-db\nHTTP_ADDR=file:1\n")
	os.Setenv("DATABASE_URL", "process-db")
	os.Setenv("HTTP_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "process-db" {
		t.Fatalf("db = %q", cfg.DatabaseURL)
	}
	if v, ok := os.LookupEnv("HTTP_ADDR"); !ok || v != "" {
		t.Fatalf("explicit empty HTTP_ADDR overridden: %q %v", v, ok)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("addr = %q", cfg.HTTPAddr)
	}
}

func TestLoadWithoutDotenvUsesOSEnv(t *testing.T) {
	isolate(t, allKeys...)
	os.Setenv("DATABASE_URL", "os-db")
	cfg, err := Load()
	if err != nil || cfg.DatabaseURL != "os-db" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}

func TestLoadMissingDatabaseURLFails(t *testing.T) {
	dir := isolate(t, allKeys...)
	writeEnv(t, dir, "HTTP_ADDR=1.2.3.4:5\n")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadEmptyDotenvIsFine(t *testing.T) {
	dir := isolate(t, allKeys...)
	writeEnv(t, dir, "")
	os.Setenv("DATABASE_URL", "os-db")
	cfg, err := Load()
	if err != nil || cfg.DatabaseURL != "os-db" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}

func TestLoadBOMAndCRLF(t *testing.T) {
	dir := isolate(t, allKeys...)
	writeEnv(t, dir, "\xEF\xBB\xBFDATABASE_URL=bom-db\r\nHTTP_ADDR=1.1.1.1:1\r\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "bom-db" || cfg.HTTPAddr != "1.1.1.1:1" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestLoadMalformedDotenvFailsSafelyWithoutPartialImport(t *testing.T) {
	dir := isolate(t, allKeys...)
	const secret = "SUPERSECRETVALUE"
	writeEnv(t, dir, "EARLY_KEY="+secret+"\nDATABASE_URL=\"unterminated "+secret+"\n")
	t.Setenv("DATABASE_URL", "process-db")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks secret: %v", err)
	}
	if _, ok := os.LookupEnv("EARLY_KEY"); ok {
		t.Fatal("earlier key imported despite parse failure")
	}
}

func TestLoadDotenvDirectoryFailsSafely(t *testing.T) {
	dir := isolate(t, allKeys...)
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0o700); err != nil {
		t.Fatal(err)
	}
	os.Setenv("DATABASE_URL", "os-db")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadIgnoresDotenvExample(t *testing.T) {
	dir := isolate(t, allKeys...)
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("DATABASE_URL=example-db\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database url error")
	}
	if _, ok := os.LookupEnv("DATABASE_URL"); ok {
		t.Fatal(".env.example was loaded")
	}
}

func TestLoadEmptyProcessDatabaseURLDoesNotUseFile(t *testing.T) {
	dir := isolate(t, allKeys...)
	writeEnv(t, dir, "DATABASE_URL=file-db\n")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "database url kosong") {
		t.Fatalf("expected missing database configuration, got %v", err)
	}
	if value, ok := os.LookupEnv("DATABASE_URL"); !ok || value != "" {
		t.Fatalf("explicit empty DATABASE_URL overridden: %q %v", value, ok)
	}
}
