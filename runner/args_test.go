package runner

import (
	"lota/config"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		cliArgs     []string
		argDefs     []config.Arg
		expected    map[string]string
		expectError bool
	}{
		{
			name:    "simple positional args",
			cliArgs: []string{"value1", "value2"},
			argDefs: []config.Arg{
				{Name: "param1", Type: "str"},
				{Name: "param2", Type: "str"},
			},
			expected: map[string]string{
				"param1": "value1",
				"param2": "value2",
			},
		},
		{
			name:    "wildcard args",
			cliArgs: []string{"backend", "python", "manage.py", "shell"},
			argDefs: []config.Arg{
				{Name: "service", Type: "str"},
				{Name: "commands", Wildcard: true},
			},
			expected: map[string]string{
				"service":  "backend",
				"commands": "python manage.py shell",
			},
		},
		{
			name:    "boolean flag --verbose",
			cliArgs: []string{"--verbose"},
			argDefs: []config.Arg{
				{Name: "verbose", Type: "bool"},
			},
			expected: map[string]string{
				"verbose": "true",
			},
		},
		{
			name:    "boolean flag with negation --!debug",
			cliArgs: []string{"--!debug"},
			argDefs: []config.Arg{
				{Name: "debug", Type: "bool", Default: "true"},
			},
			expected: map[string]string{
				"debug": "false",
			},
		},
		{
			name:    "boolean flag with value --verbose=false",
			cliArgs: []string{"--verbose=false"},
			argDefs: []config.Arg{
				{Name: "verbose", Type: "bool"},
			},
			expected: map[string]string{
				"verbose": "false",
			},
		},
		{
			name:    "short flag -v",
			cliArgs: []string{"-v"},
			argDefs: []config.Arg{
				{Name: "verbose", Short: "v", Type: "bool"},
			},
			expected: map[string]string{
				"verbose": "true",
			},
		},
		{
			name:    "flag with value --output ./dist",
			cliArgs: []string{"--output", "./dist"},
			argDefs: []config.Arg{
				{Name: "output", Short: "o", Type: "str"},
			},
			expected: map[string]string{
				"output": "./dist",
			},
		},
		{
			name:    "flag with =value --output=./dist",
			cliArgs: []string{"--output=./dist"},
			argDefs: []config.Arg{
				{Name: "output", Short: "o", Type: "str"},
			},
			expected: map[string]string{
				"output": "./dist",
			},
		},
		{
			name:    "array args",
			cliArgs: []string{"file1.txt", "file2.txt", "file3.txt"},
			argDefs: []config.Arg{
				{Name: "files", Type: "arr", MaxArr: func(i int) *int { return &i }(3)},
			},
			expected: map[string]string{
				"files": "file1.txt,file2.txt,file3.txt",
			},
		},
		{
			name:    "missing required arg",
			cliArgs: []string{},
			argDefs: []config.Arg{
				{Name: "required", Type: "str", Required: true},
			},
			expectError: true,
		},
		{
			name:    "missing optional arg with default",
			cliArgs: []string{},
			argDefs: []config.Arg{
				{Name: "optional", Type: "str", Default: "default"},
			},
			expected: map[string]string{
				"optional": "default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseArgs(tt.cliArgs, tt.argDefs)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !mapsEqual(result, tt.expected) {
				t.Errorf("ParseArgs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestArgParse(t *testing.T) {
	tests := []struct {
		name     string
		argStr   string
		expected config.Arg
	}{
		{
			name:   "simple name",
			argStr: "param1",
			expected: config.Arg{
				Name: "param1",
			},
		},
		{
			name:   "name with short",
			argStr: "param1|p",
			expected: config.Arg{
				Name:  "param1",
				Short: "p",
			},
		},
		{
			name:   "wildcard",
			argStr: "...commands",
			expected: config.Arg{
				Name:     "commands",
				Wildcard: true,
			},
		},
		{
			name:   "array with max",
			argStr: "files:arr[5]",
			expected: config.Arg{
				Name:   "files",
				Type:   "arr",
				MaxArr: func() *int { i := 5; return &i }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var arg config.Arg
			err := arg.Parse(tt.argStr)

			if err != nil {
				t.Errorf("Unexpected error parsing %s: %v", tt.argStr, err)
				return
			}

			if !argsEqual(arg, tt.expected) {
				t.Errorf("Parse(%s) = %v, want %v", tt.argStr, arg, tt.expected)
			}
		})
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func argsEqual(a, b config.Arg) bool {
	return a.Name == b.Name &&
		a.Short == b.Short &&
		a.Type == b.Type &&
		a.Default == b.Default &&
		a.Wildcard == b.Wildcard &&
		((a.MaxArr == nil && b.MaxArr == nil) ||
			(a.MaxArr != nil && b.MaxArr != nil && *a.MaxArr == *b.MaxArr))
}
