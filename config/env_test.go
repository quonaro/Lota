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

func TestExpandVarsFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "vars.env")
	if err := os.WriteFile(envPath, []byte("# comment\nFOO=bar\nEMPTY=\ninvalid line\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	basePath := filepath.Join(tmpDir, "lota.yml")
	vars := []Var{
		{Name: "INLINE", Value: "x"},
		{FromFile: "vars.env"},
	}

	expanded, err := ExpandVarsFromFile(vars, basePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := map[string]string{}
	for _, v := range expanded {
		got[v.Name] = v.Value
	}
	if got["INLINE"] != "x" || got["FOO"] != "bar" {
		t.Errorf("unexpected expanded vars: %v", got)
	}
	if _, ok := got["EMPTY"]; !ok {
		t.Error("expected EMPTY var to be loaded")
	}
}

func TestExpandVarsFromFile_Missing(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "lota.yml")
	_, err := ExpandVarsFromFile([]Var{{FromFile: "nope.env"}}, basePath)
	if err == nil {
		t.Fatal("expected error for missing env file")
	}
	if !strings.Contains(err.Error(), "env file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandVarsFromFile_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "abs.env")
	if err := os.WriteFile(envPath, []byte("ABS=1\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	expanded, err := ExpandVarsFromFile([]Var{{FromFile: envPath}}, "/elsewhere/lota.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expanded) != 1 || expanded[0].Name != "ABS" || expanded[0].Value != "1" {
		t.Fatalf("unexpected vars: %v", expanded)
	}
}

func TestExpandAllVars(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "app.env"), []byte("APP_VAR=1\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "grp.env"), []byte("GRP_VAR=2\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "cmd.env"), []byte("CMD_VAR=3\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := &AppConfig{
		Vars: []Var{{FromFile: "app.env"}},
		Commands: []Command{
			{Name: "c", Vars: []Var{{FromFile: "cmd.env"}}},
		},
		Groups: []Group{
			{
				Name: "g",
				Vars: []Var{{FromFile: "grp.env"}},
				Groups: []Group{
					{Name: "h", Vars: []Var{{FromFile: "grp.env"}}},
				},
				Commands: []Command{
					{Name: "gc", Vars: []Var{{FromFile: "cmd.env"}}},
				},
			},
		},
	}

	basePath := filepath.Join(tmpDir, "lota.yml")
	if err := ExpandAllVars(cfg, basePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	check := func(vars []Var, name string) bool {
		for _, v := range vars {
			if v.Name == name {
				return true
			}
		}
		return false
	}
	if !check(cfg.Vars, "APP_VAR") {
		t.Error("app-level vars not expanded")
	}
	if !check(cfg.Commands[0].Vars, "CMD_VAR") {
		t.Error("command-level vars not expanded")
	}
	if !check(cfg.Groups[0].Vars, "GRP_VAR") {
		t.Error("group-level vars not expanded")
	}
	if !check(cfg.Groups[0].Groups[0].Vars, "GRP_VAR") {
		t.Error("nested group vars not expanded")
	}
	if !check(cfg.Groups[0].Commands[0].Vars, "CMD_VAR") {
		t.Error("group command vars not expanded")
	}
}
