package runner

import (
	"bytes"
	"lota/config"
	"os"
	"strings"
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
	t.Setenv("CUSTOM_ENV_TEST", "1")
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
			name:   "simple variable replacement with $ syntax",
			script: "echo $ENV_VAR",
			context: InterpolationContext{
				Vars: map[string]string{"ENV_VAR": "production"},
				Args: map[string]string{},
			},
			expected: "echo production",
		},
		{
			name:   "nested variable with $ syntax",
			script: "echo $cfg.app_name",
			context: InterpolationContext{
				Vars: map[string]string{"cfg.app_name": "MyApp"},
				Args: map[string]string{},
			},
			expected: "echo MyApp",
		},
		{
			name:   "mixed $ and {{ syntax",
			script: "echo $ENV_VAR and {{OTHER_VAR}}",
			context: InterpolationContext{
				Vars: map[string]string{"ENV_VAR": "prod", "OTHER_VAR": "test"},
				Args: map[string]string{},
			},
			expected: "echo prod and test",
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
		{
			name:   "dollar var followed by dot punctuation",
			script: `echo "Invalid type: $type. Use: major, minor, patch, or auto"`,
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{"type": "patch"},
				ArgDefs: []config.Arg{
					{Name: "type", Type: "str"},
				},
			},
			expected: `echo "Invalid type: patch. Use: major, minor, patch, or auto"`,
		},
		{
			name:   "local shell variable remains untouched",
			script: "CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)\necho $CURRENT_BRANCH",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{},
			},
			expected: "CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)\necho $CURRENT_BRANCH",
		},
		{
			name:   "env variable remains untouched",
			script: "echo $CUSTOM_ENV_TEST",
			context: InterpolationContext{
				Vars: map[string]string{},
				Args: map[string]string{},
			},
			expected: "echo $CUSTOM_ENV_TEST",
		},
		{
			name:   "unknown dollar var similar to config errors",
			script: "echo $environmnt",
			context: InterpolationContext{
				Vars: map[string]string{"environment": "prod"},
				Args: map[string]string{},
			},
			wantErr: true,
		},
		{
			name:   "missing argument definition still errors",
			script: "echo $param1",
			context: InterpolationContext{
				Vars:    map[string]string{},
				Args:    map[string]string{},
				ArgDefs: []config.Arg{{Name: "param1", Type: "str"}},
			},
			wantErr: true,
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

func TestInterpolate_DeprecationWarning(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ctx := InterpolationContext{
		Vars:              map[string]string{},
		Args:              map[string]string{"port": "8080"},
		DeprecationWarned: make(map[string]bool),
	}
	_, _ = Interpolate("server --port={{port}}", ctx)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "warning: {{port}} interpolation is deprecated") {
		t.Errorf("expected deprecation warning, got: %q", output)
	}
}

func TestInterpolate_DeprecationWarning_Var(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ctx := InterpolationContext{
		Vars:              map[string]string{"ENV_VAR": "production"},
		Args:              map[string]string{},
		DeprecationWarned: make(map[string]bool),
	}
	_, _ = Interpolate("echo {{ENV_VAR}}", ctx)

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "warning: {{ENV_VAR}} interpolation is deprecated") {
		t.Errorf("expected deprecation warning for var, got: %q", output)
	}
}
