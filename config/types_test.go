package config

import (
	"strings"
	"testing"
)

func TestBuildIndexesDuplicateCommandAcrossGroupsAllowed(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "deploy"}},
		Groups: []Group{
			{
				Name:     "infra",
				Commands: []Command{{Name: "deploy"}},
			},
			{
				Name:     "app",
				Commands: []Command{{Name: "deploy"}},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Find("deploy").Command == nil {
		t.Fatal("expected top-level deploy command to be found")
	}
	if cfg.groupsMap["infra"].Find("deploy").Command == nil {
		t.Fatal("expected infra.deploy command to be found")
	}
	if cfg.groupsMap["app"].Find("deploy").Command == nil {
		t.Fatal("expected app.deploy command to be found")
	}
}

func TestBuildIndexesDuplicateCommandInSameGroupRejected(t *testing.T) {
	cfg := &AppConfig{
		Groups: []Group{
			{
				Name: "infra",
				Commands: []Command{
					{Name: "up"},
					{Name: "up"},
				},
			},
		},
	}
	err := cfg.BuildIndexes()
	if err == nil {
		t.Fatal("expected error for duplicate command name in the same group")
	}
	if !strings.Contains(err.Error(), "duplicate command name: up") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllVars(t *testing.T) {
	cfg := &AppConfig{
		Vars: []Var{{Name: "APP", Value: "1"}},
		Commands: []Command{
			{Name: "build", Vars: []Var{{Name: "CMD", Value: "2"}}},
		},
		Groups: []Group{
			{
				Name: "g1",
				Vars: []Var{{Name: "G1", Value: "3"}},
				Commands: []Command{
					{Name: "run", Vars: []Var{{Name: "G1CMD", Value: "4"}}},
				},
				Groups: []Group{
					{
						Name: "nested",
						Vars: []Var{{Name: "NESTED", Value: "5"}},
						Commands: []Command{
							{Name: "deep", Vars: []Var{{Name: "DEEP", Value: "6"}}},
						},
					},
				},
			},
		},
	}

	vars := cfg.AllVars()
	got := make(map[string]string, len(vars))
	for _, v := range vars {
		got[v.Name] = v.Value
	}
	want := map[string]string{
		"APP": "1", "CMD": "2", "G1": "3",
		"G1CMD": "4", "NESTED": "5", "DEEP": "6",
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("AllVars missing %s=%s (got %q)", name, value, got[name])
		}
	}
}

func TestCountTotalCommands(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "a"}, {Name: "b"}},
		Groups: []Group{
			{
				Name:     "g",
				Commands: []Command{{Name: "c"}},
				Groups: []Group{
					{Name: "h", Commands: []Command{{Name: "d"}, {Name: "e"}}},
				},
			},
		},
	}
	if got := cfg.countTotalCommands(); got != 5 {
		t.Errorf("countTotalCommands() = %d, want 5", got)
	}
}

func TestGetAllCommandNames(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "top"}},
		Groups: []Group{
			{
				Name:     "g",
				Commands: []Command{{Name: "mid"}},
				Groups: []Group{
					{Name: "h", Commands: []Command{{Name: "deep"}}},
				},
			},
		},
	}
	names := cfg.GetAllCommandNames()
	if len(names) != 3 {
		t.Fatalf("GetAllCommandNames() = %v, want 3 names", names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	for _, want := range []string{"top", "mid", "deep"} {
		if !seen[want] {
			t.Errorf("GetAllCommandNames() missing %q: %v", want, names)
		}
	}
}

func TestFind_NotFound(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "build"}},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes: %v", err)
	}
	res := cfg.Find("missing")
	if res.Exists {
		t.Error("expected Exists=false for missing name")
	}
	if res.Command != nil || len(res.Groups) != 0 {
		t.Error("expected empty result for missing name")
	}
}

func TestGroupFind(t *testing.T) {
	g := &Group{
		Name:     "infra",
		Commands: []Command{{Name: "up"}},
		Groups:   []Group{{Name: "db", Commands: []Command{{Name: "migrate"}}}},
	}
	if err := g.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes: %v", err)
	}

	res := g.Find("up")
	if !res.Exists || res.Command == nil || res.Command.Name != "up" {
		t.Error("expected to find command 'up' in group")
	}

	res = g.Find("db")
	if !res.Exists || len(res.Groups) != 1 || res.Groups[0].Name != "db" {
		t.Error("expected to find sub-group 'db'")
	}

	if res := g.Find("nope"); res.Exists {
		t.Error("expected Exists=false for missing name")
	}
}

func TestBuildIndexesEmptyNames(t *testing.T) {
	cfg := &AppConfig{Commands: []Command{{Name: ""}}}
	if err := cfg.BuildIndexes(); err == nil {
		t.Error("expected error for empty command name")
	}

	cfg = &AppConfig{Groups: []Group{{Name: ""}}}
	if err := cfg.BuildIndexes(); err == nil {
		t.Error("expected error for empty group name")
	}

	cfg = &AppConfig{
		Commands: []Command{{Name: "a"}, {Name: "a"}},
	}
	if err := cfg.BuildIndexes(); err == nil {
		t.Error("expected error for duplicate top-level command")
	}

	cfg = &AppConfig{Groups: []Group{{Name: "g"}, {Name: "g"}}}
	if err := cfg.BuildIndexes(); err == nil {
		t.Error("expected error for duplicate top-level group")
	}
}
