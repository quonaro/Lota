package runner

import (
	"fmt"
	"lota/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteCommand_EmptyScript(t *testing.T) {
	cmd := &config.Command{Name: "noop"}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{DryRun: true}, "sh -c", ""); err != nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{DryRun: true}, "sh -c", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Error("before hook was executed despite dry-run mode")
	}
}

func TestExecuteCommand_ScriptInterpolationError(t *testing.T) {
	cmd := &config.Command{Name: "test", Script: "echo {{undefined}}"}
	ctx := InterpolationContext{Vars: map[string]string{}, Args: map[string]string{}}

	if err := ExecuteCommand(cmd, ctx, RunOptions{}, "sh -c", ""); err == nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{}, "sh -c", ""); err == nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{ConfigDir: tmpDir}, "sh -c", "subdir"); err != nil {
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

	if err := ExecuteCommand(cmd, ctx, RunOptions{}, "sh -c", ""); err != nil {
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
