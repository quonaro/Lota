//go:build !windows

package runner

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestGracefulWait_NormalCompletion(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := gracefulWait(cmd, context.Background(), nil, nil); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestGracefulWait_SIGTERM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.Command("sleep", "10")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := gracefulWait(cmd, ctx, nil, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}
