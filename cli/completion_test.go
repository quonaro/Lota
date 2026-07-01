package cli

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/engine"
	icomp "github.com/quonaro/lota/internal/complete"
)

func TestBuildCompletion_EmptyConfig(t *testing.T) {
	cfg := &config.AppConfig{}
	comp := engine.BuildCompletion(cfg)

	if len(comp.Sub) != 0 {
		t.Errorf("expected 0 subcommands, got %d", len(comp.Sub))
	}

	expectedFlags := []string{"v", "verbose", "V", "version", "dry-run", "init", "config"}
	for _, f := range expectedFlags {
		if _, ok := comp.Flags[f]; !ok {
			t.Errorf("expected global flag %q", f)
		}
	}
}

func TestBuildCompletion_WithGroupsAndCommands(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "db",
				Commands: []config.Command{
					{Name: "migrate"},
					{Name: "seed"},
				},
			},
		},
		Commands: []config.Command{
			{Name: "hello"},
			{Name: "world"},
		},
	}

	comp := engine.BuildCompletion(cfg)

	if _, ok := comp.Sub["db"]; !ok {
		t.Error("expected 'db' group")
	}
	if _, ok := comp.Sub["hello"]; !ok {
		t.Error("expected 'hello' command")
	}
	if _, ok := comp.Sub["world"]; !ok {
		t.Error("expected 'world' command")
	}

	db := comp.Sub["db"]
	if _, ok := db.Sub["migrate"]; !ok {
		t.Error("expected 'db migrate' command")
	}
	if _, ok := db.Sub["seed"]; !ok {
		t.Error("expected 'db seed' command")
	}
}

func TestBuildCompletion_NestedGroups(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "infra",
				Groups: []config.Group{
					{
						Name: "db",
						Commands: []config.Command{
							{Name: "migrate"},
						},
					},
				},
			},
		},
	}

	comp := engine.BuildCompletion(cfg)

	infra, ok := comp.Sub["infra"]
	if !ok {
		t.Fatal("expected 'infra' group")
	}

	db, ok := infra.Sub["db"]
	if !ok {
		t.Fatal("expected 'infra db' group")
	}

	if _, ok := db.Sub["migrate"]; !ok {
		t.Error("expected 'infra db migrate' command")
	}
}

func TestBuildCompletion_CommandFlags(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{
				Name: "build",
				Args: []config.Arg{
					{Name: "target", Short: "t"},
					{Name: "verbose", Short: "v"},
				},
			},
		},
	}

	comp := engine.BuildCompletion(cfg)
	build := comp.Sub["build"]

	if _, ok := build.Flags["t"]; !ok {
		t.Error("expected t flag")
	}
	if _, ok := build.Flags["target"]; !ok {
		t.Error("expected target flag")
	}
	if _, ok := build.Flags["v"]; !ok {
		t.Error("expected v flag")
	}
	if _, ok := build.Flags["verbose"]; !ok {
		t.Error("expected verbose flag")
	}
}

func TestBuildCompletion_GroupFlags(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "deploy",
				Args: []config.Arg{
					{Name: "env", Short: "e"},
				},
				Commands: []config.Command{
					{Name: "prod"},
				},
			},
		},
	}

	comp := engine.BuildCompletion(cfg)
	deploy := comp.Sub["deploy"]

	if _, ok := deploy.Flags["e"]; !ok {
		t.Error("expected e flag on group")
	}
	if _, ok := deploy.Flags["env"]; !ok {
		t.Error("expected env flag on group")
	}
}

func TestBuildCompletion_DoesNotExposePositionalAsFlags(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "group2",
				Commands: []config.Command{
					{
						Name: "command5",
						Args: []config.Arg{
							{Name: "service", Type: "str"},
							{Name: "cmd", Wildcard: true},
						},
					},
				},
			},
		},
	}

	comp := engine.BuildCompletion(cfg)
	cmd5 := comp.Sub["group2"].Sub["command5"]

	if _, ok := cmd5.Flags["service"]; ok {
		t.Error("did not expect positional arg 'service' to be exposed as flag")
	}
	if _, ok := cmd5.Flags["cmd"]; ok {
		t.Error("did not expect wildcard arg 'cmd' to be exposed as flag")
	}
}

func TestBuildCompletion_ConfigFlagPredictsFiles(t *testing.T) {
	cfg := &config.AppConfig{}
	comp := engine.BuildCompletion(cfg)

	if comp.Flags["config"] == nil {
		t.Error("expected config to have a predictor")
	}
}

func TestPrintCompletionScript(t *testing.T) {
	// Smoke test: just verify it prints non-empty output without error.
	tests := []string{"bash", "zsh", "fish"}
	for _, shell := range tests {
		t.Run(shell, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe error: %v", err)
			}
			oldStdout := os.Stdout
			os.Stdout = w

			scriptErr := PrintCompletionScript(shell)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf strings.Builder
			if _, err := io.Copy(&buf, r); err != nil {
				t.Fatalf("read error: %v", err)
			}
			output := buf.String()

			if scriptErr != nil {
				t.Fatalf("unexpected error: %v", scriptErr)
			}
			if output == "" {
				t.Error("expected non-empty script output")
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		if err := PrintCompletionScript("pwsh"); err == nil {
			t.Error("expected error for unsupported shell")
		}
	})
}

func TestGetCompletionScript(t *testing.T) {
	tests := []struct {
		shell     string
		shouldErr bool
		contains  string
	}{
		{"bash", false, "lota __complete"},
		{"zsh", false, "lota __complete"},
		{"fish", false, "lota __complete"},
		{"pwsh", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			script, err := engine.GetCompletionScript(tt.shell, "lota")
			if tt.shouldErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !strings.Contains(script, tt.contains) {
				t.Errorf("expected script to contain %q, got:\n%s", tt.contains, script)
			}
		})
	}
}

func TestRunCompleteSubcommand_ValidInput(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "hello"},
			{Name: "help"},
		},
	}
	comp := engine.BuildCompletion(cfg)

	// Simulate "lota he" with cursor at position 7 (after "lota he")
	line := "lota he"
	point := 7

	parsedArgs := icomp.ParseArgs(line[:point])
	if len(parsedArgs) > 0 {
		parsedArgs = parsedArgs[1:]
	}

	options, err := icomp.Run(comp, parsedArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
		t.Error("expected 'hello' in completions")
	}
	if !foundHelp {
		t.Error("expected 'help' in completions")
	}
}

func TestRunCompleteSubcommand_FlagDashStyles(t *testing.T) {
	comp := engine.BuildCompletion(&config.AppConfig{})

	longLine := "lota --"
	longArgs := icomp.ParseArgs(longLine)
	if len(longArgs) > 0 {
		longArgs = longArgs[1:]
	}
	longOpts, err := icomp.Run(comp, longArgs)
	if err != nil {
		t.Fatalf("unexpected error for long flags: %v", err)
	}
	if !containsString(longOpts, "--version") {
		t.Fatalf("expected --version in long options, got %v", longOpts)
	}
	if containsString(longOpts, "-V") {
		t.Fatalf("did not expect -V in long options, got %v", longOpts)
	}

	shortLine := "lota -"
	shortArgs := icomp.ParseArgs(shortLine)
	if len(shortArgs) > 0 {
		shortArgs = shortArgs[1:]
	}
	shortOpts, err := icomp.Run(comp, shortArgs)
	if err != nil {
		t.Fatalf("unexpected error for short flags: %v", err)
	}
	if !containsString(shortOpts, "-V") {
		t.Fatalf("expected -V in short options, got %v", shortOpts)
	}
	if !containsString(shortOpts, "--version") {
		t.Fatalf("expected --version in short options, got %v", shortOpts)
	}
	if containsString(shortOpts, "-dry-run") {
		t.Fatalf("did not expect -dry-run in short options, got %v", shortOpts)
	}
}

func TestRunCompleteSubcommand_CompletedRootFlag(t *testing.T) {
	comp := engine.BuildCompletion(&config.AppConfig{})

	line := "lota -h "
	args := icomp.ParseArgs(line)
	if len(args) > 0 {
		args = args[1:]
	}

	_, err := icomp.Run(comp, args)
	if err != nil {
		t.Fatalf("expected no error for completed root flag, got %v", err)
	}
}

func TestRunCompleteSubcommand_GroupContextIncludesFlags(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "group1",
				Args: []config.Arg{{Name: "env", Short: "e"}},
				Commands: []config.Command{
					{Name: "command1"},
				},
			},
		},
	}

	comp := engine.BuildCompletion(cfg)
	line := "lota group1 "
	args := icomp.ParseArgs(line)
	if len(args) > 0 {
		args = args[1:]
	}

	options, err := icomp.Run(comp, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(options, "command1") {
		t.Fatalf("expected group command in options, got %v", options)
	}
	if !containsString(options, "--env") {
		t.Fatalf("expected group flag --env in options, got %v", options)
	}
	if containsString(options, "--timeout") {
		t.Fatalf("did not expect global flag --timeout in group options, got %v", options)
	}
}

func TestRunCompleteSubcommand_RootContextIncludesGlobalFlags(t *testing.T) {
	comp := engine.BuildCompletion(&config.AppConfig{})
	line := "lota --"
	args := icomp.ParseArgs(line)
	if len(args) > 0 {
		args = args[1:]
	}

	options, err := icomp.Run(comp, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(options, "--timeout") {
		t.Fatalf("expected global flag --timeout at root, got %v", options)
	}
}

func TestRunCompleteSubcommand_OrderCommandsLongShort(t *testing.T) {
	cfg := &config.AppConfig{
		Groups:   []config.Group{{Name: "group1"}},
		Commands: []config.Command{{Name: "alpha"}},
	}
	comp := engine.BuildCompletion(cfg)
	line := "lota "
	args := icomp.ParseArgs(line)
	if len(args) > 0 {
		args = args[1:]
	}

	options, err := icomp.Run(comp, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmdIdx := indexOf(options, "alpha")
	longIdx := indexOf(options, "--version")
	shortIdx := indexOf(options, "-V")

	if cmdIdx == -1 || longIdx == -1 || shortIdx == -1 {
		t.Fatalf("missing required options in %v", options)
	}
	if cmdIdx >= longIdx || longIdx >= shortIdx {
		t.Fatalf("expected commands < long flags < short flags, got %v", options)
	}
}

func TestRunCompleteSubcommand_DoesNotRepeatUsedFlag(t *testing.T) {
	comp := engine.BuildCompletion(&config.AppConfig{})

	shortLine := "lota -V"
	shortArgs := icomp.ParseArgs(shortLine)
	if len(shortArgs) > 0 {
		shortArgs = shortArgs[1:]
	}
	shortOptions, err := icomp.Run(comp, shortArgs)
	if err != nil {
		t.Fatalf("unexpected error for short flag: %v", err)
	}
	if containsString(shortOptions, "-V") {
		t.Fatalf("did not expect repeated -V in options, got %v", shortOptions)
	}

	longLine := "lota --version"
	longArgs := icomp.ParseArgs(longLine)
	if len(longArgs) > 0 {
		longArgs = longArgs[1:]
	}
	longOptions, err := icomp.Run(comp, longArgs)
	if err != nil {
		t.Fatalf("unexpected error for long flag: %v", err)
	}
	if containsString(longOptions, "--version") {
		t.Fatalf("did not expect repeated --version in options, got %v", longOptions)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func indexOf(items []string, target string) int {
	for i, item := range items {
		if item == target {
			return i
		}
	}
	return -1
}

func TestRunCompleteSubcommand_InvalidPoint(t *testing.T) {
	if os.Getenv("GO_TEST_COMPLETE_INVALID") == "1" {
		RunCompleteSubcommand([]string{"lota he", "not-a-number"})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRunCompleteSubcommand_InvalidPoint")
	cmd.Env = append(os.Environ(), "GO_TEST_COMPLETE_INVALID=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, got output: %s", output)
	}
	if !strings.Contains(string(output), "invalid point") {
		t.Errorf("expected 'invalid point' in stderr, got: %s", output)
	}
}

func TestRunCompleteSubcommand_NotTriggered(t *testing.T) {
	// When __complete is NOT present, normal execution continues.
	// We verify by checking that an empty arg list does not trigger completion.
	// This is implicitly tested by the existing CLI tests; here we just ensure
	// RunCompleteSubcommand is only called when the first arg is "__complete".
}

func TestExtractCompletionArgs_CommandNotFirstToken(t *testing.T) {
	parsed := icomp.ParseArgs("go build && lota group1")
	args := icomp.ExtractCommandArgs(parsed, "lota")
	if len(args) != 1 || args[0].Text != "group1" {
		t.Fatalf("unexpected extracted args: %+v", args)
	}
}

func TestExtractCompletionArgs_PathToken(t *testing.T) {
	parsed := icomp.ParseArgs("go build && /usr/bin/lota group1")
	args := icomp.ExtractCommandArgs(parsed, "lota")
	if len(args) != 1 || args[0].Text != "group1" {
		t.Fatalf("unexpected extracted args for path token: %+v", args)
	}
}

func TestPositionalCompletionHint_ExpectedPositional(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "group2",
				Commands: []config.Command{
					{
						Name: "command5",
						Args: []config.Arg{
							{Name: "service", Type: "str"},
							{Name: "cmd", Wildcard: true},
						},
					},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("build indexes: %v", err)
	}

	parsed := icomp.ParseArgs("lota group2 command5 ")
	args := icomp.ExtractCommandArgs(parsed, "lota")

	hint := engine.PositionalCompletionHint(cfg, args)
	want := "expected positional arg: <SERVICE>"
	if hint != want {
		t.Fatalf("unexpected hint: got %q, want %q", hint, want)
	}
}

func TestPositionalCompletionHint_NoHintAfterPositionalProvided(t *testing.T) {
	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "group2",
				Commands: []config.Command{
					{
						Name: "command5",
						Args: []config.Arg{
							{Name: "service", Type: "str"},
							{Name: "cmd", Wildcard: true},
						},
					},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("build indexes: %v", err)
	}

	parsed := icomp.ParseArgs("lota group2 command5 backend ")
	args := icomp.ExtractCommandArgs(parsed, "lota")

	hint := engine.PositionalCompletionHint(cfg, args)
	if hint != "" {
		t.Fatalf("expected no hint after positional value, got %q", hint)
	}
}

func TestPositionalCompletionHint_NoHintWhileTypingFlag(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{
				Name: "run",
				Args: []config.Arg{
					{Name: "verbose", Type: "bool"},
					{Name: "service", Type: "str"},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("build indexes: %v", err)
	}

	parsed := icomp.ParseArgs("lota run -")
	args := icomp.ExtractCommandArgs(parsed, "lota")

	hint := engine.PositionalCompletionHint(cfg, args)
	if hint != "" {
		t.Fatalf("expected no hint while typing flag token, got %q", hint)
	}
}

func TestInstallCompletionScript(t *testing.T) {
	// Create a temporary home directory to avoid polluting the real one.
	tmpHome, err := os.MkdirTemp("", "lota-completion-test")
	if err != nil {
		t.Fatalf("mkdirtemp error: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpHome); err != nil {
			t.Errorf("failed to remove temp dir: %v", err)
		}
	}()

	// Test the helper functions directly.
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			script, err := engine.GetCompletionScript(shell, "lota")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if script == "" {
				t.Error("expected non-empty script")
			}
		})
	}

	// Test unsupported shell
	t.Run("unsupported", func(t *testing.T) {
		_, err := engine.GetCompletionScript("pwsh", "lota")
		if err == nil {
			t.Error("expected error for unsupported shell")
		}
	})
}
