package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadYAMLFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		content      string
		path         string // optional, for @section tests
		prefix       string
		expectedVars map[string]string
		wantErr      bool
	}{
		{
			name: "simple flat yaml",
			content: `
app_name: MyApp
version: 1.0.0
`,
			prefix: "",
			expectedVars: map[string]string{
				"app_name": "MyApp",
				"version":  "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "nested yaml without prefix",
			content: `
app:
  name: MyApp
  version: 1.0.0
database:
  host: localhost
  port: 5432
`,
			prefix: "",
			expectedVars: map[string]string{
				"app.name":      "MyApp",
				"app.version":   "1.0.0",
				"database.host": "localhost",
				"database.port": "5432",
			},
			wantErr: false,
		},
		{
			name: "nested yaml with prefix",
			content: `
app_name: MyApp
version: 1.0.0
database:
  host: localhost
  port: 5432
`,
			prefix: "public",
			expectedVars: map[string]string{
				"public.app_name":      "MyApp",
				"public.version":       "1.0.0",
				"public.database.host": "localhost",
				"public.database.port": "5432",
			},
			wantErr: false,
		},
		{
			name: "yaml with arrays",
			content: `
items:
  - first
  - second
  - third
`,
			prefix: "",
			expectedVars: map[string]string{
				"items.0": "first",
				"items.1": "second",
				"items.2": "third",
			},
			wantErr: false,
		},
		{
			name:    "select section with @",
			path:    "config.yaml@section",
			content: "",
			prefix:  "cfg",
			expectedVars: map[string]string{
				"cfg.app_name": "YourTask",
				"cfg.debug":    "true",
			},
			wantErr: false,
		},
		{
			name:    "file not found",
			content: ``,
			prefix:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.name != "file not found" {
				content := tt.content
				// Special content for section test
				if tt.name == "select section with @" {
					content = `
section:
  app_name: YourTask
  debug: true
other:
  ignored: value
`
				}

				// For section test, create file without @section part
				if tt.path != "" {
					// Strip @section for file creation, but use full path for loading
					writePath := tt.path
					if idx := strings.LastIndex(tt.path, "@"); idx != -1 {
						writePath = tt.path[:idx]
					}
					filePath = filepath.Join(tempDir, tt.path)
					dir := filepath.Dir(filePath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(tempDir, writePath), []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				} else {
					filePath = filepath.Join(tempDir, tt.name+".yaml")
					if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				filePath = filepath.Join(tempDir, "nonexistent.yaml")
			}

			vars, err := loadYAMLFile(tempDir, filePath, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadYAMLFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(vars) != len(tt.expectedVars) {
				t.Errorf("Expected %d vars, got %d: %v", len(tt.expectedVars), len(vars), vars)
				return
			}

			for key, expectedVal := range tt.expectedVars {
				if vars[key] != expectedVal {
					t.Errorf("Expected %s = %q, got %q", key, expectedVal, vars[key])
				}
			}
		})
	}
}

func TestVar_UnmarshalYAML_Import(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		value    string
		expected Var
	}{
		{
			name:  "import env without prefix",
			tag:   "!import:env",
			value: ".env",
			expected: Var{
				IsFile:   true,
				Format:   "env",
				FromFile: ".env",
				Prefix:   "",
			},
		},
		{
			name:  "import yaml with prefix",
			tag:   "!import:yaml",
			value: "env.yaml public",
			expected: Var{
				IsFile:   true,
				Format:   "yaml",
				FromFile: "env.yaml",
				Prefix:   "public",
			},
		},
		{
			name:  "import yml with prefix",
			tag:   "!import:yml",
			value: "config.yml app",
			expected: Var{
				IsFile:   true,
				Format:   "yml",
				FromFile: "config.yml",
				Prefix:   "app",
			},
		},
		{
			name:  "import yaml without prefix (multiple spaces)",
			tag:   "!import:yaml",
			value: "  env.yaml  ",
			expected: Var{
				IsFile:   true,
				Format:   "yaml",
				FromFile: "env.yaml",
				Prefix:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			node.Kind = yaml.ScalarNode
			node.Tag = tt.tag
			node.Value = tt.value

			var v Var
			if err := v.UnmarshalYAML(&node); err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}

			if v.IsFile != tt.expected.IsFile {
				t.Errorf("IsFile = %v, want %v", v.IsFile, tt.expected.IsFile)
			}
			if v.Format != tt.expected.Format {
				t.Errorf("Format = %v, want %v", v.Format, tt.expected.Format)
			}
			if v.FromFile != tt.expected.FromFile {
				t.Errorf("FromFile = %v, want %v", v.FromFile, tt.expected.FromFile)
			}
			if v.Prefix != tt.expected.Prefix {
				t.Errorf("Prefix = %v, want %v", v.Prefix, tt.expected.Prefix)
			}
		})
	}
}

func TestExpandVarsFromFile_YAML(t *testing.T) {
	tempDir := t.TempDir()

	// Create test YAML file
	yamlContent := `
app_name: MyApp
database:
  host: localhost
  port: 5432
`
	yamlPath := filepath.Join(tempDir, "env.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	vars := []Var{
		{
			IsFile:   true,
			Format:   "yaml",
			FromFile: yamlPath,
			Prefix:   "public",
		},
	}

	result, err := ExpandVarsFromFile(vars, filepath.Join(tempDir, "lota.yml"))
	if err != nil {
		t.Fatalf("ExpandVarsFromFile() error = %v", err)
	}

	expected := map[string]string{
		"public.app_name":      "MyApp",
		"public.database.host": "localhost",
		"public.database.port": "5432",
	}

	if len(result) != len(expected) {
		t.Errorf("Expected %d vars, got %d: %v", len(expected), len(result), result)
		return
	}

	resultMap := make(map[string]string)
	for _, v := range result {
		resultMap[v.Name] = v.Value
	}

	for key, val := range expected {
		if resultMap[key] != val {
			t.Errorf("Expected %s = %q, got %q", key, val, resultMap[key])
		}
	}
}
