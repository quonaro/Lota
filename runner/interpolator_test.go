package runner

import (
	"lota/config"
	"testing"
)

func TestFindSimilarVars(t *testing.T) {
	tests := []struct {
		name        string
		placeholder string
		vars        map[string]string
		expected    []string
	}{
		{
			name:        "find similar with dot prefix",
			placeholder: "config.public.app_name",
			vars: map[string]string{
				"config.database.host": "localhost",
				"config.database.port": "5432",
				"config.app_name":      "MyApp",
			},
			expected: []string{"config.app_name", "config.database.host", "config.database.port"},
		},
		{
			name:        "no similar vars",
			placeholder: "unknown.var",
			vars: map[string]string{
				"other.value": "test",
			},
			expected: []string{},
		},
		{
			name:        "exact match",
			placeholder: "config",
			vars: map[string]string{
				"config": "value",
			},
			expected: []string{"config"},
		},
		{
			name:        "no dot in placeholder",
			placeholder: "simple",
			vars: map[string]string{
				"simple.value": "test",
				"simple.other": "test2",
			},
			expected: []string{"simple.other", "simple.value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSimilarVars(tt.placeholder, tt.vars)
			if len(result) != len(tt.expected) {
				t.Errorf("findSimilarVars() = %v, want %v", result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("findSimilarVars()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		context  InterpolationContext
		expected string
		wantErr  bool
	}{
		{
			name:   "simple variable replacement",
			script: "echo {{ENV_VAR}}",
			context: InterpolationContext{
				Vars: map[string]string{"ENV_VAR": "production"},
				Args: map[string]string{},
			},
			expected: "echo production",
		},
		{
			name:   "simple argument replacement",
			script: "echo {{param1}}",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{"param1": "test"},
			},
			expected: "echo test",
		},
		{
			name:   "no placeholders",
			script: "echo hello world",
			context: InterpolationContext{
				Vars: map[string]string{"VAR": "value"},
				Args: map[string]string{"arg": "value"},
			},
			expected: "echo hello world",
		},
		{
			name:   "missing placeholder error",
			script: "echo {{missing}}",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{},
			},
			wantErr: true,
		},
		{
			name:   "missing placeholder with similar vars",
			script: "echo {{config.public.app_name}}",
			context: InterpolationContext{
				Vars: map[string]string{
					"config.database.host": "localhost",
					"config.database.port": "5432",
					"config.app_name":      "MyApp",
				},
				Args: map[string]string{},
			},
			wantErr: true,
		},
		{
			name:   "typed int validation",
			script: "echo {{count}}",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{"count": "42"},
				ArgDefs: []config.Arg{
					{Name: "count", Type: "int"},
				},
			},
			expected: "echo 42",
		},
		{
			name:   "typed int validation error",
			script: "echo {{count}}",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{"count": "invalid"},
				ArgDefs: []config.Arg{
					{Name: "count", Type: "int"},
				},
			},
			wantErr: true,
		},
		{
			name:   "typed bool with negation",
			script: "echo {{debug}}",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{"debug": "!true"},
				ArgDefs: []config.Arg{
					{Name: "debug", Type: "bool"},
				},
			},
			expected: "echo false",
		},
		{
			name:   "typed array formatting",
			script: "echo {{files}}",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{"files": "file1.txt, file2.txt, file3.txt"},
				ArgDefs: []config.Arg{
					{Name: "files", Type: "arr"},
				},
			},
			expected: "echo file1.txt file2.txt file3.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Interpolate(tt.script, tt.context)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Interpolate() = %v, want %v", result, tt.expected)
			}
		})
	}
}
