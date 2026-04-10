package runner

import (
	"lota/config"
	"testing"
)

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

func TestInterpolateSimple(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		vars     map[string]string
		args     map[string]string
		expected string
	}{
		{
			name:     "backward compatibility",
			script:   "echo {{param1}} {{param2}}",
			vars:     map[string]string{},
			args:     map[string]string{"param1": "value1", "param2": "value2"},
			expected: "echo value1 value2",
		},
		{
			name:     "missing placeholder fallback",
			script:   "echo {{missing}}",
			vars:     map[string]string{},
			args:     map[string]string{},
			expected: "echo {{missing}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InterpolateSimple(tt.script, tt.vars, tt.args)
			if result != tt.expected {
				t.Errorf("InterpolateSimple() = %v, want %v", result, tt.expected)
			}
		})
	}
}
