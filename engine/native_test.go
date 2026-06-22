package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"lota/config"
)

func TestRegisterNative(t *testing.T) {
	RegisterNative("test-hello", func(ctx context.Context, nctx NativeContext) error {
		_, _ = fmt.Fprintln(nctx.Stdout, "hello from native")
		return nil
	})
	defer unregisterNative("test-hello")

	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "test-hello", Native: true, Script: "echo ignored"},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), cfg, []string{"test-hello"}, Options{
		Stdout: &stdout,
		Stderr: &stdout,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "hello from native") {
		t.Fatalf("expected native output, got: %q", stdout.String())
	}
}

func TestNativeMissingHandler(t *testing.T) {
	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "unregistered", Native: true},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	err := Run(context.Background(), cfg, []string{"unregistered"}, Options{})
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

	RegisterNative("native-check", func(ctx context.Context, nctx NativeContext) error {
		gotVars = nctx.Vars
		gotArgs = nctx.Args
		_, _ = fmt.Fprintf(nctx.Stdout, "env=%s port=%s", nctx.Vars["ENV"], nctx.Args["port"])
		return nil
	})
	defer unregisterNative("native-check")

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
	err := Run(context.Background(), cfg, []string{"native-check"}, Options{
		Stdout: &stdout,
		Stderr: &stdout,
	})
	if err != nil {
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

func TestNativeReturnsError(t *testing.T) {
	RegisterNative("native-fail", func(ctx context.Context, nctx NativeContext) error {
		return errors.New("native failure")
	})
	defer unregisterNative("native-fail")

	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "native-fail", Native: true},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	err := Run(context.Background(), cfg, []string{"native-fail"}, Options{})
	if err == nil {
		t.Fatal("expected error from native command")
	}
	if !strings.Contains(err.Error(), "native failure") {
		t.Fatalf("expected 'native failure', got: %v", err)
	}
}

func TestNativeAsDependency(t *testing.T) {
	RegisterNative("native-dep", func(ctx context.Context, nctx NativeContext) error {
		_, _ = fmt.Fprintln(nctx.Stdout, "native-dep-ran")
		return nil
	})
	defer unregisterNative("native-dep")

	cfg := &config.AppConfig{
		Commands: []config.Command{
			{Name: "native-dep", Native: true},
			{Name: "shell-cmd", Script: "echo hello", Depends: []string{"native-dep"}},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), cfg, []string{"shell-cmd"}, Options{
		Stdout: &stdout,
		Stderr: &stdout,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "native-dep-ran") {
		t.Fatalf("expected native dependency output, got: %q", stdout.String())
	}
}

func TestNativeCommandPath(t *testing.T) {
	RegisterNative("group-native", func(ctx context.Context, nctx NativeContext) error {
		_, _ = fmt.Fprintln(nctx.Stdout, "group-native-ran")
		return nil
	})
	defer unregisterNative("group-native")

	cfg := &config.AppConfig{
		Groups: []config.Group{
			{
				Name: "infra",
				Commands: []config.Command{
					{Name: "group-native", Native: true},
				},
			},
		},
	}
	if err := cfg.BuildIndexes(); err != nil {
		t.Fatalf("BuildIndexes() error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), cfg, []string{"infra", "group-native"}, Options{
		Stdout: &stdout,
		Stderr: &stdout,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !strings.Contains(stdout.String(), "group-native-ran") {
		t.Fatalf("expected output, got: %q", stdout.String())
	}
}
