package config

import (
	"testing"
)

func TestFindCommandByPath(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "build", Script: "go build"},
		},
		Groups: []Group{
			{
				Name: "infra",
				Groups: []Group{
					{
						Name:     "docker",
						Commands: []Command{{Name: "up", Script: "docker up"}},
					},
				},
				Commands: []Command{{Name: "deploy", Script: "deploy"}},
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
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "build", Script: "go build"},
			{Name: "lint", Script: "go vet", Depends: []Dependency{{Name: "build"}}},
			{Name: "test", Script: "go test", Depends: []Dependency{{Name: "build"}, {Name: "lint"}}},
		},
		Groups: []Group{
			{
				Name: "backend",
				Commands: []Command{
					{Name: "run", Script: "go run", Depends: []Dependency{{Name: "build"}}},
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
		badCfg := &AppConfig{
			Commands: []Command{
				{Name: "a", Script: "echo a", Depends: []Dependency{{Name: "b"}}},
				{Name: "b", Script: "echo b", Depends: []Dependency{{Name: "a"}}},
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

	t.Run("shared dep keeps strictest wait", func(t *testing.T) {
		wcfg := &AppConfig{
			Commands: []Command{
				{Name: "svc", Script: "sleep 1", Ready: &ReadyCheck{Script: "true"}},
				{Name: "relaxed", Script: "echo r", Depends: []Dependency{{Name: "svc", Wait: "started"}}},
				{Name: "strict", Script: "echo s", Depends: []Dependency{{Name: "svc", Wait: "ready"}}},
				{Name: "top", Script: "echo top", Depends: []Dependency{{Name: "relaxed"}, {Name: "strict"}}},
			},
		}
		if err := wcfg.BuildIndexes(); err != nil {
			t.Fatalf("BuildIndexes() error: %v", err)
		}
		result, _ := FindCommandByPath(wcfg, "top")
		deps, err := ResolveDependencies(wcfg, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, dep := range deps {
			if dep.Command.Name == "svc" && dep.Wait != WaitReady {
				t.Errorf("svc wait = %q, want %q", dep.Wait, WaitReady)
			}
		}
	})
}

func TestResolveCommand(t *testing.T) {
	newCfg := func() *AppConfig {
		cfg := &AppConfig{
			Commands: []Command{{Name: "build"}},
			Groups: []Group{
				{
					Name:     "infra",
					Commands: []Command{{Name: "up"}},
					Groups: []Group{
						{Name: "db", Commands: []Command{{Name: "migrate"}}},
					},
				},
			},
		}
		if err := cfg.BuildIndexes(); err != nil {
			t.Fatalf("BuildIndexes: %v", err)
		}
		return cfg
	}

	t.Run("empty args", func(t *testing.T) {
		res, rest, idx := ResolveCommand(newCfg(), nil)
		if res.Exists || len(rest) != 0 || idx != 0 {
			t.Errorf("got exists=%v rest=%v idx=%d", res.Exists, rest, idx)
		}
	})

	t.Run("top-level command", func(t *testing.T) {
		res, rest, _ := ResolveCommand(newCfg(), []string{"build"})
		if !res.Exists || res.Command == nil || res.Command.Name != "build" {
			t.Fatal("expected to resolve 'build'")
		}
		if len(rest) != 0 {
			t.Errorf("expected no remaining args, got %v", rest)
		}
	})

	t.Run("command with extra args", func(t *testing.T) {
		res, rest, _ := ResolveCommand(newCfg(), []string{"build", "-v", "extra"})
		if res.Command == nil || res.Command.Name != "build" {
			t.Fatal("expected to resolve 'build'")
		}
		if len(rest) != 2 || rest[0] != "-v" || rest[1] != "extra" {
			t.Errorf("unexpected remaining args: %v", rest)
		}
	})

	t.Run("nested group command", func(t *testing.T) {
		res, rest, _ := ResolveCommand(newCfg(), []string{"infra", "db", "migrate", "--force"})
		if res.Command == nil || res.Command.Name != "migrate" {
			t.Fatal("expected to resolve infra.db.migrate")
		}
		if len(res.Groups) != 2 || res.Groups[0].Name != "infra" || res.Groups[1].Name != "db" {
			t.Errorf("unexpected groups chain: %v", res.Groups)
		}
		if len(rest) != 1 || rest[0] != "--force" {
			t.Errorf("unexpected remaining args: %v", rest)
		}
	})

	t.Run("group only", func(t *testing.T) {
		res, _, _ := ResolveCommand(newCfg(), []string{"infra"})
		if !res.Exists || res.Command != nil {
			t.Fatal("expected group result without command")
		}
		if len(res.Groups) != 1 || res.Groups[0].Name != "infra" {
			t.Errorf("unexpected groups: %v", res.Groups)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		res, rest, _ := ResolveCommand(newCfg(), []string{"nope", "x"})
		if res.Exists {
			t.Error("expected Exists=false")
		}
		if len(rest) != 2 {
			t.Errorf("expected all args returned, got %v", rest)
		}
	})

	t.Run("flags skipped during resolution", func(t *testing.T) {
		res, rest, _ := ResolveCommand(newCfg(), []string{"infra", "--flag", "value", "up"})
		if res.Command == nil || res.Command.Name != "up" {
			t.Fatalf("expected to resolve 'up', got %+v", res)
		}
		if len(rest) != 0 {
			t.Errorf("unexpected remaining args: %v", rest)
		}
	})
}

func TestFindCommandByPath_Errors(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "build"}},
		Groups:   []Group{{Name: "infra", Commands: []Command{{Name: "up"}}}},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes: %v", err)
	}

	if _, err := FindCommandByPath(cfg, "missing"); err == nil {
		t.Error("expected error for unknown command")
	}
	if _, err := FindCommandByPath(cfg, "infra.missing"); err == nil {
		t.Error("expected error for unknown nested command")
	}
	if _, err := FindCommandByPath(cfg, "build.sub"); err == nil {
		t.Error("expected error traversing into a command")
	}
	if _, err := FindCommandByPath(cfg, "infra"); err == nil {
		t.Error("expected error: group path is not a command")
	}
}

func TestCommandPath(t *testing.T) {
	cmd := &Command{Name: "migrate"}
	groups := []*Group{{Name: "infra"}, {Name: "db"}}
	if got := CommandPath(cmd, groups); got != "infra.db.migrate" {
		t.Errorf("CommandPath = %q, want infra.db.migrate", got)
	}
	if got := CommandPath(cmd, nil); got != "migrate" {
		t.Errorf("CommandPath = %q, want migrate", got)
	}
}
