package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/shared"
)

func TestHashColor(t *testing.T) {
	c1 := hashColor("build")
	c2 := hashColor("build")
	if c1 != c2 {
		t.Errorf("hashColor not deterministic: %q vs %q", c1, c2)
	}
	c3 := hashColor("test")
	if c1 == c3 {
		t.Errorf("different inputs should produce different colors: %q vs %q", c1, c3)
	}
	if len(c1) != 7 || c1[0] != '#' {
		t.Errorf("expected 7-char hex color starting with #, got %q", c1)
	}
}

func TestIsFlagArg(t *testing.T) {
	tests := []struct {
		name     string
		arg      config.Arg
		expected bool
	}{
		{
			name:     "plain positional",
			arg:      config.Arg{Name: "filename", Type: "str"},
			expected: false,
		},
		{
			name:     "positional with type int",
			arg:      config.Arg{Name: "count", Type: "int"},
			expected: false,
		},
		{
			name:     "short alias makes flag",
			arg:      config.Arg{Name: "output", Short: "o", Type: "str"},
			expected: true,
		},
		{
			name:     "bool type makes flag",
			arg:      config.Arg{Name: "verbose", Type: "bool"},
			expected: true,
		},
		{
			name:     "default value is still positional",
			arg:      config.Arg{Name: "env", Type: "str", Default: "dev"},
			expected: false,
		},
		{
			name:     "wildcard is positional",
			arg:      config.Arg{Name: "cmd", Wildcard: true},
			expected: false,
		},
		{
			name:     "wildcard with default is still positional",
			arg:      config.Arg{Name: "cmd", Wildcard: true, Default: "default"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFlagArg(tt.arg)
			if got != tt.expected {
				t.Errorf("isFlagArg(%+v) = %v, want %v", tt.arg, got, tt.expected)
			}
		})
	}
}

func TestBuildCommandUsage(t *testing.T) {
	positional := []config.Arg{
		{Name: "service", Required: true, Type: "str"},
		{Name: "env", Required: false, Type: "str"},
		{Name: "files", Type: "arr"},
		{Name: "tail", Wildcard: true},
	}
	flags := []config.Arg{{Name: "verbose", Type: "bool"}}

	got := buildCommandUsage("infra deploy", positional, flags)
	want := "Usage: lota infra deploy [OPTIONS] <SERVICE> [<ENV>] [<FILES...>] [...<TAIL>]"
	if got != want {
		t.Fatalf("buildCommandUsage() = %q, want %q", got, want)
	}
}

func TestUsageArgName(t *testing.T) {
	if got := usageArgName("service-name"); got != "SERVICE_NAME" {
		t.Fatalf("usageArgName() = %q, want %q", got, "SERVICE_NAME")
	}
	if got := usageArgName("!!!"); got != "ARG" {
		t.Fatalf("usageArgName() = %q, want %q", got, "ARG")
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
		{
			name:     "direct color wins",
			objColor: "red",
			want:     "red",
		},
		{
			name:         "inherit from closest ancestor",
			inheritColor: &trueVal,
			ancestors: []*config.Group{
				{Name: "outer", Color: "blue"},
				{Name: "inner", Color: "green"},
			},
			want: "green",
		},
		{
			name:         "inherit skips ancestor without color",
			inheritColor: &trueVal,
			ancestors: []*config.Group{
				{Name: "outer", Color: "blue"},
				{Name: "inner", Color: ""},
			},
			want: "blue",
		},
		{
			name:         "no inheritance when not set",
			objColor:     "",
			inheritColor: nil,
			ancestors: []*config.Group{
				{Name: "g", Color: "yellow"},
			},
			want: "",
		},
		{
			name:         "inherit disabled explicitly",
			objColor:     "",
			inheritColor: &falseVal,
			ancestors: []*config.Group{
				{Name: "g", Color: "yellow"},
			},
			want: "",
		},
		{
			name:         "empty ancestors with inherit",
			objColor:     "",
			inheritColor: &trueVal,
			ancestors:    []*config.Group{},
			want:         "",
		},
		{
			name:     "inherit via ancestor flag",
			objColor: "",
			ancestors: []*config.Group{
				{Name: "parent", Color: "yellow", InheritColor: &trueVal},
			},
			want: "yellow",
		},
		{
			name:     "ancestor inherit false blocks",
			objColor: "",
			ancestors: []*config.Group{
				{Name: "outer", Color: "yellow", InheritColor: &trueVal},
				{Name: "inner", Color: "", InheritColor: &falseVal},
			},
			want: "",
		},
		{
			name:         "own inherit true overrides ancestor false",
			objColor:     "",
			inheritColor: &trueVal,
			ancestors: []*config.Group{
				{Name: "parent", Color: "yellow", InheritColor: &falseVal},
			},
			want: "yellow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveColor(tt.objColor, tt.inheritColor, tt.ancestors)
			if got != tt.want {
				t.Errorf("resolveColor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColorize(t *testing.T) {
	// colorize with empty colorName returns text unchanged
	if got := colorize("hello", ""); got != "hello" {
		t.Errorf("colorize(hello, \"\") = %q, want %q", got, "hello")
	}
	// colorize with invalid colorName returns text unchanged
	if got := colorize("hello", "invalid"); got != "hello" {
		t.Errorf("colorize(hello, invalid) = %q, want %q", got, "hello")
	}
	// colorize with valid colorName returns non-empty text (ANSI may be disabled in non-TTY)
	got := colorize("hello", "red")
	if got == "" {
		t.Error("colorize(hello, red) should return non-empty text")
	}
	// colorize with hex color returns non-empty text (ANSI may be disabled in non-TTY)
	if gotHex := colorize("hello", "#FF0000"); gotHex == "" {
		t.Error("colorize(hello, \"#FF0000\") should return non-empty text")
	}
	// colorize with invalid hex falls back to plain text
	if gotBad := colorize("hello", "#GGGGGG"); gotBad != "hello" {
		t.Errorf("colorize(hello, \"#GGGGGG\") = %q, want plain \"hello\"", gotBad)
	}
}

func TestIsHidden(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		show     *bool
		expected bool
	}{
		{"nil show", nil, false},
		{"show true", &trueVal, false},
		{"show false", &falseVal, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHidden(tt.show)
			if got != tt.expected {
				t.Errorf("isHidden(%v) = %v, want %v", tt.show, got, tt.expected)
			}
		})
	}
}

func TestSeparateArgs(t *testing.T) {
	args := []config.Arg{
		{Name: "filename", Type: "str"},            // positional
		{Name: "output", Short: "o", Type: "str"},  // flag
		{Name: "verbose", Type: "bool"},            // flag
		{Name: "env", Type: "str", Default: "dev"}, // positional (has default)
		{Name: "cmd", Wildcard: true},              // positional
	}

	positional, flags := separateArgs(args)

	if len(positional) != 3 {
		t.Errorf("expected 3 positional args, got %d", len(positional))
	}
	if len(flags) != 2 {
		t.Errorf("expected 2 flag args, got %d", len(flags))
	}

	expectedPositional := []string{"filename", "env", "cmd"}
	for i, name := range expectedPositional {
		if positional[i].Name != name {
			t.Errorf("positional[%d].Name = %q, want %q", i, positional[i].Name, name)
		}
	}

	expectedFlags := []string{"output", "verbose"}
	for i, name := range expectedFlags {
		if flags[i].Name != name {
			t.Errorf("flags[%d].Name = %q, want %q", i, flags[i].Name, name)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	_ = w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(out)
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		in      string
		r, g, b uint8
		ok      bool
	}{
		{"#FF8000", 0xFF, 0x80, 0x00, true},
		{"#000000", 0, 0, 0, true},
		{"#ffffff", 0xFF, 0xFF, 0xFF, true},
		{"red", 0, 0, 0, false},
		{"#FFF", 0, 0, 0, false},
		{"#GGGGGG", 0, 0, 0, false},
		{"FF8000", 0, 0, 0, false},
		{"", 0, 0, 0, false},
	}
	for _, tt := range tests {
		r, g, b, ok := parseHexColor(tt.in)
		if ok != tt.ok || (ok && (r != tt.r || g != tt.g || b != tt.b)) {
			t.Errorf("parseHexColor(%q) = %d,%d,%d,%v", tt.in, r, g, b, ok)
		}
	}
}

func TestPaddedName(t *testing.T) {
	got := paddedName("build", "", 10)
	if got != "build     " {
		t.Errorf("paddedName = %q, want %q", got, "build     ")
	}
	// Name longer than width: no padding, no truncation.
	got = paddedName("averylongcommandname", "", 5)
	if got != "averylongcommandname" {
		t.Errorf("paddedName = %q", got)
	}
	// Colored name: padding based on visible length.
	got = paddedName("x", "red", 4)
	if !strings.HasSuffix(got, "   ") {
		t.Errorf("expected trailing padding, got %q", got)
	}
}

func TestBuildCommandName(t *testing.T) {
	cmd := config.Command{Name: "up"}
	if got := buildCommandName(cmd, nil); got != "up" {
		t.Errorf("got %q", got)
	}
	groups := []*config.Group{{Name: "infra"}, {Name: "db"}}
	if got := buildCommandName(cmd, groups); got != "infra db up" {
		t.Errorf("got %q", got)
	}
}

func TestDetermineArgScope(t *testing.T) {
	cfg := config.AppConfig{
		Args: []config.Arg{{Name: "global-arg"}},
	}
	groups := []*config.Group{
		{Args: []config.Arg{{Name: "group-arg"}}},
	}
	cmd := config.Command{
		Name: "c",
		Args: []config.Arg{{Name: "cmd-arg"}},
	}

	tests := []struct {
		arg  config.Arg
		want string
	}{
		{config.Arg{Name: "cmd-arg"}, "Command"},
		{config.Arg{Name: "group-arg"}, "Group"},
		{config.Arg{Name: "global-arg"}, "Global"},
		{config.Arg{Name: "unknown"}, "Unknown"},
	}
	for _, tt := range tests {
		if got := determineArgScope(tt.arg, cfg, groups, cmd); got != tt.want {
			t.Errorf("determineArgScope(%q) = %q, want %q", tt.arg.Name, got, tt.want)
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
		{config.Arg{Name: "a", Type: "arr"}, "[<A...>]"},
		{config.Arg{Name: "rest", Wildcard: true}, "[...<REST>]"},
		{config.Arg{Name: "", Required: true}, "<ARG>"},
	}
	for _, tt := range tests {
		if got := formatPositionalUsage(tt.arg); got != tt.want {
			t.Errorf("formatPositionalUsage(%+v) = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

// captureColorOutput redirects fatih/color's static Output writer (used by
// PrintError/PrintVersion) into a buffer for the duration of fn.
func captureColorOutput(t *testing.T, fn func()) string {
	t.Helper()
	var buf strings.Builder
	old := color.Output
	color.Output = &buf
	defer func() { color.Output = old }()
	fn()
	return buf.String()
}

func TestPrintError(t *testing.T) {
	out := captureColorOutput(t, func() { PrintError("something broke") })
	if !strings.Contains(out, "something broke") {
		t.Errorf("PrintError output = %q", out)
	}
	out = captureColorOutput(t, func() { PrintErrorf("code %d", 3) })
	if !strings.Contains(out, "code 3") {
		t.Errorf("PrintErrorf output = %q", out)
	}
}

func TestPrintVersion(t *testing.T) {
	out := captureStdout(t, PrintVersionShort)
	if !strings.Contains(out, shared.Version) {
		t.Errorf("PrintVersionShort output = %q, want to contain %q", out, shared.Version)
	}
	out = captureColorOutput(t, PrintVersion)
	if !strings.Contains(out, shared.Version) {
		t.Errorf("PrintVersion output = %q, want to contain %q", out, shared.Version)
	}
}

func TestInitConfig(t *testing.T) {
	dir := t.TempDir()

	// Explicit file path.
	path := filepath.Join(dir, "custom.yml")
	if err := InitConfig(path); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not created: %v", err)
	}

	// Existing file errors.
	if err := InitConfig(path); err == nil {
		t.Error("expected error for existing file")
	}

	// Directory path: writes lota.yml inside.
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := InitConfig(sub); err != nil {
		t.Fatalf("InitConfig(dir): %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, shared.ConfigFileName)); err != nil {
		t.Fatalf("expected %s in dir: %v", shared.ConfigFileName, err)
	}

	// Empty path: writes to cwd.
	t.Chdir(dir)
	captureStdout(t, func() {
		if err := InitConfig(""); err != nil {
			t.Errorf("InitConfig(default): %v", err)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, shared.ConfigFileName)); err != nil {
		t.Errorf("expected default config: %v", err)
	}
}

func TestPrintHelp_NoConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.yml")
	out := captureStdout(t, func() { PrintHelp(missing) })
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' message, got %q", out)
	}
	if !strings.Contains(out, "--init") {
		t.Errorf("expected global options to be printed, got %q", out)
	}
}

func TestPrintHelp_WithConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lota.yml")
	content := `vars:
- X=1
hello:
  desc: Say hi
  script: echo hi
group1:
  desc: A group
  hidden:
    show: false
    script: echo secret
  inner:
    script: echo inner
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { PrintHelp(path) })
	if !strings.Contains(out, "hello") || !strings.Contains(out, "Say hi") {
		t.Errorf("expected command in help, got %q", out)
	}
	if !strings.Contains(out, "group1") {
		t.Errorf("expected group in help, got %q", out)
	}
}

func TestPrintCommandHelp(t *testing.T) {
	cfg := &config.AppConfig{
		Args: []config.Arg{{Name: "global-flag", Type: "bool"}},
		Commands: []config.Command{
			{
				Name:   "deploy",
				Desc:   "Deploy the app",
				Script: "echo deploy",
				Args:   []config.Arg{{Name: "env", Required: true, Type: "str"}, {Name: "force", Short: "f", Type: "bool"}},
				Dir:    "/srv",
				Before: "echo before",
				After:  "echo after",
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatal(err)
	}
	res := cfg.Find("deploy")

	out := captureStdout(t, func() { PrintCommandHelp(cfg, res, false) })
	for _, want := range []string{"Usage:", "deploy", "Deploy the app", "env", "--force"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}

	// Verbose adds scope markers and script sections.
	out = captureStdout(t, func() { PrintCommandHelp(cfg, res, true) })
	for _, want := range []string{"[from:", "Script:", "Before hook:", "After hook:", "Directory:", "Options:"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose help missing %q:\n%s", want, out)
		}
	}

	// Nil command is a no-op.
	out = captureStdout(t, func() { PrintCommandHelp(cfg, config.SearchResult{}, false) })
	if out != "" {
		t.Errorf("expected no output for nil command, got %q", out)
	}
}

func TestPrintGroupHelp(t *testing.T) {
	show := true
	_ = show
	hidden := false
	group := &config.Group{
		Name: "infra",
		Desc: "Infra tasks",
		Vars: []config.Var{{Name: "ENV", Value: "prod"}},
		Args: []config.Arg{
			{Name: "target", Type: "str"},
			{Name: "dry", Short: "d", Type: "bool"},
		},
		Groups: []config.Group{{Name: "db", Desc: "DB tasks"}},
		Commands: []config.Command{
			{Name: "up", Desc: "Start infra"},
			{Name: "secret", Show: &hidden},
		},
	}
	if err := group.BuildIndexes(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() { PrintGroupHelp(group, nil, false) })
	for _, want := range []string{"Infra tasks", "up", "db", "target", "--dry"} {
		if !strings.Contains(out, want) {
			t.Errorf("group help missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Error("hidden command should not be listed")
	}

	out = captureStdout(t, func() { PrintGroupHelp(group, nil, true) })
	if !strings.Contains(out, "ENV=prod") || !strings.Contains(out, "Variables:") {
		t.Errorf("verbose group help should list vars:\n%s", out)
	}
}
