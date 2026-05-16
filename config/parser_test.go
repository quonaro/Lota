package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func intPtr(i int) *int {
	return &i
}

func TestArg_Parse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Arg
	}{
		{
			name:  "simple name",
			input: "param1",
			expected: Arg{
				Name: "param1",
			},
		},
		{
			name:  "with short",
			input: "verbose|v",
			expected: Arg{
				Name:  "verbose",
				Short: "v",
			},
		},
		{
			name:  "with type",
			input: "count:int",
			expected: Arg{
				Name: "count",
				Type: "int",
			},
		},
		{
			name:  "with short and type",
			input: "output|o:str",
			expected: Arg{
				Name:  "output",
				Short: "o",
				Type:  "str",
			},
		},
		{
			name:  "with default value",
			input: "port:int=8080",
			expected: Arg{
				Name:    "port",
				Type:    "int",
				Default: "8080",
			},
		},
		{
			name:  "with short, type and default",
			input: "output|o:str=./bin",
			expected: Arg{
				Name:    "output",
				Short:   "o",
				Type:    "str",
				Default: "./bin",
			},
		},
		{
			name:  "wildcard",
			input: "...args",
			expected: Arg{
				Name:     "args",
				Wildcard: true,
			},
		},
		{
			name:  "array without limit",
			input: "files:arr",
			expected: Arg{
				Name: "files",
				Type: "arr",
			},
		},
		{
			name:  "array with limit",
			input: "files:arr[5]",
			expected: Arg{
				Name:   "files",
				Type:   "arr",
				MaxArr: intPtr(5),
			},
		},
		{
			name:  "bool type with default true",
			input: "verbose|v:bool=true",
			expected: Arg{
				Name:    "verbose",
				Short:   "v",
				Type:    "bool",
				Default: "true",
			},
		},
		{
			name:  "bool type with default false",
			input: "debug|d:bool=false",
			expected: Arg{
				Name:    "debug",
				Short:   "d",
				Type:    "bool",
				Default: "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Arg
			if err := a.Parse(tt.input); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if a.Name != tt.expected.Name {
				t.Errorf("Name = %v, want %v", a.Name, tt.expected.Name)
			}
			if a.Short != tt.expected.Short {
				t.Errorf("Short = %v, want %v", a.Short, tt.expected.Short)
			}
			if a.Type != tt.expected.Type {
				t.Errorf("Type = %v, want %v", a.Type, tt.expected.Type)
			}
			if a.Default != tt.expected.Default {
				t.Errorf("Default = %v, want %v", a.Default, tt.expected.Default)
			}
			if a.Wildcard != tt.expected.Wildcard {
				t.Errorf("Wildcard = %v, want %v", a.Wildcard, tt.expected.Wildcard)
			}
			if (a.MaxArr == nil) != (tt.expected.MaxArr == nil) {
				t.Errorf("MaxArr nil mismatch")
			} else if a.MaxArr != nil && tt.expected.MaxArr != nil && *a.MaxArr != *tt.expected.MaxArr {
				t.Errorf("MaxArr = %v, want %v", *a.MaxArr, *tt.expected.MaxArr)
			}
		})
	}
}

func TestVar_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Var
	}{
		{
			name:  "simple name without value",
			input: "VAR_NAME",
			expected: Var{
				Name:  "VAR_NAME",
				Value: "",
			},
		},
		{
			name:  "name with value",
			input: "KEY=value",
			expected: Var{
				Name:  "KEY",
				Value: "value",
			},
		},
		{
			name:  "name with value containing equals",
			input: "DB_URL=postgres://user:pass@host/db",
			expected: Var{
				Name:  "DB_URL",
				Value: "postgres://user:pass@host/db",
			},
		},
		{
			name:  "empty value",
			input: "EMPTY=",
			expected: Var{
				Name:  "EMPTY",
				Value: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			node.Kind = yaml.ScalarNode
			node.Value = tt.input

			var v Var
			if err := v.UnmarshalYAML(&node); err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}
			if v.Name != tt.expected.Name {
				t.Errorf("Name = %v, want %v", v.Name, tt.expected.Name)
			}
			if v.Value != tt.expected.Value {
				t.Errorf("Value = %v, want %v", v.Value, tt.expected.Value)
			}
		})
	}
}

func TestVar_UnmarshalYAML_NonScalar(t *testing.T) {
	var node yaml.Node
	node.Kind = yaml.MappingNode

	var v Var
	if err := v.UnmarshalYAML(&node); err == nil {
		t.Error("Expected error for non-scalar node, got nil")
	}
}

func TestHasField(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
nested:
  child: value
`
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &node); err != nil {
		t.Fatalf("Failed to unmarshal yaml: %v", err)
	}

	root := node.Content[0]

	tests := []struct {
		field    string
		expected bool
	}{
		{"key1", true},
		{"key2", true},
		{"nested", true},
		{"missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := hasField(root, tt.field)
			if result != tt.expected {
				t.Errorf("hasField(%q) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestHasField_NonMapping(t *testing.T) {
	var node yaml.Node
	node.Kind = yaml.SequenceNode
	node.Content = []*yaml.Node{{Value: "item"}}

	if hasField(&node, "key") {
		t.Error("hasField should return false for non-mapping node")
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
		check       func(*testing.T, *AppConfig)
	}{
		{
			name: "config with global vars",
			yamlContent: `vars:
  - ENV=production
  - DEBUG=false
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Vars) != 2 {
					t.Errorf("Expected 2 vars, got %d", len(cfg.Vars))
					return
				}
				if cfg.Vars[0].Name != "ENV" || cfg.Vars[0].Value != "production" {
					t.Errorf("First var = %v, want ENV=production", cfg.Vars[0])
				}
			},
		},
		{
			name: "config with top-level command",
			yamlContent: `build:
  desc: Build the application
  script: go build -o bin/app
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Commands) != 1 {
					t.Errorf("Expected 1 command, got %d", len(cfg.Commands))
					return
				}
				cmd := cfg.Commands[0]
				if cmd.Name != "build" {
					t.Errorf("Command name = %v, want build", cmd.Name)
				}
				if cmd.Script != "go build -o bin/app" {
					t.Errorf("Command script = %v, want 'go build -o bin/app'", cmd.Script)
				}
			},
		},
		{
			name: "config with group",
			yamlContent: `dev:
  desc: Development commands
  run:
    desc: Run the app
    script: go run .
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Groups) != 1 {
					t.Errorf("Expected 1 group, got %d", len(cfg.Groups))
					return
				}
				group := cfg.Groups[0]
				if group.Name != "dev" {
					t.Errorf("Group name = %v, want dev", group.Name)
				}
				if len(group.Commands) != 1 {
					t.Errorf("Expected 1 command in group, got %d", len(group.Commands))
				}
			},
		},
		{
			name: "command with args",
			yamlContent: `deploy:
  desc: Deploy app
  args:
    - env:str=production
    - force|f:bool=false
  script: deploy {{env}} {{force}}
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Commands) != 1 {
					t.Errorf("Expected 1 command, got %d", len(cfg.Commands))
					return
				}
				cmd := cfg.Commands[0]
				if len(cmd.Args) != 2 {
					t.Errorf("Expected 2 args, got %d", len(cmd.Args))
					return
				}
				if cmd.Args[0].Name != "env" || cmd.Args[0].Type != "str" || cmd.Args[0].Default != "production" {
					t.Errorf("First arg = %v, want env:str=production", cmd.Args[0])
				}
				if cmd.Args[1].Name != "force" || cmd.Args[1].Short != "f" || cmd.Args[1].Type != "bool" {
					t.Errorf("Second arg = %v, want force|f:bool", cmd.Args[1])
				}
			},
		},
		{
			name: "command with before and after hooks",
			yamlContent: `test:
  before: echo "Starting tests"
  script: go test ./...
  after: echo "Tests complete"
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				cmd := cfg.Commands[0]
				if cmd.Before != "echo \"Starting tests\"" {
					t.Errorf("Before = %v, want 'echo \"Starting tests\"'", cmd.Before)
				}
				if cmd.After != "echo \"Tests complete\"" {
					t.Errorf("After = %v, want 'echo \"Tests complete\"'", cmd.After)
				}
			},
		},
		{
			name: "command with dir",
			yamlContent: `build:
  desc: Build backend
  dir: ./backend
  script: go build .
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Commands) != 1 {
					t.Errorf("Expected 1 command, got %d", len(cfg.Commands))
					return
				}
				cmd := cfg.Commands[0]
				if cmd.Dir != "./backend" {
					t.Errorf("Command dir = %v, want ./backend", cmd.Dir)
				}
			},
		},
		{
			name: "group with dir",
			yamlContent: `backend:
  desc: Backend commands
  dir: ./backend
  build:
    desc: Build
    script: go build .
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Groups) != 1 {
					t.Errorf("Expected 1 group, got %d", len(cfg.Groups))
					return
				}
				group := cfg.Groups[0]
				if group.Dir != "./backend" {
					t.Errorf("Group dir = %v, want ./backend", group.Dir)
				}
			},
		},
		{
			name: "command with depends",
			yamlContent: `test:
  desc: Run tests
  depends:
    - build
    - lint
  script: go test ./...
`,
			wantErr: false,
			check: func(t *testing.T, cfg *AppConfig) {
				if len(cfg.Commands) != 1 {
					t.Errorf("Expected 1 command, got %d", len(cfg.Commands))
					return
				}
				cmd := cfg.Commands[0]
				if len(cmd.Depends) != 2 {
					t.Errorf("Expected 2 depends, got %d", len(cmd.Depends))
				}
				if cmd.Depends[0] != "build" || cmd.Depends[1] != "lint" {
					t.Errorf("Depends = %v, want [build, lint]", cmd.Depends)
				}
			},
		},
		{
			name:        "empty config",
			yamlContent: ``,
			wantErr:     true,
			check:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "test.yml")
			if err := os.WriteFile(tmpFile, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, err := ParseConfig(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestParseConfig_FileNotFound(t *testing.T) {
	_, err := ParseConfig("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestParseConfig_InvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.yml")
	if err := os.WriteFile(tmpFile, []byte("not: valid: yaml: ["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseConfig(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestParseConfig_WithGroupArgs(t *testing.T) {
	yamlContent := `
args:
- environment|env:str="dev"
- verbose|v:bool=false

group1:
  desc: Test group with args
  args:
  - environment|env:str="prod"
  - timeout:int=30
  command1:
    desc: Test command
    args:
    - param1:str
    script: echo "{{environment}} {{param1}} {{timeout}}"
`

	tmpFile := filepath.Join(t.TempDir(), "test_group_args_config.yml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	config, err := ParseConfig(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if len(config.Args) != 2 {
		t.Errorf("Expected 2 app level args, got %d", len(config.Args))
	}

	appEnvFound := false
	appVerboseFound := false
	for _, arg := range config.Args {
		if arg.Name == "environment" && arg.Default == `"dev"` {
			appEnvFound = true
		}
		if arg.Name == "verbose" && arg.Default == "false" {
			appVerboseFound = true
		}
	}
	if !appEnvFound {
		t.Error("Expected environment arg with default 'dev' at app level")
	}
	if !appVerboseFound {
		t.Error("Expected verbose arg with default 'false' at app level")
	}

	if len(config.Groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(config.Groups))
	}

	group := config.Groups[0]
	if len(group.Args) != 2 {
		t.Errorf("Expected 2 group level args, got %d", len(group.Args))
	}

	groupEnvFound := false
	groupTimeoutFound := false
	for _, arg := range group.Args {
		if arg.Name == "environment" && arg.Default == `"prod"` {
			groupEnvFound = true
		}
		if arg.Name == "timeout" && arg.Default == "30" {
			groupTimeoutFound = true
		}
	}
	if !groupEnvFound {
		t.Error("Expected environment arg with default 'prod' at group level")
	}
	if !groupTimeoutFound {
		t.Error("Expected timeout arg with default '30' at group level")
	}
}

func TestParseConfig_WithAppLevelCommandsWithArgs(t *testing.T) {
	yamlContent := `
args:
- global_flag|g:bool=false

top-level-command:
  desc: Command at app level
  args:
  - local_param:str
  script: echo "{{global_flag}} {{local_param}}"
`

	tmpFile := filepath.Join(t.TempDir(), "test_app_level_args_config.yml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	config, err := ParseConfig(tmpFile)
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if len(config.Args) != 1 {
		t.Errorf("Expected 1 app level arg, got %d", len(config.Args))
	}

	if config.Args[0].Name != "global_flag" || config.Args[0].Default != "false" {
		t.Error("Expected global_flag arg with default 'false' at app level")
	}

	if len(config.Commands) != 1 {
		t.Fatalf("Expected 1 top-level command, got %d", len(config.Commands))
	}

	command := config.Commands[0]
	if len(command.Args) != 1 {
		t.Errorf("Expected 1 command level arg, got %d", len(command.Args))
	}

	if command.Args[0].Name != "local_param" {
		t.Errorf("Expected local_param arg, got %s", command.Args[0].Name)
	}
}
