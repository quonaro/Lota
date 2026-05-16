package cli

import (
	"testing"

	"lota/config"
)

func TestBuildCompletion_EmptyConfig(t *testing.T) {
	cfg := &config.AppConfig{}
	comp := BuildCompletion(cfg)

	if len(comp.Sub) != 0 {
		t.Errorf("expected 0 subcommands, got %d", len(comp.Sub))
	}

	expectedFlags := []string{"-h", "--help", "-v", "--verbose", "-V", "--version", "--dry-run", "--init", "--config"}
	for _, f := range expectedFlags {
		if _, ok := comp.Flags[f]; !ok {
			t.Errorf("expected global flag %q", f)
		}
	}
}

func TestBuildCompletion_WithGroupsAndCommands(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "db",
				Commands: []config.Command{
					{Name: "migrate"},
					{Name: "seed"},
				},
			},
		},
		Commands: []config.Command{
			{Name: "hello"},
			{Name: "world"},
		},
	}

	comp := BuildCompletion(cfg)

	if _, ok := comp.Sub["db"]; !ok {
		t.Error("expected 'db' group")
	}
	if _, ok := comp.Sub["hello"]; !ok {
		t.Error("expected 'hello' command")
	}
	if _, ok := comp.Sub["world"]; !ok {
		t.Error("expected 'world' command")
	}

	db := comp.Sub["db"]
	if _, ok := db.Sub["migrate"]; !ok {
		t.Error("expected 'db migrate' command")
	}
	if _, ok := db.Sub["seed"]; !ok {
		t.Error("expected 'db seed' command")
	}
}

func TestBuildCompletion_NestedGroups(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "infra",
				Groups: []config.Group{
					{
						Name: "db",
						Commands: []config.Command{
							{Name: "migrate"},
						},
					},
				},
			},
		},
	}

	comp := BuildCompletion(cfg)

	infra, ok := comp.Sub["infra"]
	if !ok {
		t.Fatal("expected 'infra' group")
	}

	db, ok := infra.Sub["db"]
	if !ok {
		t.Fatal("expected 'infra db' group")
	}

	if _, ok := db.Sub["migrate"]; !ok {
		t.Error("expected 'infra db migrate' command")
	}
}

func TestBuildCompletion_CommandFlags(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{
				Name: "build",
				Args: []config.Arg{
					{Name: "target", Short: "t"},
					{Name: "verbose", Short: "v"},
				},
			},
		},
	}

	comp := BuildCompletion(cfg)
	build := comp.Sub["build"]

	if _, ok := build.Flags["-t"]; !ok {
		t.Error("expected -t flag")
	}
	if _, ok := build.Flags["--target"]; !ok {
		t.Error("expected --target flag")
	}
	if _, ok := build.Flags["-v"]; !ok {
		t.Error("expected -v flag")
	}
	if _, ok := build.Flags["--verbose"]; !ok {
		t.Error("expected --verbose flag")
	}
}

func TestBuildCompletion_GroupFlags(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "deploy",
				Args: []config.Arg{
					{Name: "env", Short: "e"},
				},
				Commands: []config.Command{
					{Name: "prod"},
				},
			},
		},
	}

	comp := BuildCompletion(cfg)
	deploy := comp.Sub["deploy"]

	if _, ok := deploy.Flags["-e"]; !ok {
		t.Error("expected -e flag on group")
	}
	if _, ok := deploy.Flags["--env"]; !ok {
		t.Error("expected --env flag on group")
	}
}

func TestBuildCompletion_ConfigFlagPredictsFiles(t *testing.T) {
	cfg := &config.AppConfig{}
	comp := BuildCompletion(cfg)

	if comp.Flags["--config"] == nil {
		t.Error("expected --config to have a predictor")
	}
}

func TestPrintCompletionScript(t *testing.T) {
	tests := []struct {
		shell     string
		shouldErr bool
	}{
		{"bash", false},
		{"zsh", false},
		{"fish", false},
		{"pwsh", true},
	}

	for _, tt := range tests {
		err := PrintCompletionScript(tt.shell)
		if tt.shouldErr && err == nil {
			t.Errorf("%s: expected error", tt.shell)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("%s: unexpected error: %v", tt.shell, err)
		}
	}
}
