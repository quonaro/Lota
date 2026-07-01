package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quonaro/lota/config"
)

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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

	result, _ := config.FindCommandByPath(cfg, "test")
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

	result, _ := config.FindCommandByPath(cfg, "c")
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

	var stdout bytes.Buffer
	opts := Options{Stdout: &stdout, Stderr: &stdout}

	err := Run(context.Background(), cfg, []string{"test"}, opts)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(stdout.String(), "=> Running dependency: build") {
		t.Fatalf("expected dependency progress output, got %q", stdout.String())
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Run(ctx, cfg, []string{"client"}, Options{})
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

	var stdout bytes.Buffer
	if err := Run(context.Background(), cfg, []string{"version", "bump"}, Options{DryRun: true, Stdout: &stdout, Stderr: &stdout}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestRunCommand(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "hello", Script: "echo hello"},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run(context.Background(), cfg, []string{"hello"}, Options{Stdout: &stdout, Stderr: &stdout}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Fatalf("expected output to contain 'hello', got %q", stdout.String())
	}
}

func TestRunCommandNotFound(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "hello", Script: "echo hello"},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	if err := Run(context.Background(), cfg, []string{"missing"}, Options{}); err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestLoadConfig(t *testing.T) {
	data := []byte(`
hello:
  script: echo hello
`)
	cfg, err := LoadConfig(data)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.Commands) != 1 || cfg.Commands[0].Name != "hello" {
		t.Fatalf("expected command 'hello', got %v", cfg.Commands)
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	data := []byte(`not valid yaml: [`)
	if _, err := LoadConfig(data); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadConfigFromPath(t *testing.T) {
	yamlContent := `
build:
  desc: Build the project
  script: echo build
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "lota.yml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, configDir, err := LoadConfigFromPath(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error: %v", err)
	}
	if len(cfg.Commands) != 1 || cfg.Commands[0].Name != "build" {
		t.Fatalf("expected command 'build', got %v", cfg.Commands)
	}
	if configDir != tmpDir {
		t.Fatalf("expected configDir %q, got %q", tmpDir, configDir)
	}
}

func TestLoadConfigFromPathNotFound(t *testing.T) {
	_, _, err := LoadConfigFromPath("/nonexistent/path/lota.yml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestPrintHelp(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{Name: "dev", Desc: "Development tasks"},
		},
		Commands: []config.Command{
			{Name: "build", Desc: "Build the project"},
			{Name: "test", Desc: "Run tests"},
		},
	}

	var buf bytes.Buffer
	PrintHelp(cfg, &buf, "myapp")

	out := buf.String()
	if !strings.Contains(out, "Usage: myapp") {
		t.Errorf("expected usage header, got: %q", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("expected command 'build', got: %q", out)
	}
	if !strings.Contains(out, "dev") {
		t.Errorf("expected group 'dev', got: %q", out)
	}
	// Must NOT contain CLI-specific global flags
	if strings.Contains(out, "--init") {
		t.Errorf("help should not contain --init in embedded mode, got: %q", out)
	}
	if strings.Contains(out, "--completion-script") {
		t.Errorf("help should not contain --completion-script in embedded mode, got: %q", out)
	}
}

func TestPrintHelpEmpty(t *testing.T) {
	cfg := &config.AppConfig{}
	var buf bytes.Buffer
	PrintHelp(cfg, &buf, "")
	out := buf.String()
	if !strings.Contains(out, "Usage: app") {
		t.Errorf("expected default app name, got: %q", out)
	}
}

func TestRun_GroupError(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "admin",
				Desc: "Administration tools",
				Commands: []config.Command{
					{Name: "users", Script: "echo users"},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	err := Run(context.Background(), cfg, []string{"admin"}, Options{})
	if err == nil {
		t.Fatal("expected error for group, got nil")
	}

	var groupErr *GroupError
	if !errors.As(err, &groupErr) {
		t.Fatalf("expected *GroupError, got %T: %v", err, err)
	}
	if groupErr.Path != "admin" {
		t.Errorf("expected Path='admin', got %q", groupErr.Path)
	}
	if len(groupErr.Groups) != 1 || groupErr.Groups[0].Name != "admin" {
		t.Errorf("expected Groups=[admin], got %v", groupErr.Groups)
	}
}

func TestPrintGroupHelp(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "admin",
				Desc: "Administration tools",
				Commands: []config.Command{
					{Name: "users", Desc: "Manage users"},
				},
				Groups: []config.Group{
					{Name: "db", Desc: "Database tools"},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	var buf bytes.Buffer
	groups := []*config.Group{&cfg.Groups[0]}
	PrintGroupHelp(cfg, groups, &buf, "outless")

	out := buf.String()
	if !strings.Contains(out, "Usage: outless admin") {
		t.Errorf("expected usage header, got: %q", out)
	}
	if !strings.Contains(out, "Administration tools") {
		t.Errorf("expected group desc, got: %q", out)
	}
	if !strings.Contains(out, "users") {
		t.Errorf("expected command 'users', got: %q", out)
	}
	if !strings.Contains(out, "db") {
		t.Errorf("expected sub-group 'db', got: %q", out)
	}
	// Must NOT contain CLI-specific global flags
	if strings.Contains(out, "--init") {
		t.Errorf("help should not contain --init in embedded mode, got: %q", out)
	}
}

func TestPrintGroupHelpFallback(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "build", Desc: "Build the project"},
		},
	}
	var buf bytes.Buffer
	PrintGroupHelp(cfg, nil, &buf, "myapp")
	out := buf.String()
	if !strings.Contains(out, "Usage: myapp") {
		t.Errorf("expected fallback to PrintHelp, got: %q", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("expected command 'build', got: %q", out)
	}
}

func TestPrintHelp_HiddenItems(t *testing.T) {
	falseVal := false
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{Name: "dev", Desc: "Development tasks"},
			{Name: "secret", Desc: "Secret tasks", Show: &falseVal},
		},
		Commands: []config.Command{
			{Name: "build", Desc: "Build the project"},
			{Name: "test", Desc: "Run tests", Show: &falseVal},
		},
	}

	var buf bytes.Buffer
	PrintHelp(cfg, &buf, "myapp")

	out := buf.String()
	if !strings.Contains(out, "dev") {
		t.Errorf("expected group 'dev', got: %q", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("expected command 'build', got: %q", out)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("help should not contain hidden group 'secret', got: %q", out)
	}
	if strings.Contains(out, "test") {
		t.Errorf("help should not contain hidden command 'test', got: %q", out)
	}
}

func TestPrintGroupHelp_HiddenItems(t *testing.T) {
	falseVal := false
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "deploy",
				Desc: "Deploy tasks",
				Groups: []config.Group{
					{Name: "canary", Desc: "Canary deploy"},
					{Name: "dark", Desc: "Dark deploy", Show: &falseVal},
				},
				Commands: []config.Command{
					{Name: "build", Desc: "Build"},
					{Name: "push", Desc: "Push", Show: &falseVal},
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintGroupHelp(cfg, []*config.Group{&cfg.Groups[0]}, &buf, "myapp")

	out := buf.String()
	if !strings.Contains(out, "canary") {
		t.Errorf("expected sub-group 'canary', got: %q", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("expected command 'build', got: %q", out)
	}
	if strings.Contains(out, "dark") {
		t.Errorf("help should not contain hidden sub-group 'dark', got: %q", out)
	}
	if strings.Contains(out, "push") {
		t.Errorf("help should not contain hidden command 'push', got: %q", out)
	}
}

func TestApp_NewApp(t *testing.T) {
	data := []byte(`
hello:
  script: echo hello
`)
	app, err := NewApp(data, Options{})
	if err != nil {
		t.Fatalf("NewApp() error: %v", err)
	}
	if app.Config() == nil {
		t.Fatal("expected non-nil config")
	}
	if app.opts.Stdout == nil {
		t.Error("expected default Stdout")
	}
}

func TestApp_NewAppInvalid(t *testing.T) {
	_, err := NewApp([]byte(`not yaml`), Options{})
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestApp_Run(t *testing.T) {
	data := []byte(`
hello:
  script: echo hello
`)
	app, err := NewApp(data, Options{})
	if err != nil {
		t.Fatalf("NewApp() error: %v", err)
	}

	var stdout bytes.Buffer
	app.opts.Stdout = &stdout
	app.opts.Stderr = &stdout

	if err := app.Run(context.Background(), []string{"hello"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Fatalf("expected output 'hello', got: %q", stdout.String())
	}
}

func TestApp_PrintHelp(t *testing.T) {
	data := []byte(`
build:
  desc: Build project
  script: echo build
`)
	app, err := NewApp(data, Options{})
	if err != nil {
		t.Fatalf("NewApp() error: %v", err)
	}

	app.name = "myapp"
	var buf bytes.Buffer
	app.opts.Stdout = &buf
	app.PrintHelp()

	out := buf.String()
	if !strings.Contains(out, "Usage: myapp") {
		t.Errorf("expected usage header, got: %q", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("expected command 'build', got: %q", out)
	}
}

func TestAppFromPath(t *testing.T) {
	yamlContent := `
deploy:
  desc: Deploy app
  native: true
`
	tmpFile := filepath.Join(t.TempDir(), "tasks.yml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := NewAppFromPath(tmpFile, Options{})
	if err != nil {
		t.Fatalf("NewAppFromPath() error: %v", err)
	}
	if app.opts.ConfigDir != filepath.Dir(tmpFile) {
		t.Errorf("expected ConfigDir=%q, got %q", filepath.Dir(tmpFile), app.opts.ConfigDir)
	}
}

func TestAppBuilder(t *testing.T) {
	data := []byte(`
hello:
  desc: Say hello
  native: true
`)

	var called bool
	app, err := NewBuilder("myapp", data).
		RegisterNative("hello", func(ctx context.Context, nctx NativeContext) error {
			called = true
			return nil
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if app.name != "myapp" {
		t.Errorf("expected name=myapp, got %q", app.name)
	}

	if err := app.Run(context.Background(), []string{"hello"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !called {
		t.Fatal("expected native handler to be called")
	}
}

func TestLoadConfigFromPath_ArbitraryName(t *testing.T) {
	yamlContent := `
build:
  desc: Build project
  script: echo build
`
	tmpDir := t.TempDir()

	// Non-standard name: not lota.yml or lota.yaml
	tmpFile := filepath.Join(tmpDir, "my-custom-tasks.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, configDir, err := LoadConfigFromPath(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() error: %v", err)
	}
	if len(cfg.Commands) != 1 || cfg.Commands[0].Name != "build" {
		t.Fatalf("expected command 'build', got %v", cfg.Commands)
	}
	if configDir != tmpDir {
		t.Fatalf("expected configDir %q, got %q", tmpDir, configDir)
	}
}
