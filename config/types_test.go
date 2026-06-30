package config

import "testing"

func TestBuildIndexesDuplicateCommandAcrossGroups(t *testing.T) {
	cfg := &AppConfig{
		Commands: []Command{{Name: "deploy"}},
		Groups: []Group{
			{
				Name:     "infra",
				Commands: []Command{{Name: "deploy"}},
			},
		},
	}
	err := cfg.BuildIndexes()
	if err == nil {
		t.Fatal("expected error for duplicate command name across groups")
	}
	if err.Error() != `duplicate command name "deploy" at paths deploy, infra.deploy` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildIndexesDuplicateCommandInNestedGroups(t *testing.T) {
	cfg := &AppConfig{
		Groups: []Group{
			{
				Name: "infra",
				Groups: []Group{
					{
						Name:     "docker",
						Commands: []Command{{Name: "up"}},
					},
				},
				Commands: []Command{{Name: "up"}},
			},
		},
	}
	err := cfg.BuildIndexes()
	if err == nil {
		t.Fatal("expected error for duplicate command name in nested groups")
	}
	if err.Error() != `duplicate command name "up" at paths infra.up, infra.docker.up` {
		t.Fatalf("unexpected error: %v", err)
	}
}
