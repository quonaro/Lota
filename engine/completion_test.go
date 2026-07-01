package engine

import (
	"strings"
	"testing"
)

func TestGetCompletionScript(t *testing.T) {
	tests := []struct {
		shell     string
		binary    string
		shouldErr bool
		contains  []string
	}{
		{
			shell:    "bash",
			binary:   "lota",
			contains: []string{"lota __complete", "COMPREPLY"},
		},
		{
			shell:    "bash",
			binary:   "my-app",
			contains: []string{"my-app __complete", "_my_app_complete"},
		},
		{
			shell:    "zsh",
			binary:   "lota",
			contains: []string{"lota __complete", "compadd"},
		},
		{
			shell:    "fish",
			binary:   "lota",
			contains: []string{"lota __complete", "complete -c lota"},
		},
		{
			shell:     "pwsh",
			binary:    "lota",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.shell+"_"+tt.binary, func(t *testing.T) {
			script, err := GetCompletionScript(tt.shell, tt.binary)
			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, s := range tt.contains {
				if !strings.Contains(script, s) {
					t.Errorf("expected script to contain %q, got:\n%s", s, script)
				}
			}
		})
	}
}

func TestAppComplete(t *testing.T) {
	app, err := NewBuilder("test", []byte(`
hello:
  desc: Say hello
help:
  desc: Show help
infra:
  deploy:
    desc: Deploy infra
`)).Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	t.Run("root commands", func(t *testing.T) {
		options, hint, err := app.Complete("test he", 7, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hint != "" {
			t.Fatalf("unexpected hint: %q", hint)
		}
		foundHello := false
		foundHelp := false
		for _, opt := range options {
			if opt == "hello" {
				foundHello = true
			}
			if opt == "help" {
				foundHelp = true
			}
		}
		if !foundHello {
			t.Errorf("expected 'hello' in completions, got %v", options)
		}
		if !foundHelp {
			t.Errorf("expected 'help' in completions, got %v", options)
		}
	})

	t.Run("group command", func(t *testing.T) {
		options, hint, err := app.Complete("test infra ", 11, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hint != "" {
			t.Fatalf("unexpected hint: %q", hint)
		}
		found := false
		for _, opt := range options {
			if opt == "deploy" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'deploy' in completions, got %v", options)
		}
	})

	t.Run("global flags", func(t *testing.T) {
		options, _, err := app.Complete("test --", 7, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		found := false
		for _, opt := range options {
			if opt == "--version" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected '--version' in completions, got %v", options)
		}
	})
}
