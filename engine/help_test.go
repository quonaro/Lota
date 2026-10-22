package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quonaro/lota/config"
	icomp "github.com/quonaro/lota/internal/complete"
)

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		in      string
		r, g, b uint8
		ok      bool
	}{
		{"#FF8000", 0xFF, 0x80, 0x00, true},
		{"#ffffff", 0xFF, 0xFF, 0xFF, true},
		{"#000000", 0, 0, 0, true},
		{"red", 0, 0, 0, false},
		{"#FFF", 0, 0, 0, false},
		{"#GGGGGG", 0, 0, 0, false},
		{"", 0, 0, 0, false},
	}
	for _, tt := range tests {
		r, g, b, ok := parseHexColor(tt.in)
		if ok != tt.ok || (ok && (r != tt.r || g != tt.g || b != tt.b)) {
			t.Errorf("parseHexColor(%q) = %d,%d,%d,%v", tt.in, r, g, b, ok)
		}
	}
}

func TestColorize(t *testing.T) {
	if got := colorize("text", ""); got != "text" {
		t.Errorf("empty color should return text as-is, got %q", got)
	}
	if got := colorize("text", "notacolor"); got != "text" {
		t.Errorf("unknown color should return text as-is, got %q", got)
	}
	// Valid names produce output (possibly with ANSI codes).
	if got := colorize("text", "red"); !strings.Contains(got, "text") {
		t.Errorf("expected output to contain text, got %q", got)
	}
	if got := colorize("text", "#FF8000"); !strings.Contains(got, "text") {
		t.Errorf("expected output to contain text, got %q", got)
	}
	// Case-insensitive color names.
	if got := colorize("text", "RED"); !strings.Contains(got, "text") {
		t.Errorf("expected output to contain text, got %q", got)
	}
}

func TestResolveColor(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name         string
		objColor     string
		inheritColor *bool
		ancestors    []*config.Group
		want         string
	}{
		{name: "own color wins", objColor: "red", want: "red"},
		{name: "no color no inherit", want: ""},
		{
			name:         "inherit disabled",
			inheritColor: &falseVal,
			ancestors:    []*config.Group{{Color: "blue"}},
			want:         "",
		},
		{
			name:         "inherit enabled takes closest ancestor color",
			inheritColor: &trueVal,
			ancestors:    []*config.Group{{Color: "blue"}, {Color: "green"}},
			want:         "green",
		},
		{
			name:      "ancestor allows inheritance",
			ancestors: []*config.Group{{Color: "blue", InheritColor: &trueVal}},
			want:      "blue",
		},
		{
			name:      "ancestor forbids inheritance",
			ancestors: []*config.Group{{Color: "blue", InheritColor: &falseVal}},
			want:      "",
		},
		{
			name:         "inherit enabled but no ancestor color",
			inheritColor: &trueVal,
			ancestors:    []*config.Group{{}},
			want:         "",
		},
	}
	for _, tt := range tests {
		if got := resolveColor(tt.objColor, tt.inheritColor, tt.ancestors); got != tt.want {
			t.Errorf("%s: resolveColor() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPaddedName(t *testing.T) {
	if got := paddedName("build", "", 10); got != "build     " {
		t.Errorf("paddedName = %q", got)
	}
	if got := paddedName("toolongname", "", 4); got != "toolongname" {
		t.Errorf("paddedName should not truncate, got %q", got)
	}
}

func TestIsHidden(t *testing.T) {
	show := true
	hide := false
	if isHidden(nil) {
		t.Error("nil Show should not be hidden")
	}
	if isHidden(&show) {
		t.Error("show=true should not be hidden")
	}
	if !isHidden(&hide) {
		t.Error("show=false should be hidden")
	}
}

func TestIsFlagArg(t *testing.T) {
	tests := []struct {
		arg  config.Arg
		want bool
	}{
		{config.Arg{Name: "pos", Type: "str"}, false},
		{config.Arg{Name: "n", Type: "int"}, false},
		{config.Arg{Name: "out", Short: "o"}, true},
		{config.Arg{Name: "v", Type: "bool"}, true},
		{config.Arg{Name: "rest", Wildcard: true, Short: "r"}, false},
	}
	for _, tt := range tests {
		if got := isFlagArg(tt.arg); got != tt.want {
			t.Errorf("isFlagArg(%+v) = %v, want %v", tt.arg, got, tt.want)
		}
	}
}

func TestSeparateArgs(t *testing.T) {
	args := []config.Arg{
		{Name: "file", Type: "str"},
		{Name: "verbose", Type: "bool"},
		{Name: "out", Short: "o"},
		{Name: "rest", Wildcard: true},
	}
	pos, flags := separateArgs(args)
	if len(pos) != 2 || pos[0].Name != "file" || pos[1].Name != "rest" {
		t.Errorf("positional = %v", pos)
	}
	if len(flags) != 2 || flags[0].Name != "verbose" || flags[1].Name != "out" {
		t.Errorf("flags = %v", flags)
	}
}

func TestUsageArgName(t *testing.T) {
	tests := map[string]string{
		"service-name": "SERVICE_NAME",
		"plain":        "PLAIN",
		"a.b-c_d":      "A_B_C_D",
		"!!!":          "ARG",
		"":             "ARG",
		"--x--":        "X",
	}
	for in, want := range tests {
		if got := usageArgName(in); got != want {
			t.Errorf("usageArgName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatPositionalUsage(t *testing.T) {
	tests := []struct {
		arg  config.Arg
		want string
	}{
		{config.Arg{Name: "a", Required: true}, "<A>"},
		{config.Arg{Name: "a"}, "[<A>]"},
		{config.Arg{Name: "a", Type: "arr", Required: true}, "<A...>"},
		{config.Arg{Name: "rest", Wildcard: true}, "[...<REST>]"},
	}
	for _, tt := range tests {
		if got := formatPositionalUsage(tt.arg); got != tt.want {
			t.Errorf("formatPositionalUsage(%+v) = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

func TestPositionalPlaceholder(t *testing.T) {
	if got := positionalPlaceholder("env-name"); got != "<ENV_NAME>" {
		t.Errorf("got %q", got)
	}
	if got := positionalPlaceholder("!!!"); got != "<ARG>" {
		t.Errorf("got %q", got)
	}
}

func TestIsFlagArgForCompletion(t *testing.T) {
	tests := []struct {
		arg  config.Arg
		want bool
	}{
		{config.Arg{Name: "pos", Type: "str"}, false},
		{config.Arg{Name: "out", Short: "o"}, true},
		{config.Arg{Name: "v", Type: "bool"}, true},
		{config.Arg{Name: "mode", Default: "dev"}, true},
		{config.Arg{Name: "rest", Wildcard: true, Default: "x"}, false},
	}
	for _, tt := range tests {
		if got := isFlagArgForCompletion(tt.arg); got != tt.want {
			t.Errorf("isFlagArgForCompletion(%+v) = %v, want %v", tt.arg, got, tt.want)
		}
	}
}

func TestAddArgFlag(t *testing.T) {
	cmd := BuildCompletion(&config.AppConfig{})
	addArgFlag(cmd, config.Arg{Name: "output", Short: "o"})
	if _, ok := cmd.Flags["output"]; !ok {
		t.Error("expected long flag to be registered")
	}
	if _, ok := cmd.Flags["o"]; !ok {
		t.Error("expected short flag to be registered")
	}
}

func TestAnythingPredictor(t *testing.T) {
	if got := predictAnything.Predict("any"); got != nil {
		t.Errorf("expected nil predictions, got %v", got)
	}
}

func TestGroupError_Error(t *testing.T) {
	err := &GroupError{Path: "infra.db"}
	msg := err.Error()
	if !strings.Contains(msg, "infra.db") || !strings.Contains(msg, "group") {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestNextPositionalHint(t *testing.T) {
	argDefs := []config.Arg{
		{Name: "env", Type: "str", Required: true},
		{Name: "tag", Type: "str"},
		{Name: "force", Type: "bool"},
		{Name: "out", Short: "o", Type: "str"},
	}

	// No args consumed: hint for the first positional.
	if got := nextPositionalHint(nil, argDefs); got != "expected positional arg: <ENV>" {
		t.Errorf("got %q", got)
	}

	// One positional consumed: hint for the second.
	args := icomp.ParseArgs("prod")
	if got := nextPositionalHint(args, argDefs); got != "expected positional arg: <TAG>" {
		t.Errorf("got %q", got)
	}

	// All positionals consumed, no wildcard: no hint.
	args = icomp.ParseArgs("prod v1")
	if got := nextPositionalHint(args, argDefs); got != "" {
		t.Errorf("expected no hint, got %q", got)
	}

	// Bool flag does not consume the next token.
	args = icomp.ParseArgs("--force prod")
	if got := nextPositionalHint(args, argDefs); got != "expected positional arg: <TAG>" {
		t.Errorf("got %q", got)
	}

	// Non-bool flag consumes the next token.
	args = icomp.ParseArgs("-o file prod v1")
	if got := nextPositionalHint(args, argDefs); got != "" {
		t.Errorf("expected no hint, got %q", got)
	}

	// Unknown flag → no hint.
	args = icomp.ParseArgs("--bogus prod")
	if got := nextPositionalHint(args, argDefs); got != "" {
		t.Errorf("expected no hint, got %q", got)
	}
}

func TestNextPositionalHint_Wildcard(t *testing.T) {
	argDefs := []config.Arg{
		{Name: "env", Type: "str"},
		{Name: "rest", Wildcard: true},
	}
	args := icomp.ParseArgs("prod a b c")
	if got := nextPositionalHint(args, argDefs); got != "" {
		t.Errorf("wildcard absorbs extras, want no hint, got %q", got)
	}
}

func TestPositionalCompletionHint_NoArgs(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{{Name: "build"}},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatal(err)
	}
	if got := PositionalCompletionHint(cfg, nil); got != "" {
		t.Errorf("expected empty hint, got %q", got)
	}
}

func TestNewBuilderFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yml")
	content := `hello:
  desc: greet
  script: echo hi
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := NewBuilderFromPath("testapp", path).Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestNewBuilderFromPath_Invalid(t *testing.T) {
	_, err := NewBuilderFromPath("testapp", filepath.Join(t.TempDir(), "missing.yml")).Build()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestApp_PrintGroupAndCommandHelp(t *testing.T) {
	data := []byte(`infra:
  desc: Infra tasks
  up:
    desc: Start infra
    script: echo up
hello:
  script: echo hi
`)
	var buf bytes.Buffer
	app, err := NewApp(data, Options{Stdout: &buf, Stderr: &buf})
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	// Command help via app method.
	res, _, _ := config.ResolveCommand(app.cfg, []string{"infra", "up"})
	buf.Reset()
	app.PrintCommandHelp(res, false)
	if !strings.Contains(buf.String(), "up") {
		t.Errorf("expected command help output, got %q", buf.String())
	}

	// Group help via app method.
	res, _, _ = config.ResolveCommand(app.cfg, []string{"infra"})
	buf.Reset()
	app.PrintGroupHelp(res.Groups)
	if !strings.Contains(buf.String(), "up") {
		t.Errorf("expected group help output, got %q", buf.String())
	}
}
