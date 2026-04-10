package cli

import (
	"lota/config"
	"reflect"
	"testing"
)


func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name          string
		input         []string
		expectedFlags GlobalFlags
		expectedArgs  []string
		wantErr       bool
	}{
		{
			name:          "no flags",
			input:         []string{"command"},
			expectedFlags: GlobalFlags{},
			expectedArgs:  []string{"command"},
		},
		{
			name:          "verbose long flag",
			input:         []string{"--verbose", "command"},
			expectedFlags: GlobalFlags{Verbose: true},
			expectedArgs:  []string{"command"},
		},
		{
			name:          "verbose short flag",
			input:         []string{"-v", "command"},
			expectedFlags: GlobalFlags{Verbose: true},
			expectedArgs:  []string{"command"},
		},
		{
			name:          "help flag",
			input:         []string{"--help"},
			expectedFlags: GlobalFlags{Help: true},
			expectedArgs:  []string{},
		},
		{
			name:          "help short flag",
			input:         []string{"-h"},
			expectedFlags: GlobalFlags{Help: true},
			expectedArgs:  []string{},
		},
		{
			name:          "version flag",
			input:         []string{"-V"},
			expectedFlags: GlobalFlags{Version: true},
			expectedArgs:  []string{},
		},
		{
			name:          "dry-run flag",
			input:         []string{"--dry-run", "command"},
			expectedFlags: GlobalFlags{DryRun: true},
			expectedArgs:  []string{"command"},
		},
		{
			name:          "init flag",
			input:         []string{"--init"},
			expectedFlags: GlobalFlags{Init: true},
			expectedArgs:  []string{},
		},
		{
			name:          "config flag",
			input:         []string{"--config", "/path/to/config.yml", "command"},
			expectedFlags: GlobalFlags{Config: "/path/to/config.yml"},
			expectedArgs:  []string{"command"},
		},
		{
			name:          "unknown flag stops parsing",
			input:         []string{"--unknown", "--verbose"},
			expectedFlags: GlobalFlags{},
			expectedArgs:  []string{"--unknown", "--verbose"},
		},
		{
			name:          "multiple flags",
			input:         []string{"--verbose", "--dry-run", "command"},
			expectedFlags: GlobalFlags{Verbose: true, DryRun: true},
			expectedArgs:  []string{"command"},
		},
		{
			name:    "config flag without value",
			input:   []string{"--config"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, remaining, err := ParseGlobalFlags(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if flags != tt.expectedFlags {
				t.Errorf("flags = %+v, want %+v", flags, tt.expectedFlags)
			}
			if !reflect.DeepEqual(remaining, tt.expectedArgs) {
				t.Errorf("remaining = %v, want %v", remaining, tt.expectedArgs)
			}
		})
	}
}

func TestResolveCommand(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "build", Script: "go build"},
			{Name: "test", Script: "go test ./..."},
		},
		Groups: []config.Group{
			{
				Name: "dev",
				Commands: []config.Command{
					{Name: "run", Script: "go run ."},
					{Name: "watch", Script: "air"},
				},
				Groups: []config.Group{
					{
						Name: "docker",
						Commands: []config.Command{
							{Name: "up", Script: "docker-compose up"},
						},
						Groups: []config.Group{
							{
								Name: "logs",
								Commands: []config.Command{
									{Name: "tail", Script: "docker-compose logs -f"},
								},
							},
						},
					},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		expectExists   bool
		expectCommand  bool
		expectGroups   int
		expectRemain   []string
	}{
		{
			name:         "empty input",
			args:         []string{},
			expectExists: false,
		},
		{
			name:          "top-level command",
			args:          []string{"build"},
			expectExists:  true,
			expectCommand: true,
			expectRemain:  []string{},
		},
		{
			name:          "top-level command with flag args",
			args:          []string{"build", "--verbose"},
			expectExists:  true,
			expectCommand: true,
			expectRemain:  []string{"--verbose"},
		},
		{
			name:         "non-existing command",
			args:         []string{"nonexistent"},
			expectExists: false,
		},
		{
			name:         "existing group",
			args:         []string{"dev"},
			expectExists: true,
			expectGroups: 1,
			expectRemain: []string{},
		},
		{
			name:          "command inside group",
			args:          []string{"dev", "run"},
			expectExists:  true,
			expectCommand: true,
			expectGroups:  1,
			expectRemain:  []string{},
		},
		{
			name:          "command inside group with positional args",
			args:          []string{"dev", "run", "myarg", "--port", "8080"},
			expectExists:  true,
			expectCommand: true,
			expectGroups:  1,
			expectRemain:  []string{"myarg", "--port", "8080"},
		},
		{
			name:         "nested group",
			args:         []string{"dev", "docker"},
			expectExists: true,
			expectGroups: 2,
			expectRemain: []string{},
		},
		{
			name:          "command in nested group",
			args:          []string{"dev", "docker", "up"},
			expectExists:  true,
			expectCommand: true,
			expectGroups:  2,
			expectRemain:  []string{},
		},
		{
			name:          "deeply nested command (3 levels)",
			args:          []string{"dev", "docker", "logs", "tail"},
			expectExists:  true,
			expectCommand: true,
			expectGroups:  3,
			expectRemain:  []string{},
		},
		{
			name:         "deeply nested group",
			args:         []string{"dev", "docker", "logs"},
			expectExists: true,
			expectGroups: 3,
			expectRemain: []string{},
		},
		{
			name:          "deeply nested command with remaining args",
			args:          []string{"dev", "docker", "logs", "tail", "--follow"},
			expectExists:  true,
			expectCommand: true,
			expectGroups:  3,
			expectRemain:  []string{"--follow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, remain := ResolveCommand(cfg, tt.args)

			if result.Exists != tt.expectExists {
				t.Errorf("Exists = %v, want %v", result.Exists, tt.expectExists)
			}
			if tt.expectCommand && result.Command == nil {
				t.Error("expected Command to be non-nil")
			}
			if !tt.expectCommand && result.Command != nil {
				t.Errorf("expected Command to be nil, got %v", result.Command.Name)
			}
			if tt.expectGroups > 0 && len(result.Groups) != tt.expectGroups {
				t.Errorf("Groups count = %v, want %v", len(result.Groups), tt.expectGroups)
			}
			if tt.expectRemain != nil && !reflect.DeepEqual(remain, tt.expectRemain) {
				t.Errorf("remaining = %v, want %v", remain, tt.expectRemain)
			}
		})
	}
}

