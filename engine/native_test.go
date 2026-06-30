package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/quonaro/lota/config"
)

func TestNativeHandler(t *testing.T) {
	var stdout bytes.Buffer
	app, err := NewBuilder("myapp", []byte(`
test-hello:
  desc: Say hello
  native: true
  script: echo ignored
`)).
		RegisterNative("test-hello", func(ctx context.Context, nctx NativeContext) error {
			_, _ = fmt.Fprintln(nctx.Stdout, "hello from native")
			return nil
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	app.opts.Stdout = &stdout
	app.opts.Stderr = &stdout

	if err := app.Run(context.Background(), []string{"test-hello"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello from native") {
		t.Fatalf("expected native output, got: %q", stdout.String())
	}
}

func TestNativeMissingHandler(t *testing.T) {
	app, err := NewBuilder("myapp", []byte(`
unregistered:
  desc: Unregistered native
  native: true
`)).Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	err = app.Run(context.Background(), []string{"unregistered"})
	if err == nil {
		t.Fatal("expected error for unregistered native command")
	}
	if !strings.Contains(err.Error(), "no registered handler") {
		t.Fatalf("expected 'no registered handler' error, got: %v", err)
	}
}

func TestNativeReceivesVarsAndArgs(t *testing.T) {
	var gotVars map[string]string
	var gotArgs map[string]string

	cmd := config.Command{
		Name:    "native-check",
		Native:  true,
		Vars:    []config.Var{{Name: "EXTRA", Value: "value"}},
		RawArgs: []string{"port:int=8080"},
	}
	cmd.Args = make([]config.Arg, len(cmd.RawArgs))
	for i, raw := range cmd.RawArgs {
		_ = cmd.Args[i].Parse(raw)
	}

	cfg := &config.AppConfig{
		Vars:     []config.Var{{Name: "ENV", Value: "prod"}},
		Commands: []config.Command{cmd},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	var stdout bytes.Buffer
	app := &App{
		cfg: cfg,
		opts: Options{
			Stdout:         &stdout,
			Stderr:         &stdout,
			NativeHandlers: map[string]NativeFunc{"native-check": nativeCheckHandler(&gotVars, &gotArgs)},
		},
		natives: map[string]NativeFunc{},
	}

	if err := app.Run(context.Background(), []string{"native-check"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if gotVars == nil {
		t.Fatal("expected Vars to be set")
	}
	if gotVars["ENV"] != "prod" {
		t.Errorf("Vars[ENV] = %q, want prod", gotVars["ENV"])
	}
	if gotVars["EXTRA"] != "value" {
		t.Errorf("Vars[EXTRA] = %q, want value", gotVars["EXTRA"])
	}
	if gotArgs["port"] != "8080" {
		t.Errorf("Args[port] = %q, want 8080", gotArgs["port"])
	}
	if !strings.Contains(stdout.String(), "env=prod port=8080") {
		t.Fatalf("expected output, got: %q", stdout.String())
	}
}

func nativeCheckHandler(gotVars *map[string]string, gotArgs *map[string]string) NativeFunc {
	return func(ctx context.Context, nctx NativeContext) error {
		*gotVars = nctx.Vars
		*gotArgs = nctx.Args
		_, _ = fmt.Fprintf(nctx.Stdout, "env=%s port=%s", nctx.Vars["ENV"], nctx.Args["port"])
		return nil
	}
}

func TestNativeReturnsError(t *testing.T) {
	app, err := NewBuilder("myapp", []byte(`
native-fail:
  desc: Failing native
  native: true
`)).
		RegisterNative("native-fail", func(ctx context.Context, nctx NativeContext) error {
			return errors.New("native failure")
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	err = app.Run(context.Background(), []string{"native-fail"})
	if err == nil {
		t.Fatal("expected error from native command")
	}
	if !strings.Contains(err.Error(), "native failure") {
		t.Fatalf("expected 'native failure', got: %v", err)
	}
}

func TestNativeAsDependency(t *testing.T) {
	var stdout bytes.Buffer
	app, err := NewBuilder("myapp", []byte(`
native-dep:
  desc: Native dependency
  native: true

shell-cmd:
  desc: Shell command
  script: echo hello
  depends:
    - native-dep
`)).
		RegisterNative("native-dep", func(ctx context.Context, nctx NativeContext) error {
			_, _ = fmt.Fprintln(nctx.Stdout, "native-dep-ran")
			return nil
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	app.opts.Stdout = &stdout
	app.opts.Stderr = &stdout

	if err := app.Run(context.Background(), []string{"shell-cmd"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "native-dep-ran") {
		t.Fatalf("expected native dependency output, got: %q", stdout.String())
	}
}

func TestNativeCommandPath(t *testing.T) {
	var stdout bytes.Buffer
	app, err := NewBuilder("myapp", []byte(`
infra:
  group-native:
    desc: Group native
    native: true
`)).
		RegisterNative("infra.group-native", func(ctx context.Context, nctx NativeContext) error {
			_, _ = fmt.Fprintln(nctx.Stdout, "group-native-ran")
			return nil
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	app.opts.Stdout = &stdout
	app.opts.Stderr = &stdout

	if err := app.Run(context.Background(), []string{"infra", "group-native"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "group-native-ran") {
		t.Fatalf("expected output, got: %q", stdout.String())
	}
}

func TestNativeDuplicateNameRejected(t *testing.T) {
	_, err := NewBuilder("myapp", []byte(`
infra:
  deploy:
    desc: Infra deploy
    native: true
app:
  deploy:
    desc: App deploy
    native: true
`)).
		RegisterNative("infra.deploy", func(ctx context.Context, nctx NativeContext) error {
			return nil
		}).
		RegisterNative("app.deploy", func(ctx context.Context, nctx NativeContext) error {
			return nil
		}).
		Build()
	if err == nil {
		t.Fatal("expected Build() error for duplicate command names")
	}
	if !strings.Contains(err.Error(), "duplicate command name") {
		t.Fatalf("expected duplicate command name error, got: %v", err)
	}
}
