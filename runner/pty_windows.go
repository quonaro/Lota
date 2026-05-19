//go:build windows

package runner

import (
	"context"
	"io"
	"os/exec"
)

func runWithPTY(cmd *exec.Cmd, stdout, stderr io.Writer, ctx context.Context) (bool, error) {
	return false, nil
}
