package complete

import (
	"testing"

	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/predict"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"lota he", []string{"lota", "he"}},
		{"lota hello --verbose", []string{"lota", "hello", "--verbose"}},
		{"", []string{}},
		{"lota", []string{"lota"}},
	}

	for _, tt := range tests {
		args := ParseArgs(tt.line)
		if len(args) != len(tt.want) {
			t.Fatalf("ParseArgs(%q) got %d args, want %d", tt.line, len(args), len(tt.want))
		}
		for i, arg := range args {
			if arg.Text != tt.want[i] {
				t.Errorf("arg[%d].Text = %q, want %q", i, arg.Text, tt.want[i])
			}
		}
	}
}

func TestExtractCommandArgs(t *testing.T) {
	tests := []struct {
		line        string
		commandName string
		want        []string
	}{
		{"lota he", "lota", []string{"he"}},
		{"/usr/bin/lota he", "lota", []string{"he"}},
		{"lota", "lota", nil},
		{"", "lota", nil},
		{"lota hello world", "lota", []string{"hello", "world"}},
	}

	for _, tt := range tests {
		parsed := ParseArgs(tt.line)
		got := ExtractCommandArgs(parsed, tt.commandName)
		if len(got) != len(tt.want) {
			t.Fatalf("ExtractCommandArgs(%q, %q) got %d args, want %d", tt.line, tt.commandName, len(got), len(tt.want))
		}
		for i := range got {
			if got[i].Text != tt.want[i] {
				t.Errorf("arg[%d].Text = %q, want %q", i, got[i].Text, tt.want[i])
			}
		}
	}
}

func TestRun_SubCommands(t *testing.T) {
	cmd := &complete.Command{
		Sub: map[string]*complete.Command{
			"hello": {},
			"help":  {},
		},
	}

	args := ParseArgs("lota he")
	args = args[1:] // drop program name

	options, err := Run(cmd, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundHello, foundHelp := false, false
	for _, opt := range options {
		if opt == "hello" {
			foundHello = true
		}
		if opt == "help" {
			foundHelp = true
		}
	}
	if !foundHello {
		t.Error("expected 'hello' in completions")
	}
	if !foundHelp {
		t.Error("expected 'help' in completions")
	}
}

func TestRun_Flags(t *testing.T) {
	cmd := &complete.Command{
		Flags: map[string]complete.Predictor{
			"verbose": predict.Nothing,
		},
		Args: predict.Nothing,
	}

	args := ParseArgs("lota -")
	args = args[1:]

	options, err := Run(cmd, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundVerbose := false
	for _, opt := range options {
		if opt == "--verbose" {
			foundVerbose = true
		}
	}
	if !foundVerbose {
		t.Errorf("expected verbose flag in completions, got %v", options)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	cmd := &complete.Command{
		Sub: map[string]*complete.Command{
			"hello": {},
		},
	}

	args := ParseArgs("lota unknown ")
	args = args[1:]

	_, err := Run(cmd, args)
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestRun_AutoDescendWithoutTrailingSpace(t *testing.T) {
	cmd := &complete.Command{
		Sub: map[string]*complete.Command{
			"group1": {
				Sub: map[string]*complete.Command{
					"command1": {
						Flags: map[string]complete.Predictor{
							"verbose": predict.Nothing,
						},
					},
				},
			},
		},
	}

	args := ParseArgs("lota group1 command1")
	args = args[1:]

	options, err := Run(cmd, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(options) == 0 {
		t.Fatalf("expected completion options, got empty")
	}
	if !contains(options, "--verbose") {
		t.Fatalf("expected --verbose suggestion without trailing space, got %v", options)
	}
}

func TestRun_NoAutoDescendWhenAmbiguous(t *testing.T) {
	cmd := &complete.Command{
		Sub: map[string]*complete.Command{
			"test":        {},
			"test-import": {},
		},
	}

	args := ParseArgs("lota test")
	args = args[1:]

	options, err := Run(cmd, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(options, "test") || !contains(options, "test-import") {
		t.Fatalf("expected ambiguous subcommands in suggestions, got %v", options)
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
