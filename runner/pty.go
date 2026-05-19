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
func runWithPTY(cmd *exec.Cmd, stdout, stderr io.Writer, ctx context.Context) (bool, error) {
	ptmxOut, ptsOut, err := pty.Open()
	if err != nil {
		return false, nil
	}
	defer func() { _ = ptmxOut.Close() }()

	ptmxErr, ptsErr, err := pty.Open()
	if err != nil {
		return false, nil
	}
	defer func() { _ = ptmxErr.Close() }()

	cmd.Stdout = ptsOut
	cmd.Stderr = ptsErr

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start command: %w", err)
	}
	_ = ptsOut.Close()
	_ = ptsErr.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stdout, ptmxOut)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(stderr, ptmxErr)
	}()

	err = gracefulWait(cmd, ctx)
	wg.Wait()
	return true, err
}
