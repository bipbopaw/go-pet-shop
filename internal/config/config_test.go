package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMustLoadReadsEnvironmentAndConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	configYAML := "env: dev\nhttp_server:\n  address: 127.0.0.1:9090\n  timeout: 5s\n  idle_timeout: 30s\n  database_timeout: 8s\n"
	if err := os.WriteFile(filepath.Join(configDir, "dev.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("DB_TIMEOUT", "3s")

	config := MustLoad()
	if config.Env != "dev" || config.DatabaseURL != "postgres://test" {
		t.Fatalf("unexpected environment config: %+v", config)
	}
	if config.Address != "127.0.0.1:9090" || config.Timeout != 5*time.Second || config.IdleTimeout != 30*time.Second {
		t.Fatalf("unexpected HTTP config: %+v", config.HTTPServer)
	}
	if config.DatabaseTimeout != 3*time.Second {
		t.Fatalf("expected DB timeout from environment, got %s", config.DatabaseTimeout)
	}
}

func TestMustLoadPanicsWhenConfigFileIsMissing(t *testing.T) {
	withConfigDirectory(t, t.TempDir())
	t.Setenv("APP_ENV", "missing")
	t.Setenv("DATABASE_URL", "postgres://test")

	assertPanicsContaining(t, "config file not found", func() { MustLoad() })
}

func TestMustLoadPanicsWhenDatabaseURLIsMissing(t *testing.T) {
	root := t.TempDir()
	withConfigDirectory(t, root)
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "")

	assertPanicsContaining(t, "DATABASE_URL not set", func() { MustLoad() })
}

func withConfigDirectory(t *testing.T, root string) {
	t.Helper()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "local.yaml"), []byte("env: local\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })
}

func assertPanicsContaining(t *testing.T, want string, run func()) {
	t.Helper()
	defer func() {
		got := recover()
		if got == nil || !strings.Contains(fmt.Sprint(got), want) {
			t.Fatalf("expected panic containing %q, got %v", want, got)
		}
	}()
	run()
}
