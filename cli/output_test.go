package cli

import (
	"testing"

	"github.com/quonaro/lota/config"
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
