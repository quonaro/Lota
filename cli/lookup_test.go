package cli

import (
	"lota/config"
	"testing"
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
