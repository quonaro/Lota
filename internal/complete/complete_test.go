package complete

import (
	"strings"
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

func TestTokener(t *testing.T) {
	t.Run("plain text", func(t *testing.T) {
		var tok Tokener
		for i := 0; i < len("abc"); i++ {
			tok.Visit("abc"[i])
		}
		if tok.Quoted() {
			t.Error("should not be quoted")
		}
		if tok.Escaped() {
			t.Error("should not be escaped")
		}
		if tok.Fixed() != "abc" {
			t.Errorf("Fixed = %q", tok.Fixed())
		}
		if tok.Closed() != "abc" {
			t.Errorf("Closed = %q", tok.Closed())
		}
	})

	t.Run("escape", func(t *testing.T) {
		var tok Tokener
		for i := 0; i < len("a\\"); i++ {
			tok.Visit("a\\"[i])
		}
		if !tok.Escaped() {
			t.Error("trailing backslash should set escaped")
		}
	})

	t.Run("open quote closed on Close", func(t *testing.T) {
		var tok Tokener
		s := `echo "hel`
		for i := 0; i < len(s); i++ {
			tok.Visit(s[i])
		}
		if !tok.Quoted() {
			t.Error("should be inside quote")
		}
		if got := tok.Closed(); !strings.HasSuffix(got, `"`) {
			t.Errorf("Closed should append missing quote, got %q", got)
		}
	})

	t.Run("balanced quotes", func(t *testing.T) {
		var tok Tokener
		s := `a "b" 'c'`
		for i := 0; i < len(s); i++ {
			tok.Visit(s[i])
		}
		if tok.Quoted() {
			t.Error("quotes should be balanced")
		}
	})
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		s      string
		prefix string
		ok     bool
	}{
		{"build", "b", true},
		{"build", "build", true},
		{"build", "builder", false},
		{"--flag", "--", true},
		{"abc", "", true},
	}
	for _, tt := range tests {
		_, ok := hasPrefix(tt.s, tt.prefix)
		if ok != tt.ok {
			t.Errorf("hasPrefix(%q, %q) = %v, want %v", tt.s, tt.prefix, ok, tt.ok)
		}
	}
}

func TestFilterByPrefix(t *testing.T) {
	got := filterByPrefix("b", "build", "bench", "test")
	if len(got) != 2 {
		t.Errorf("expected 2 filtered, got %v", got)
	}
	// No matches → original options returned.
	got = filterByPrefix("zzz", "a", "b")
	if len(got) != 2 {
		t.Errorf("expected all options when nothing matches, got %v", got)
	}
}

func TestFlagMatchesDashStyle(t *testing.T) {
	if !flagMatchesDashStyle("verbose", "--") {
		t.Error("long flag should match --")
	}
	if flagMatchesDashStyle("v", "--") {
		t.Error("single-char flag should not match --")
	}
	if !flagMatchesDashStyle("v", "-") {
		t.Error("single-char flag should match -")
	}
	if !flagMatchesDashStyle("verbose", "-") {
		t.Error("long flag should match -")
	}
}

func TestFormatFlag(t *testing.T) {
	if got := formatFlag("v"); got != "-v" {
		t.Errorf("got %q", got)
	}
	if got := formatFlag("verbose"); got != "--verbose" {
		t.Errorf("got %q", got)
	}
}

func TestUniqueStrings(t *testing.T) {
	got := uniqueStrings([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 {
		t.Fatalf("expected 3 unique, got %v", got)
	}
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("order should be preserved, got %v", got)
	}
	if got := uniqueStrings(nil); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestFilterUsedFlags(t *testing.T) {
	used := map[string]struct{}{"--verbose": {}}
	got := filterUsedFlags([]string{"--verbose", "--force"}, used)
	if len(got) != 1 || got[0] != "--force" {
		t.Errorf("got %v", got)
	}
	// Empty used map returns everything.
	got = filterUsedFlags([]string{"--a"}, nil)
	if len(got) != 1 {
		t.Errorf("got %v", got)
	}
}

func TestCompletionCategory(t *testing.T) {
	if completionCategory("build") != 0 {
		t.Error("command should be category 0")
	}
	if completionCategory("--long") != 1 {
		t.Error("--flag should be category 1")
	}
	if completionCategory("-s") != 2 {
		t.Error("-f should be category 2")
	}
}

func TestSortCompletionOptions(t *testing.T) {
	got := sortCompletionOptions([]string{"-v", "--verbose", "build", "alpha", "--all"})
	// Commands first (sorted), then long flags, then short flags.
	want := []string{"alpha", "build", "--all", "--verbose", "-v"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// Duplicates removed.
	got = sortCompletionOptions([]string{"a", "a"})
	if len(got) != 1 {
		t.Errorf("expected dedup, got %v", got)
	}
}

func TestSuggestFlagValue(t *testing.T) {
	p := predict.Set([]string{"one", "two"})
	cmd := &complete.Command{Flags: map[string]complete.Predictor{"mode": p}}
	c := completer{Completer: cmd}

	got := c.suggestFlagValue("mode", "")
	if len(got) != 2 {
		t.Errorf("expected 2 values, got %v", got)
	}
	got = c.suggestFlagValue("mode", "o")
	if len(got) != 1 || got[0] != "one" {
		t.Errorf("expected [one], got %v", got)
	}
	// Unknown flag → no options.
	if got := c.suggestFlagValue("missing", ""); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestUsedFormattedFlags(t *testing.T) {
	c := completer{
		args: []Arg{
			{Text: "--verbose", Completed: true, Parsed: Parsed{HasFlag: true, Flag: "verbose"}},
			{Text: "pos", Completed: true},
		},
	}
	used := c.usedFormattedFlags()
	if _, ok := used["--verbose"]; !ok {
		t.Errorf("expected --verbose in %v", used)
	}
	if len(used) != 1 {
		t.Errorf("got %v", used)
	}
}
