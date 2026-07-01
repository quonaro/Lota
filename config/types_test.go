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
