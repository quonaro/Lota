package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseGlobalFlags_Values(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		check     func(t *testing.T, f GlobalFlags)
		remaining []string
	}{
		{
			name: "no flags",
			args: []string{"build"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.Help || f.Verbose || f.DryRun {
					t.Error("no flags should be set")
				}
			},
			remaining: []string{"build"},
		},
		{
			name: "all boolean flags",
			args: []string{"-h", "-v", "-V", "--version", "--dry-run", "-g"},
			check: func(t *testing.T, f GlobalFlags) {
				if !f.Help || !f.Verbose || !f.VersionShort || !f.VersionLong || !f.DryRun || !f.GlobalConfig {
					t.Errorf("flags not parsed: %+v", f)
				}
			},
		},
		{
			name: "config value",
			args: []string{"--config", "custom.yml", "build"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.Config != "custom.yml" {
					t.Errorf("Config = %q", f.Config)
				}
			},
			remaining: []string{"build"},
		},
		{
			name: "timeout value",
			args: []string{"--timeout", "5s"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.Timeout != 5*time.Second {
					t.Errorf("Timeout = %v", f.Timeout)
				}
			},
		},
		{
			name: "completion script value",
			args: []string{"--completion-script", "bash"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.CompletionScript != "bash" {
					t.Errorf("CompletionScript = %q", f.CompletionScript)
				}
			},
		},
		{
			name: "install completion with value",
			args: []string{"--install-completion", "zsh"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.InstallCompletion != "zsh" {
					t.Errorf("InstallCompletion = %q", f.InstallCompletion)
				}
			},
		},
		{
			name: "install completion auto",
			args: []string{"--install-completion"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.InstallCompletion != "auto" {
					t.Errorf("InstallCompletion = %q, want auto", f.InstallCompletion)
				}
			},
		},
		{
			name: "install completion consumes next token",
			args: []string{"--install-completion", "fish", "build"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.InstallCompletion != "fish" {
					t.Errorf("InstallCompletion = %q, want fish", f.InstallCompletion)
				}
			},
			remaining: []string{"build"},
		},
		{
			name: "stops at first non-flag",
			args: []string{"build", "--verbose"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.Verbose {
					t.Error("flags after command are not global")
				}
			},
			remaining: []string{"build", "--verbose"},
		},
		{
			name: "unknown flag stops parsing",
			args: []string{"--bogus", "-v"},
			check: func(t *testing.T, f GlobalFlags) {
				if f.Verbose {
					t.Error("parsing should stop at unknown flag")
				}
			},
			remaining: []string{"--bogus", "-v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, rest, err := ParseGlobalFlags(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, flags)
			if tt.remaining != nil {
				if len(rest) != len(tt.remaining) {
					t.Fatalf("remaining = %v, want %v", rest, tt.remaining)
				}
				for i := range rest {
					if rest[i] != tt.remaining[i] {
						t.Fatalf("remaining = %v, want %v", rest, tt.remaining)
					}
				}
			}
		})
	}
}

func TestParseGlobalFlags_Errors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"config missing value", []string{"--config"}},
		{"completion-script missing value", []string{"--completion-script"}},
		{"timeout missing value", []string{"--timeout"}},
		{"timeout invalid value", []string{"--timeout", "abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ParseGlobalFlags(tt.args); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestValidateFlags_Conflicts(t *testing.T) {
	tests := []struct {
		name  string
		flags GlobalFlags
		want  string
	}{
		{"init+help", GlobalFlags{Init: true, Help: true}, "--init cannot be used"},
		{"init+verbose", GlobalFlags{Init: true, Verbose: true}, "--init cannot be used"},
		{"init+global", GlobalFlags{Init: true, GlobalConfig: true}, "--init cannot be used"},
		{"completion+help", GlobalFlags{CompletionScript: "bash", Help: true}, "--completion-script cannot be used"},
		{"completion+version", GlobalFlags{CompletionScript: "bash", VersionLong: true}, "--completion-script cannot be used"},
		{"install+dryrun", GlobalFlags{InstallCompletion: "auto", DryRun: true}, "--install-completion cannot be used"},
		{"g+config", GlobalFlags{GlobalConfig: true, Config: "x.yml"}, "-g cannot be used with --config"},
		{"u+config", GlobalFlags{UserConfig: true, Config: "x.yml"}, "-u cannot be used with --config"},
		{"g+u", GlobalFlags{GlobalConfig: true, UserConfig: true}, "cannot be used together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlags(tt.flags)
			if err == nil {
				t.Fatal("expected conflict error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}

	// --install-completion followed by another flag means "auto", which then
	// conflicts with that flag.
	if _, _, err := ParseGlobalFlags([]string{"--install-completion", "-v"}); err == nil {
		t.Error("expected conflict between --install-completion and -v")
	}

	if err := validateFlags(GlobalFlags{}); err != nil {
		t.Errorf("empty flags should be valid: %v", err)
	}
	if err := validateFlags(GlobalFlags{Init: true}); err != nil {
		t.Errorf("init alone should be valid: %v", err)
	}
}

func TestHasHelpFlag(t *testing.T) {
	if !hasHelpFlag([]string{"build", "--help"}) {
		t.Error("should detect --help")
	}
	if !hasHelpFlag([]string{"-h"}) {
		t.Error("should detect -h")
	}
	if hasHelpFlag([]string{"build", "--helper"}) {
		t.Error("should not match --helper")
	}
}

func TestHasVerboseFlag(t *testing.T) {
	if !hasVerboseFlag([]string{"build", "-v"}) {
		t.Error("should detect -v")
	}
	if !hasVerboseFlag([]string{"--verbose"}) {
		t.Error("should detect --verbose")
	}
	if hasVerboseFlag([]string{"build"}) {
		t.Error("should return false")
	}
}

func TestHandleGlobalFlags_Version(t *testing.T) {
	handled, err := HandleGlobalFlags(GlobalFlags{VersionShort: true})
	if !handled || err != nil {
		t.Errorf("VersionShort: handled=%v err=%v", handled, err)
	}
	handled, err = HandleGlobalFlags(GlobalFlags{VersionLong: true})
	if !handled || err != nil {
		t.Errorf("VersionLong: handled=%v err=%v", handled, err)
	}
}

func TestHandleGlobalFlags_None(t *testing.T) {
	handled, err := HandleGlobalFlags(GlobalFlags{})
	if handled || err != nil {
		t.Errorf("empty flags should not be handled: %v %v", handled, err)
	}
	// -g/-u alone are modifiers, not actions.
	handled, err = HandleGlobalFlags(GlobalFlags{GlobalConfig: true})
	if handled || err != nil {
		t.Errorf("-g alone should not be handled: %v %v", handled, err)
	}
}

func TestHandleGlobalFlags_Init(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lota.yml")
	handled, err := HandleGlobalFlags(GlobalFlags{Init: true, Config: path})
	if !handled || err != nil {
		t.Fatalf("Init: handled=%v err=%v", handled, err)
	}

	// Second run must fail: file exists.
	_, err = HandleGlobalFlags(GlobalFlags{Init: true, Config: path})
	if err == nil {
		t.Error("expected error when config already exists")
	}
}

func TestHandleGlobalFlags_CompletionScript(t *testing.T) {
	handled, err := HandleGlobalFlags(GlobalFlags{CompletionScript: "bash"})
	if !handled || err != nil {
		t.Errorf("completion-script bash: handled=%v err=%v", handled, err)
	}

	handled, err = HandleGlobalFlags(GlobalFlags{CompletionScript: "nushell"})
	if !handled || err == nil {
		t.Error("expected error for unsupported shell")
	}
}

func TestHandleGlobalFlags_InstallCompletionError(t *testing.T) {
	// Unsupported shell must fail before touching the filesystem.
	handled, err := HandleGlobalFlags(GlobalFlags{InstallCompletion: "pwsh"})
	if !handled || err == nil {
		t.Error("expected error for unsupported shell")
	}
}
