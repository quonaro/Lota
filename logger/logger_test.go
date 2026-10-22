package logger

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

func TestEnabled(t *testing.T) {
	if Enabled() != enabled {
		t.Errorf("Enabled() = %v, want %v", Enabled(), enabled)
	}
}

func TestLoggers_Disabled(t *testing.T) {
	old := enabled
	enabled = false
	defer func() { enabled = old }()

	out := captureStdout(t, func() {
		Debug("debug message")
		Debugf("debug %s", "message")
		Info("info message")
		Infof("info %s", "message")
		Error("error message")
		Errorf("error %s", "message")
		Warn("warn message")
		Warnf("warn %s", "message")
	})

	if out != "" {
		t.Errorf("expected no output when disabled, got %q", out)
	}
}

func TestLoggers_Enabled(t *testing.T) {
	old := enabled
	enabled = true
	defer func() { enabled = old }()

	tests := []struct {
		name     string
		fn       func()
		contains string
	}{
		{"Debug", func() { Debug("dbg") }, "dbg\n"},
		{"Debugf", func() { Debugf("number %d", 42) }, "number 42\n"},
		{"Info", func() { Info("inf") }, "inf\n"},
		{"Infof", func() { Infof("str %s", "x") }, "str x\n"},
		{"Error", func() { Error("err") }, "err\n"},
		{"Errorf", func() { Errorf("code %d", 1) }, "code 1\n"},
		{"Warn", func() { Warn("wrn") }, "wrn\n"},
		{"Warnf", func() { Warnf("level %s", "high") }, "level high\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, tt.fn)
			if !strings.Contains(out, tt.contains) {
				t.Errorf("expected output to contain %q, got %q", tt.contains, out)
			}
		})
	}
}
