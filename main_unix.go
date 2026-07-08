//go:build !windows

package main

import (
	"os"

	"github.com/quonaro/lota/runner"
)

func setupTerminal() (interface{}, bool) {
	f := os.Stdout
	// Check if stdout is a terminal
	if runner.IsTerminal(f) {
		// It's a terminal, disable signal echo
		oldState, _ := runner.DisableSignalEcho(f)
		return oldState, false
	}
	return nil, true // Not a terminal, no restoration needed
}

func restoreTerminal(state interface{}) error {
	if state == nil {
		return nil
	}
	return runner.RestoreTerminal(os.Stdout, state)
}
