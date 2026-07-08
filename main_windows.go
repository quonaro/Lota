//go:build windows

package main

func setupTerminal() (interface{}, bool) {
	// Windows: no terminal setup needed
	return nil, true
}

func restoreTerminal(state interface{}) error {
	// Windows: no terminal restoration needed
	return nil
}
