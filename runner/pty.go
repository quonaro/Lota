//go:build !windows

package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// runWithPTY runs cmd with a pseudo-terminal attached to stdout and stderr,
// then copies the PTY output into the supplied writers.
// This preserves ANSI colors from child processes that check isatty.
// It returns (false, nil) if PTY allocation fails so the caller can fall back
// to normal pipes.
func runWithPTY(cmd *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer, ctx context.Context, shutdownOnce *sync.Once) (bool, error) {
	ptmx, pts, err := pty.Open()
	if err != nil {
		return false, nil
	}
	defer func() { _ = ptmx.Close() }()

	cmd.Stdout = pts
	cmd.Stderr = pts
	cmd.Stdin = pts

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start command: %w", err)
	}
	_ = pts.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(ptmx, stdin)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stdout, ptmx)
	}()

	err = gracefulWait(cmd, ctx, shutdownOnce, stderr)
	wg.Wait()
	return true, err
}
