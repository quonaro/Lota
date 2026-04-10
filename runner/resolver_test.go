package runner

import (
	"lota/config"
	"reflect"
	"sort"
	"testing"
)

func TestVarsToEnv(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]string
		expected []string
	}{
		{
			name: "simple vars",
			vars: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expected: []string{"KEY1=value1", "KEY2=value2"},
		},
		{
			name:     "empty vars",
			vars:     map[string]string{},
			expected: []string{},
		},
		{
			name: "empty value",
			vars: map[string]string{
				"EMPTY": "",
			},
			expected: []string{"EMPTY="},
		},
		{
			name: "single var",
			vars: map[string]string{
				"SINGLE": "only",
			},
			expected: []string{"SINGLE=only"},
		},
		{
			name: "vars with special characters",
			vars: map[string]string{
				"URL":  "https://example.com/path?query=1",
				"PATH": "/usr/local/bin:/usr/bin",
			},
			expected: []string{"PATH=/usr/local/bin:/usr/bin", "URL=https://example.com/path?query=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VarsToEnv(tt.vars)
			sort.Strings(result)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("VarsToEnv() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResolveVars(t *testing.T) {
	tests := []struct {
		name     string
		app      config.AppConfig
		group    *config.Group
		command  config.Command
		expected map[string]string
	}{
		{
			name:    "only app vars",
			app:     config.AppConfig{Vars: []config.Var{{Name: "APP_VAR", Value: "app_value"}}},
			group:   nil,
			command: config.Command{Name: "cmd"},
			expected: map[string]string{
				"APP_VAR": "app_value",
			},
		},
		{
			name: "group overrides app",
			app:  config.AppConfig{Vars: []config.Var{{Name: "DOCKER", Value: "docker"}}},
			group: &config.Group{
				Name: "dev",
				Vars: []config.Var{{Name: "DOCKER", Value: "docker-compose"}},
			},
			command: config.Command{Name: "run"},
			expected: map[string]string{
				"DOCKER": "docker-compose",
			},
		},
		{
			name: "command overrides group and app",
			app:  config.AppConfig{Vars: []config.Var{{Name: "ENV", Value: "global"}}},
			group: &config.Group{
				Name: "dev",
				Vars: []config.Var{{Name: "ENV", Value: "dev"}},
			},
			command: config.Command{
				Name: "run",
				Vars: []config.Var{{Name: "ENV", Value: "local"}},
			},
			expected: map[string]string{
				"ENV": "local",
			},
		},
		{
			name: "merge vars from all levels",
			app: config.AppConfig{
				Vars: []config.Var{
					{Name: "APP_NAME", Value: "myapp"},
					{Name: "GLOBAL", Value: "global_value"},
				},
			},
			group: &config.Group{
				Name: "dev",
				Vars: []config.Var{
					{Name: "ENV", Value: "development"},
					{Name: "GLOBAL", Value: "group_overridden"},
				},
			},
			command: config.Command{
				Name: "run",
				Vars: []config.Var{
					{Name: "DEBUG", Value: "true"},
				},
			},
			expected: map[string]string{
				"APP_NAME": "myapp",
				"GLOBAL":   "group_overridden",
				"ENV":      "development",
				"DEBUG":    "true",
			},
		},
		{
			name:     "no vars",
			app:      config.AppConfig{},
			group:    nil,
			command:  config.Command{Name: "cmd"},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveVars(tt.app, tt.group, tt.command)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ResolveVars() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCollectAllVars(t *testing.T) {
	tests := []struct {
		name     string
		configs  []config.AppConfig
		expected []ScopedVar
	}{
		{
			name:     "empty config",
			configs:  []config.AppConfig{},
			expected: []ScopedVar{},
		},
		{
			name: "app level vars",
			configs: []config.AppConfig{
				{
					Vars: []config.Var{{Name: "APP_VAR", Value: "value"}},
				},
			},
			expected: []ScopedVar{
				{Scope: "app", Path: "app[0]", Var: config.Var{Name: "APP_VAR", Value: "value"}},
			},
		},
		{
			name: "mixed scopes",
			configs: []config.AppConfig{
				{
					Vars: []config.Var{{Name: "GLOBAL", Value: "g"}},
					Groups: []config.Group{
						{
							Name:     "dev",
							Vars:     []config.Var{{Name: "DEV", Value: "d"}},
							Commands: []config.Command{{Name: "run", Vars: []config.Var{{Name: "RUN", Value: "r"}}}},
						},
					},
					Commands: []config.Command{{Name: "build", Vars: []config.Var{{Name: "BUILD", Value: "b"}}}},
				},
			},
			expected: []ScopedVar{
				{Scope: "app", Path: "app[0]", Var: config.Var{Name: "GLOBAL", Value: "g"}},
				{Scope: "group", Path: "app[0].dev", Var: config.Var{Name: "DEV", Value: "d"}},
				{Scope: "command", Path: "app[0].dev.run", Var: config.Var{Name: "RUN", Value: "r"}},
				{Scope: "command", Path: "app[0].build", Var: config.Var{Name: "BUILD", Value: "b"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CollectAllVars(tt.configs)
			if !scopedVarSlicesEqual(result, tt.expected) {
				t.Errorf("CollectAllVars() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResolveArgs(t *testing.T) {
	tests := []struct {
		name     string
		app      config.AppConfig
		group    *config.Group
		command  config.Command
		expected []config.Arg
	}{
		{
			name: "app level args only",
			app: config.AppConfig{
				Args: []config.Arg{
					{Name: "environment", Type: "str", Default: "dev"},
					{Name: "verbose", Type: "bool", Default: "false"},
				},
			},
			group:   nil,
			command: config.Command{Name: "test-command"},
			expected: []config.Arg{
				{Name: "environment", Type: "str", Default: "dev"},
				{Name: "verbose", Type: "bool", Default: "false"},
			},
		},
		{
			name: "command overrides group and app",
			app: config.AppConfig{
				Args: []config.Arg{
					{Name: "environment", Type: "str", Default: "dev"},
					{Name: "verbose", Type: "bool", Default: "false"},
				},
			},
			group: &config.Group{
				Name: "test-group",
				Args: []config.Arg{
					{Name: "environment", Type: "str", Default: "prod"},
					{Name: "timeout", Type: "int", Default: "30"},
				},
			},
			command: config.Command{
				Name: "test-command",
				Args: []config.Arg{
					{Name: "timeout", Type: "int", Default: "60"},
					{Name: "debug", Type: "bool", Default: "true"},
				},
			},
			expected: []config.Arg{
				{Name: "environment", Type: "str", Default: "prod"},
				{Name: "verbose", Type: "bool", Default: "false"},
				{Name: "timeout", Type: "int", Default: "60"},
				{Name: "debug", Type: "bool", Default: "true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveArgs(tt.app, tt.group, tt.command)
			if !argsSlicesEqual(result, tt.expected) {
				t.Errorf("ResolveArgs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func scopedVarSlicesEqual(a, b []ScopedVar) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]ScopedVar)
	bMap := make(map[string]ScopedVar)
	for _, v := range a {
		aMap[v.Path+"."+v.Var.Name] = v
	}
	for _, v := range b {
		bMap[v.Path+"."+v.Var.Name] = v
	}
	return reflect.DeepEqual(aMap, bMap)
}

func argsSlicesEqual(a, b []config.Arg) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]config.Arg)
	bMap := make(map[string]config.Arg)
	for _, v := range a {
		aMap[v.Name] = v
	}
	for _, v := range b {
		bMap[v.Name] = v
	}
	return reflect.DeepEqual(aMap, bMap)
}
