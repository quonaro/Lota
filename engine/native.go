package engine

import (
	"context"
	"fmt"
	"io"
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

// runNative executes a registered native handler with the given context.
// The handler is looked up in the per-app natives map by its full command path.
func runNative(ctx context.Context, name string, nctx NativeContext, natives map[string]NativeFunc) error {
	if natives == nil {
		return fmt.Errorf("native command %q has no registered handler", name)
	}
	fn, ok := natives[name]
	if !ok {
		return fmt.Errorf("native command %q has no registered handler", name)
	}
	return fn(ctx, nctx)
}
