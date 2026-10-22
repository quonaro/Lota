package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGetValidator(t *testing.T) {
	cfg := &AppConfig{}
	v := GetValidator(cfg, "/tmp/lota.yml")
	if v.AppConfig != cfg {
		t.Error("validator should wrap the provided config")
	}
	if v.basePath != "/tmp/lota.yml" {
		t.Errorf("basePath = %q, want /tmp/lota.yml", v.basePath)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "build", Script: "go build"}},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", res.Warnings)
	}
}

func TestValidate_NoCommands(t *testing.T) {
	cfg := &AppConfig{}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error == nil {
		t.Fatal("expected error for config without commands")
	}
	if !strings.Contains(res.Error.Error(), "no commands") {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_InvalidWaitValue(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "a", Script: "true"},
			{Name: "b", Script: "true", Depends: []Dependency{{Name: "a", Wait: "bogus"}}},
		},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error == nil {
		t.Fatal("expected error for invalid wait value")
	}
	if !strings.Contains(res.Error.Error(), "invalid wait") {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_DependencyNotFound(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "b", Script: "true", Depends: []Dependency{{Name: "missing"}}},
		},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error == nil {
		t.Fatal("expected error for missing dependency")
	}
	if !strings.Contains(res.Error.Error(), "missing") {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_WaitReadyWithoutReadyBlock(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "db", Script: "sleep 100"},
			{Name: "app", Script: "true", Depends: []Dependency{{Name: "db", Wait: WaitReady}}},
		},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error == nil {
		t.Fatal("expected error for wait:ready without ready block")
	}
	if !strings.Contains(res.Error.Error(), "no ready block") {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_ReadyBlockWithoutScript(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "db", Script: "true", Ready: &ReadyCheck{}},
		},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error == nil {
		t.Fatal("expected error for ready block without script")
	}
	if !strings.Contains(res.Error.Error(), "requires a script") {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_ValidDependencies(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{
			{Name: "db", Script: "true", Ready: &ReadyCheck{Script: "pg_isready"}},
			{Name: "mig", Script: "true"},
			{Name: "app", Script: "true", Depends: []Dependency{
				{Name: "db", Wait: WaitReady},
				{Name: "mig", Wait: WaitDone},
				{Name: "mig", Wait: WaitStarted},
				{Name: "mig"},
			}},
		},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_DependencyInNestedGroup(t *testing.T) {
	cfg := &AppConfig{
		Groups: []Group{
			{
				Name:     "infra",
				Commands: []Command{{Name: "up", Script: "true"}},
				Groups: []Group{
					{
						Name: "app",
						Commands: []Command{
							{Name: "run", Script: "true", Depends: []Dependency{{Name: "infra.up"}}},
						},
					},
				},
			},
		},
	}
	res := GetValidator(cfg, filepath.Join(t.TempDir(), "lota.yml")).Validate()
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
}

func TestValidate_MissingEnvFileIsWarning(t *testing.T) {
	dir := t.TempDir()
	cfg := &AppConfig{
		Commands: []Command{{Name: "build", Script: "true"}},
		Vars:     []Var{{FromFile: "missing.env"}},
	}
	res := GetValidator(cfg, filepath.Join(dir, "lota.yml")).Validate()
	if res.Error != nil {
		t.Fatalf("expected warning, got error: %v", res.Error)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected warning for missing env file")
	}
	if !strings.Contains(res.Warnings[0], "env file not found") {
		t.Fatalf("unexpected warning: %v", res.Warnings[0])
	}
}
