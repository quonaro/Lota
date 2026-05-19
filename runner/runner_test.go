package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"lota/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteCommand_EmptyScript(t *testing.T) {
	cmd := &config.Command{Name: "noop"}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteCommand_DryRun_ScriptNotExecuted(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	cmd := &config.Command{
		Name:   "test",
		Script: fmt.Sprintf("touch %s", marker),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{DryRun: true}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Error("script was executed despite dry-run mode")
	}
}

func TestExecuteCommand_DryRun_BeforeHookNotExecuted(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "before_marker")
	cmd := &config.Command{
		Name:   "test",
		Before: fmt.Sprintf("touch %s", marker),
		Script: "echo noop",
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{DryRun: true}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Error("before hook was executed despite dry-run mode")
	}
}

func TestExecuteCommand_DryRun_PrintsScriptAndAfter(t *testing.T) {
	cmd := &config.Command{
		Name:   "test",
		Before: "echo before",
		Script: "echo script",
		After:  "echo after",
	}
	ctx := InterpolationContext{Vars: map[string]string{"Z": "2", "A": "1"}, Args: map[string]string{}}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{DryRun: true}, "sh -c", "")

	_ = w.Close()
	os.Stdout = oldStdout

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read output error: %v", err)
	}
	text := out.String()

	for _, fragment := range []string{"[dry-run] env:", "  A=1", "  Z=2", "[dry-run] before:", "[dry-run] script:", "[dry-run] after:"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected output to contain %q, got %q", fragment, text)
		}
	}
}

func TestExecuteCommand_ScriptInterpolationError(t *testing.T) {
	cmd := &config.Command{Name: "test", Script: "echo {{undefined}}"}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err == nil {
		t.Error("expected error for undefined placeholder, got nil")
	}
}

func TestExecuteCommand_BeforeHookInterpolationError(t *testing.T) {
	cmd := &config.Command{
		Name:   "test",
		Before: "echo {{undefined}}",
		Script: "echo noop",
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err == nil {
		t.Error("expected error for undefined placeholder in before hook, got nil")
	}
}

func TestExecuteCommand_WithInterpolation(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.txt")
	cmd := &config.Command{
		Name:   "test",
		Script: fmt.Sprintf("echo {{msg}} > \"%s\"", out),
	}
	ctx := InterpolationContext{
		Vars:    map[string]string{},
		Args:    map[string]string{"msg": "hello"},
		ArgDefs: []config.Arg{{Name: "msg", Type: "str"}},
	}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Errorf("expected output to contain 'hello', got %q", string(data))
	}
}

func TestExecuteCommand_BeforeAndAfterHooksExecuted(t *testing.T) {
	dir := t.TempDir()
	beforeMarker := filepath.Join(dir, "before")
	afterMarker := filepath.Join(dir, "after")

	cmd := &config.Command{
		Name:   "test",
		Before: fmt.Sprintf("touch \"%s\"", beforeMarker),
		Script: "echo noop",
		After:  fmt.Sprintf("touch \"%s\"", afterMarker),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(beforeMarker); err != nil {
		t.Error("before hook was not executed")
	}
	if _, err := os.Stat(afterMarker); err != nil {
		t.Error("after hook was not executed")
	}
}

func TestExecuteCommand_WithDir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	marker := filepath.Join(subDir, "marker")

	cmd := &config.Command{
		Name:   "test",
		Dir:    "subdir",
		Script: fmt.Sprintf("touch %s", "marker"),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{ConfigDir: tmpDir}, "sh -c", "subdir"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker to be created in subdir: %v", err)
	}
}

func TestExecuteCommand_VarsPassedAsEnv(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.txt")
	cmd := &config.Command{
		Name:   "test",
		Script: fmt.Sprintf("echo $MY_VAR > \"%s\"", out),
	}
	ctx := InterpolationContext{
		Vars: map[string]string{"MY_VAR": "from_env"},
		Args: map[string]string{},
	}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !strings.Contains(string(data), "from_env") {
		t.Errorf("expected output to contain 'from_env', got %q", string(data))
	}
}

func TestExecuteCommand_ArgsPassedAsEnv(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.txt")
	cmd := &config.Command{
		Name:   "test",
		Script: fmt.Sprintf("echo $MSG > \"%s\"", out),
	}
	ctx := InterpolationContext{
		Vars: map[string]string{},
		Args: map[string]string{"MSG": "from_arg"},
	}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !strings.Contains(string(data), "from_arg") {
		t.Errorf("expected output to contain 'from_arg', got %q", string(data))
	}
}

func TestExecuteCommand_ArgsOverrideVarsInEnv(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.txt")
	cmd := &config.Command{
		Name:   "test",
		Script: fmt.Sprintf("echo $PORT > \"%s\"", out),
	}
	ctx := InterpolationContext{
		Vars: map[string]string{"PORT": "80"},
		Args: map[string]string{"PORT": "8080"},
	}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !strings.Contains(string(data), "8080") {
		t.Errorf("expected args to override vars in env, got %q", string(data))
	}
}

func TestExecuteCommand_AfterRunsOnScriptFailure(t *testing.T) {
	dir := t.TempDir()
	afterMarker := filepath.Join(dir, "after")

	cmd := &config.Command{
		Name:   "test",
		Script: "exit 1",
		After:  fmt.Sprintf("touch \"%s\"", afterMarker),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", "")
	if err == nil {
		t.Fatal("expected script error, got nil")
	}

	if _, err := os.Stat(afterMarker); err != nil {
		t.Error("after hook was not executed after script failure")
	}
}

func TestExecuteCommand_AfterNotRunOnBeforeFailure(t *testing.T) {
	dir := t.TempDir()
	afterMarker := filepath.Join(dir, "after")

	cmd := &config.Command{
		Name:   "test",
		Before: "exit 1",
		Script: "echo noop",
		After:  fmt.Sprintf("touch \"%s\"", afterMarker),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", "")
	if err == nil {
		t.Fatal("expected before error, got nil")
	}

	if _, err := os.Stat(afterMarker); err == nil {
		t.Error("after hook should not run when before fails")
	}
}

func TestExecuteCommand_Timeout(t *testing.T) {
	cmd := &config.Command{
		Name:   "test",
		Script: "sleep 5",
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	start := time.Now()
	err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{Timeout: 100 * time.Millisecond}, "sh -c", "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout was not enforced: elapsed %v", elapsed)
	}
}

func TestExecuteCommand_ExitCodePropagation(t *testing.T) {
	cmd := &config.Command{
		Name:   "test",
		Script: "exit 42",
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{}, "sh -c", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	shellErr, ok := err.(*ShellError)
	if !ok {
		t.Fatalf("expected *ShellError, got %T", err)
	}
	if shellErr.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", shellErr.ExitCode)
	}
	if !strings.Contains(shellErr.Command, "exit 42") {
		t.Errorf("expected command summary to contain script fragment, got %q", shellErr.Command)
	}
}

func TestExecuteCommand_DefaultsToConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "marker")

	cmd := &config.Command{
		Name:   "test",
		Script: fmt.Sprintf("touch %s", "marker"),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{ConfigDir: tmpDir}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker to be created in ConfigDir: %v", err)
	}
}

func TestExecuteCommand_CWD(t *testing.T) {
	cwdDir := t.TempDir()
	marker := filepath.Join(cwdDir, "marker")

	cmd := &config.Command{
		Name:   "test",
		Dir:    "$CWD",
		Script: fmt.Sprintf("touch %s", "marker"),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{ConfigDir: "/nonexistent", WorkingDir: cwdDir}, "sh -c", "$CWD"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker to be created in CWD: %v", err)
	}
}

func TestExecuteCommand_CWDSubdir(t *testing.T) {
	cwdDir := t.TempDir()
	subDir := filepath.Join(cwdDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	marker := filepath.Join(subDir, "marker")

	cmd := &config.Command{
		Name:   "test",
		Dir:    "$CWD/sub",
		Script: fmt.Sprintf("touch %s", "marker"),
	}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(context.Background(), cmd, ctx, RunOptions{ConfigDir: "/nonexistent", WorkingDir: cwdDir}, "sh -c", "$CWD/sub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker to be created in CWD/sub: %v", err)
	}
}

func TestResolveDir_Unit(t *testing.T) {
	tests := []struct {
		name       string
		baseDir    string
		workingDir string
		dir        string
		expected   string
	}{
		{"empty defaults to baseDir", "/config", "/cwd", "", "/config"},
		{"$CWD", "/config", "/cwd", "$CWD", "/cwd"},
		{"$CWD/sub", "/config", "/cwd", "$CWD/sub", filepath.Join("/cwd", "sub")},
		{"relative to baseDir", "/config", "/cwd", "backend", filepath.Join("/config", "backend")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveDir(tt.baseDir, tt.workingDir, tt.dir)
			if result != tt.expected {
				t.Errorf("resolveDir() = %v, want %v", result, tt.expected)
			}
		})
	}
}
