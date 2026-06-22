package engine

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// NativeContext holds all information passed to a native command handler.
type NativeContext struct {
	Vars   map[string]string
	Args   map[string]string
	Stdout io.Writer
	Stderr io.Writer
}

// NativeFunc is the signature for Go functions that replace shell execution.
type NativeFunc func(ctx context.Context, nctx NativeContext) error

var (
	nativeRegistry = make(map[string]NativeFunc)
	nativeMu       sync.RWMutex
)

// RegisterNative registers a Go function as the handler for a command name.
// If a command with the same name was already registered, it is overwritten.
func RegisterNative(name string, fn NativeFunc) {
	nativeMu.Lock()
	defer nativeMu.Unlock()
	nativeRegistry[name] = fn
}

// unregisterNative removes a previously registered native handler.
// Used only in tests.
func unregisterNative(name string) {
	nativeMu.Lock()
	defer nativeMu.Unlock()
	delete(nativeRegistry, name)
}

// lookupNative returns the registered handler and whether it exists.
func lookupNative(name string) (NativeFunc, bool) {
	nativeMu.RLock()
	defer nativeMu.RUnlock()
	fn, ok := nativeRegistry[name]
	return fn, ok
}

// runNative executes a registered native handler with the given context.
func runNative(ctx context.Context, name string, nctx NativeContext) error {
	fn, ok := lookupNative(name)
	if !ok {
		return fmt.Errorf("native command %q has no registered handler", name)
	}
	return fn(ctx, nctx)
}
