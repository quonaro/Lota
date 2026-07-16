package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvironmentFile_RespectsReservedSystemVars(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("env file with reserved variable", func(t *testing.T) {
		path := filepath.Join(tmpDir, "bad.env")
		if err := os.WriteFile(path, []byte("PATH=/overridden\nOK=value\n"), 0644); err != nil {
			t.Fatalf("write env file: %v", err)
		}

		_, err := loadEnvironmentFile(path, path, "env", "")
		if err == nil {
			t.Fatal("expected error for reserved env variable, got nil")
		}
		if !strings.Contains(err.Error(), "PATH") {
			t.Fatalf("expected error to mention PATH, got: %v", err)
		}
	})

	t.Run("yaml file with reserved variable", func(t *testing.T) {
		path := filepath.Join(tmpDir, "bad.yaml")
		if err := os.WriteFile(path, []byte("PATH: /overridden\nok: value\n"), 0644); err != nil {
			t.Fatalf("write yaml file: %v", err)
		}

		_, err := loadEnvironmentFile(path, path, "yaml", "")
		if err == nil {
			t.Fatal("expected error for reserved yaml variable, got nil")
		}
		if !strings.Contains(err.Error(), "PATH") {
			t.Fatalf("expected error to mention PATH, got: %v", err)
		}
	})

	t.Run("env file with safe variables", func(t *testing.T) {
		path := filepath.Join(tmpDir, "good.env")
		if err := os.WriteFile(path, []byte("FOO=bar\nBAZ=qux\n"), 0644); err != nil {
			t.Fatalf("write env file: %v", err)
		}

		vars, err := loadEnvironmentFile(path, path, "env", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vars) != 2 {
			t.Fatalf("expected 2 vars, got %d", len(vars))
		}
	})
}
