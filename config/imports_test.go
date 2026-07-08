package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeGroups(t *testing.T) {
	tests := []struct {
		name     string
		local    []Group
		imported []Group
		expected []Group
	}{
		{
			name:     "no groups",
			local:    []Group{},
			imported: []Group{},
			expected: []Group{},
		},
		{
			name:     "local only",
			local:    []Group{{Name: "local1"}, {Name: "local2"}},
			imported: []Group{},
			expected: []Group{{Name: "local1"}, {Name: "local2"}},
		},
		{
			name:     "imported only",
			local:    []Group{},
			imported: []Group{{Name: "imported1"}, {Name: "imported2"}},
			expected: []Group{{Name: "imported1"}, {Name: "imported2"}},
		},
		{
			name:     "merge without conflicts",
			local:    []Group{{Name: "local1"}},
			imported: []Group{{Name: "imported1"}},
			expected: []Group{{Name: "local1"}, {Name: "imported1"}},
		},
		{
			name:     "merge with conflict - local wins",
			local:    []Group{{Name: "shared", Desc: "local"}},
			imported: []Group{{Name: "shared", Desc: "imported"}},
			expected: []Group{{Name: "shared", Desc: "local"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeGroups(tt.local, tt.imported)
			// Check that all expected groups are present
			resultMap := make(map[string]Group)
			for _, g := range result {
				resultMap[g.Name] = g
			}
			for _, expected := range tt.expected {
				if actual, exists := resultMap[expected.Name]; !exists {
					t.Errorf("expected group %q not found", expected.Name)
				} else if actual.Desc != expected.Desc {
					t.Errorf("group %q desc = %q, want %q", expected.Name, actual.Desc, expected.Desc)
				}
			}
		})
	}
}

func TestMergeCommands(t *testing.T) {
	tests := []struct {
		name     string
		local    []Command
		imported []Command
		expected []Command
	}{
		{
			name:     "no commands",
			local:    []Command{},
			imported: []Command{},
			expected: []Command{},
		},
		{
			name:     "local only",
			local:    []Command{{Name: "local1"}, {Name: "local2"}},
			imported: []Command{},
			expected: []Command{{Name: "local1"}, {Name: "local2"}},
		},
		{
			name:     "imported only",
			local:    []Command{},
			imported: []Command{{Name: "imported1"}, {Name: "imported2"}},
			expected: []Command{{Name: "imported1"}, {Name: "imported2"}},
		},
		{
			name:     "merge without conflicts",
			local:    []Command{{Name: "local1"}},
			imported: []Command{{Name: "imported1"}},
			expected: []Command{{Name: "local1"}, {Name: "imported1"}},
		},
		{
			name:     "merge with conflict - local wins",
			local:    []Command{{Name: "shared", Script: "local"}},
			imported: []Command{{Name: "shared", Script: "imported"}},
			expected: []Command{{Name: "shared", Script: "local"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeCommands(tt.local, tt.imported)
			// Check that all expected commands are present
			resultMap := make(map[string]Command)
			for _, c := range result {
				resultMap[c.Name] = c
			}
			for _, expected := range tt.expected {
				if actual, exists := resultMap[expected.Name]; !exists {
					t.Errorf("expected command %q not found", expected.Name)
				} else if actual.Script != expected.Script {
					t.Errorf("command %q script = %q, want %q", expected.Name, actual.Script, expected.Script)
				}
			}
		})
	}
}

func TestProcessImports(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create an imported config file
	importedConfig := `
vars:
- IMPORTED_VAR=imported_value

imported_group:
  desc: Imported group
  imported_cmd:
    desc: Imported command
    script: echo "imported"
`
	importedPath := filepath.Join(tmpDir, "imported.yml")
	if err := os.WriteFile(importedPath, []byte(importedConfig), 0644); err != nil {
		t.Fatalf("failed to write imported config: %v", err)
	}

	// Create a main config with import
	mainConfig := `
imports:
- url: imported.yml

local_group:
  desc: Local group
  script: echo "local"
`
	mainPath := filepath.Join(tmpDir, "main.yml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatalf("failed to write main config: %v", err)
	}

	// Parse the main config with imports allowed
	cfg, err := ParseConfigWithWriterAndImports(mainPath, nil, true)
	if err != nil {
		t.Fatalf("ParseConfigWithWriterAndImports failed: %v", err)
	}

	// Process imports (simulating what validator does)
	if err := ProcessImports(cfg, mainPath); err != nil {
		t.Fatalf("ProcessImports failed: %v", err)
	}

	// Check that vars were merged
	varFound := false
	for _, v := range cfg.Vars {
		if v.Name == "IMPORTED_VAR" && v.Value == "imported_value" {
			varFound = true
			break
		}
	}
	if !varFound {
		t.Error("IMPORTED_VAR not found in merged config")
	}

	// Check that groups were merged
	groupFound := false
	for _, g := range cfg.Groups {
		if g.Name == "imported_group" {
			groupFound = true
			break
		}
	}
	if !groupFound {
		t.Errorf("imported_group not found in merged config. Groups: %+v", cfg.Groups)
	}
}

func TestProcessImportsWithNamespace(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create an imported config file
	importedConfig := `
imported_group:
  desc: Imported group
  imported_cmd:
    desc: Imported command
    script: echo "imported"
`
	importedPath := filepath.Join(tmpDir, "imported.yml")
	if err := os.WriteFile(importedPath, []byte(importedConfig), 0644); err != nil {
		t.Fatalf("failed to write imported config: %v", err)
	}

	// Create a main config with namespaced import
	mainConfig := `
imports:
- url: imported.yml
  namespace: backend

local_group:
  desc: Local group
  script: echo "local"
`
	mainPath := filepath.Join(tmpDir, "main.yml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatalf("failed to write main config: %v", err)
	}

	// Parse the main config
	cfg, err := ParseConfigWithWriterAndImports(mainPath, nil, true)
	if err != nil {
		t.Fatalf("ParseConfigWithWriterAndImports failed: %v", err)
	}

	// Process imports
	if err := ProcessImports(cfg, mainPath); err != nil {
		t.Fatalf("ProcessImports failed: %v", err)
	}

	// Check that a namespace group was created
	namespaceFound := false
	for _, g := range cfg.Groups {
		if g.Name == "backend" {
			namespaceFound = true
			// Check that the imported group is inside the namespace
			groupFound := false
			for _, sg := range g.Groups {
				if sg.Name == "imported_group" {
					groupFound = true
					break
				}
			}
			if !groupFound {
				t.Error("imported_group not found inside namespace")
			}
			break
		}
	}
	if !namespaceFound {
		t.Error("namespace group 'backend' not found")
	}
}
