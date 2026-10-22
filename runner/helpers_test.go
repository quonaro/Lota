package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quonaro/lota/config"
)

func TestPrefixWriter(t *testing.T) {
	var buf bytes.Buffer
	pw := &PrefixWriter{Writer: &buf, Prefix: "[task]"}

	// Partial line stays buffered until newline.
	if _, err := pw.Write([]byte("hel")); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected buffered partial line, got %q", buf.String())
	}
	if _, err := pw.Write([]byte("lo\nworld\n")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "[task] hello\n[task] world\n" {
		t.Errorf("got %q", got)
	}
	// Flush emits the trailing unterminated line.
	if _, err := pw.Write([]byte("tail")); err != nil {
		t.Fatal(err)
	}
	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.HasSuffix(got, "[task] tail\n") {
		t.Errorf("got %q", got)
	}
	// Flush on empty buffer is a no-op.
	before := buf.String()
	if err := pw.Flush(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != before {
		t.Error("second Flush should be a no-op")
	}
}

func TestPrefixWriter_CRLF(t *testing.T) {
	var buf bytes.Buffer
	pw := &PrefixWriter{Writer: &buf, Prefix: ">"}
	if _, err := pw.Write([]byte("line\r\nnext")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "> line\n" {
		t.Errorf("CRLF should normalize to LF, got %q", got)
	}
}

func TestSplitShellCommand(t *testing.T) {
	tests := []struct {
		in   string
		want []string
		err  bool
	}{
		{"bash -c", []string{"bash", "-c"}, false},
		{"bash", []string{"bash"}, false},
		{"cmd /c", []string{"cmd", "/c"}, false},
		{"powershell -NoProfile -Command", []string{"powershell", "-NoProfile", "-Command"}, false},
		{`bash -c "quoted arg"`, []string{"bash", "-c", "quoted arg"}, false},
		{`sh -c 'single'`, []string{"sh", "-c", "single"}, false},
		{"unclosed \"quote", nil, true},
		{"", nil, false},
	}
	for _, tt := range tests {
		got, err := splitShellCommand(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("splitShellCommand(%q): expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitShellCommand(%q): %v", tt.in, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("splitShellCommand(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitShellCommand(%q) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

func TestSummarizeShellCommand(t *testing.T) {
	if got := summarizeShellCommand([]string{"bash", "-c"}, "  echo   hi\n  echo   bye  "); got != "echo hi echo bye" {
		t.Errorf("got %q", got)
	}
	// Empty script: falls back to the shell parts.
	if got := summarizeShellCommand([]string{"bash", "-c"}, "   "); got != "bash -c" {
		t.Errorf("got %q", got)
	}
	// Long scripts are truncated to 80 chars + ellipsis.
	long := strings.Repeat("x", 200)
	if got := summarizeShellCommand([]string{"bash"}, long); len(got) != 83 || !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncated summary, got len=%d", len(got))
	}
}

func TestSortedMapKeys(t *testing.T) {
	got := sortedMapKeys(map[string]string{"b": "1", "a": "2", "c": "3"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("got %v", got)
	}
	if got := sortedMapKeys(nil); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestShellError_Error(t *testing.T) {
	err := &ShellError{ExitCode: 3, Command: "deploy"}
	if got := err.Error(); !strings.Contains(got, "3") || !strings.Contains(got, "deploy") {
		t.Errorf("got %q", got)
	}
}

func TestSuggestSimilarFlag(t *testing.T) {
	defs := map[string]*config.Arg{
		"--output": {Name: "output"},
		"--force":  {Name: "force"},
	}
	if got := suggestSimilarFlag("--outpu", defs); got != "--output" {
		t.Errorf("got %q, want --output", got)
	}
	if got := suggestSimilarFlag("--completely-different", defs); got != "" {
		t.Errorf("expected no suggestion, got %q", got)
	}
	if got := suggestSimilarFlag("--x", map[string]*config.Arg{}); got != "" {
		t.Errorf("expected empty for no defs, got %q", got)
	}
}

func TestValidBooleanValues(t *testing.T) {
	got := validBooleanValues()
	for _, v := range []string{"true", "false", "yes", "no"} {
		if !strings.Contains(got, v) {
			t.Errorf("missing %q in %q", v, got)
		}
	}
}

func TestParseBoolValue(t *testing.T) {
	trues := []string{"true", "TRUE", "1", "yes", "on"}
	for _, v := range trues {
		if got, err := parseBoolValue(v); err != nil || got != "true" {
			t.Errorf("parseBoolValue(%q) = %q, %v", v, got, err)
		}
	}
	falses := []string{"false", "FALSE", "0", "no", "off"}
	for _, v := range falses {
		if got, err := parseBoolValue(v); err != nil || got != "false" {
			t.Errorf("parseBoolValue(%q) = %q, %v", v, got, err)
		}
	}
	// Negation prefix.
	if got, err := parseBoolValue("!true"); err != nil || got != "false" {
		t.Errorf("parseBoolValue(!true) = %q, %v", got, err)
	}
	if got, err := parseBoolValue("!false"); err != nil || got != "true" {
		t.Errorf("parseBoolValue(!false) = %q, %v", got, err)
	}
	if got, err := parseBoolValue("!other"); err != nil || got != "false" {
		t.Errorf("parseBoolValue(!other) = %q, %v", got, err)
	}
	// Invalid.
	if _, err := parseBoolValue("maybe"); err == nil {
		t.Error("expected error for invalid value")
	}
}

func TestResolveShell(t *testing.T) {
	app := config.AppConfig{Shell: "sh"}
	groups := []*config.Group{{Shell: "dash"}, {Shell: "zsh"}}
	cmd := config.Command{Shell: "fish"}

	// Command wins over group and app.
	if got := ResolveShell(app, groups, cmd); got != "fish -c" {
		t.Errorf("got %q", got)
	}
	// Innermost group wins when command has none.
	if got := ResolveShell(app, groups, config.Command{}); got != "zsh -c" {
		t.Errorf("got %q", got)
	}
	// App shell when groups don't define one.
	if got := ResolveShell(app, nil, config.Command{}); got != "sh -c" {
		t.Errorf("got %q", got)
	}
	// Default when nothing is set.
	got := ResolveShell(config.AppConfig{}, nil, config.Command{})
	if got == "" || !strings.HasSuffix(got, "-c") {
		t.Errorf("expected default shell with -c, got %q", got)
	}
}

func TestNormalizeShell(t *testing.T) {
	tests := map[string]string{
		"bash":           "bash -c",
		"sh":             "sh -c",
		"fish":           "fish -c",
		"powershell.exe": "powershell.exe -NoProfile -ExecutionPolicy Bypass -Command",
		"pwsh":           "pwsh -NoProfile -ExecutionPolicy Bypass -Command",
		"cmd":            "cmd /c",
		"cmd.exe":        "cmd.exe /c",
		"custom-shell":   "custom-shell",
	}
	for in, want := range tests {
		if got := normalizeShell(in); got != want {
			t.Errorf("normalizeShell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStandardShell(t *testing.T) {
	for _, osName := range []string{"windows", "linux", "darwin"} {
		got := standardShell(&osName)
		if got == "" {
			t.Errorf("standardShell(%s) returned empty", osName)
		}
	}
	win := "windows"
	if got := standardShell(&win); !strings.Contains(got, "powershell") {
		t.Errorf("windows should use powershell, got %q", got)
	}
	linux := "linux"
	if got := standardShell(&linux); !strings.HasPrefix(got, "bash") {
		t.Errorf("linux should use bash, got %q", got)
	}
	// Unknown OS → empty result from the map, normalized to itself.
	plan9 := "plan9"
	if got := standardShell(&plan9); got != "" && !strings.Contains(got, "plan9") {
		t.Errorf("unexpected shell for plan9: %q", got)
	}
}

func TestIsValidShellIdentifier(t *testing.T) {
	valid := []string{"FOO", "_bar", "a1", "_"}
	for _, v := range valid {
		if !isValidShellIdentifier(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"", "1abc", "a-b", "a b", "a$"}
	for _, v := range invalid {
		if isValidShellIdentifier(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestExtractIdentifier(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"FOO=1", "FOO", true},
		{"  BAR  ", "BAR", true},
		{"_x9+", "_x9", true},
		{"1abc", "", false},
		{"", "", false},
		{"   ", "", false},
		{"-x", "", false},
	}
	for _, tt := range tests {
		got, ok := extractIdentifier(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("extractIdentifier(%q) = %q,%v want %q,%v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTrimQuotes(t *testing.T) {
	if got := trimQuotes(`"abc"`); got != "abc" {
		t.Errorf("got %q", got)
	}
	if got := trimQuotes(`abc`); got != "abc" {
		t.Errorf("got %q", got)
	}
	// Only double-quoted strings are trimmed.
	if got := trimQuotes(`'abc'`); got != "'abc'" {
		t.Errorf("got %q", got)
	}
}

func TestInterpolateInt(t *testing.T) {
	if got, err := interpolateInt("N", "42"); err != nil || got != "42" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := interpolateInt("N", ""); err != nil || got != "0" {
		t.Errorf("empty should be 0, got %q, %v", got, err)
	}
	if got, err := interpolateInt("N", `"17"`); err != nil || got != "17" {
		t.Errorf("quoted value should unwrap, got %q, %v", got, err)
	}
	if _, err := interpolateInt("N", "abc"); err == nil {
		t.Error("expected error for non-integer")
	}
}

func TestInterpolateBool(t *testing.T) {
	if got, err := interpolateBool("B", "yes"); err != nil || got != "true" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := interpolateBool("B", ""); err != nil || got != "false" {
		t.Errorf("empty should be false, got %q, %v", got, err)
	}
	if _, err := interpolateBool("B", "maybe"); err == nil {
		t.Error("expected error for invalid bool")
	}
}

func TestInterpolateArray(t *testing.T) {
	if got, err := interpolateArray("a,b, c "); err != nil || got != "a b c" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := interpolateArray(""); err != nil || got != "" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := interpolateArray(`"x,y"`); err != nil || got != "x y" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{Placeholder: "$FOO", Reason: "x"}
	if got := err.Error(); !strings.Contains(got, "$FOO") {
		t.Errorf("got %q", got)
	}
}

func TestOpenLogFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Dry run must not create the file.
	f, err := openLogFile(path, false, true, nil)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if f != nil {
		_ = f.Close()
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("dry-run should not create log file")
	}

	// Real open creates the file.
	f, err = openLogFile(path, false, false, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString("first\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// Truncate clears previous content.
	f, err = openLogFile(path, true, false, nil)
	if err != nil {
		t.Fatalf("truncate open: %v", err)
	}
	_ = f.Close()
	data, _ := os.ReadFile(path)
	if len(data) != 0 {
		t.Errorf("expected truncated file, got %q", data)
	}
}

func TestRunCheck(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	if err := RunCheck(ctx, "exit 0", nil, "bash -c", dir, "", ""); err != nil {
		t.Errorf("exit 0 should succeed: %v", err)
	}
	if err := RunCheck(ctx, "exit 3", nil, "bash -c", dir, "", ""); err == nil {
		t.Error("exit 3 should fail")
	}
	// Working dir is honored.
	if err := RunCheck(ctx, "test -d sub", nil, "bash -c", dir, "", ""); err == nil {
		t.Error("expected failure: sub dir does not exist")
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := RunCheck(ctx, "test -d sub", nil, "bash -c", dir, "", ""); err != nil {
		t.Errorf("expected success in dir: %v", err)
	}
	// Empty shell string errors.
	if err := RunCheck(ctx, "true", nil, "", dir, "", ""); err == nil {
		t.Error("expected error for empty shell")
	}
	// Cancelled context kills the check.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := RunCheck(cancelled, "sleep 5", nil, "bash -c", dir, "", ""); err == nil {
		t.Error("expected error for cancelled context")
	}
}
