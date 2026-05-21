package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"lota/config"
	"lota/runner"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindCommandByPath(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "build", Script: "go build"},
		},
		Groups: []config.Group{
			{
				Name: "infra",
				Groups: []config.Group{
					{
						Name:     "docker",
						Commands: []config.Command{{Name: "up", Script: "docker up"}},
					},
				},
				Commands: []config.Command{{Name: "deploy", Script: "deploy"}},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		cmdName string
	}{
		{"top-level command", "build", false, "build"},
		{"nested command", "infra.docker.up", false, "up"},
		{"group command", "infra.deploy", false, "deploy"},
		{"missing", "missing", true, ""},
		{"invalid traverse", "build.nested", true, ""},
		{"group only", "infra", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindCommandByPath(cfg, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindCommandByPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result.Command.Name != tt.cmdName {
				t.Errorf("FindCommandByPath(%q) command = %v, want %v", tt.path, result.Command.Name, tt.cmdName)
			}
		})
	}
}

func TestResolveDependencies(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "build", Script: "go build"},
			{Name: "lint", Script: "go vet", Depends: []string{"build"}},
			{Name: "test", Script: "go test", Depends: []string{"build", "lint"}},
		},
		Groups: []config.Group{
			{
				Name: "backend",
				Commands: []config.Command{
					{Name: "run", Script: "go run", Depends: []string{"build"}},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	t.Run("linear deps", func(t *testing.T) {
		result, _ := FindCommandByPath(cfg, "test")
		deps, err := ResolveDependencies(cfg, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deps) != 2 {
			t.Fatalf("expected 2 deps, got %d", len(deps))
		}
		if deps[0].Command.Name != "build" {
			t.Errorf("first dep = %v, want build", deps[0].Command.Name)
		}
		if deps[1].Command.Name != "lint" {
			t.Errorf("second dep = %v, want lint", deps[1].Command.Name)
		}
	})

	t.Run("no deps", func(t *testing.T) {
		result, _ := FindCommandByPath(cfg, "build")
		deps, err := ResolveDependencies(cfg, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deps) != 0 {
			t.Errorf("expected 0 deps, got %d", len(deps))
		}
	})

	t.Run("circular dependency", func(t *testing.T) {
		badCfg := &config.AppConfig{
			Commands: []config.Command{
				{Name: "a", Script: "echo a", Depends: []string{"b"}},
				{Name: "b", Script: "echo b", Depends: []string{"a"}},
			},
		}
		if err := badCfg.BuildIndexes(); err != nil {
			t.Fatalf("BuildIndexes() error: %v", err)
		}
		result, _ := FindCommandByPath(badCfg, "a")
		_, err := ResolveDependencies(badCfg, result)
		if err == nil {
			t.Error("expected circular dependency error, got nil")
		}
	})

	t.Run("nested group dependency", func(t *testing.T) {
		result, _ := FindCommandByPath(cfg, "backend.run")
		deps, err := ResolveDependencies(cfg, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deps) != 1 || deps[0].Command.Name != "build" {
			t.Errorf("expected [build], got %v", deps)
		}
	})
}

func TestResolveDependencyLevels(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "compile", Script: "echo compile"},
			{Name: "build", Script: "echo build", Depends: []string{"compile"}},
			{Name: "lint", Script: "echo lint"},
			{Name: "test", Script: "echo test", Depends: []string{"build", "lint"}},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	result, _ := FindCommandByPath(cfg, "test")
	levels, err := resolveDependencyLevels(cfg, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(levels) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(levels))
	}

	// Level 0: compile, lint (no deps)
	lvl0 := levels[0]
	if len(lvl0) != 2 {
		t.Errorf("level 0 expected 2 commands, got %d", len(lvl0))
	}
	names := make([]string, len(lvl0))
	for i, r := range lvl0 {
		names[i] = r.Command.Name
	}
	if !contains(names, "compile") || !contains(names, "lint") {
		t.Errorf("level 0 expected [compile lint], got %v", names)
	}

	// Level 1: build (depends on compile)
	lvl1 := levels[1]
	if len(lvl1) != 1 || lvl1[0].Command.Name != "build" {
		t.Errorf("level 1 expected [build], got %v", lvl1)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestResolveDependencyLevels_ParallelFalse(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "a", Script: "echo a"},
			{Name: "b", Script: "echo b"},
			{Name: "c", Script: "echo c", Depends: []string{"a", "b"}},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	result, _ := FindCommandByPath(cfg, "c")
	levels, err := resolveDependencyLevels(cfg, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(levels))
	}
	if len(levels[0]) != 2 {
		t.Fatalf("expected 2 commands in level 0, got %d", len(levels[0]))
	}
}

func TestRunCommand_SequentialPrintsDependencyProgress(t *testing.T) {
	parallelFalse := false
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "build", Script: "echo build >/dev/null"},
			{Name: "test", Script: "echo test >/dev/null", Depends: []string{"build"}, Parallel: &parallelFalse},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	result, err := FindCommandByPath(cfg, "test")
	if err != nil {
		t.Fatalf("FindCommandByPath() error: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w

	runErr := RunCommand(context.Background(), cfg, result, nil, runner.RunOptions{})

	_ = w.Close()
	os.Stdout = oldStdout

	if runErr != nil {
		t.Fatalf("RunCommand() error: %v", runErr)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed reading output: %v", err)
	}

	if !strings.Contains(buf.String(), "=> Running dependency: build") {
		t.Fatalf("expected dependency progress output, got %q", buf.String())
	}
}

func TestRunCommand_ParallelDoesNotBlockLevels(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "server", Script: "sleep 30"},
			{Name: "client", Script: "echo client", Depends: []string{"server"}},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	result, err := FindCommandByPath(cfg, "client")
	if err != nil {
		t.Fatalf("FindCommandByPath() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = RunCommand(ctx, cfg, result, nil, runner.RunOptions{})
	if err == nil {
		t.Fatal("expected timeout from background server, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}
}

func TestRunCommand_NestedCommandArgsDefault(t *testing.T) {
	yamlContent := `
version:
  desc: Version management commands
  bump:
    args:
    - "type:str=none"
    desc: Bump version
    script: |
      echo $type
`
	tmpFile := filepath.Join(t.TempDir(), "lota.yml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.ParseConfig(tmpFile)
	if err != nil {
		t.Fatalf("ParseConfig() error: %v", err)
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	result, err := FindCommandByPath(cfg, "version.bump")
	if err != nil {
		t.Fatalf("FindCommandByPath() error: %v", err)
	}

	if err := RunCommand(context.Background(), cfg, result, []string{}, runner.RunOptions{DryRun: true}); err != nil {
		t.Fatalf("RunCommand() error: %v", err)
	}
}
