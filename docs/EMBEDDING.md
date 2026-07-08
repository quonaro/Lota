# Lota Embedding Guide

Lota can be embedded into other Go binaries as a library. The `lota/engine` package provides a clean API for loading configurations and running commands programmatically, without depending on CLI-specific code like flag parsing, shell completion, or `os.Args`.

## Table of Contents

- [Why embed Lota?](#why-embed-lota)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Advanced Usage](#advanced-usage)
- [Native Commands](#native-commands)
- [API Reference](#api-reference)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Why embed Lota?

Embedding Lota into your Go application enables:

- **Single binary distribution** - Ship a task runner built into your application (e.g., `myapp run` instead of `lota run`)
- **Declarative configuration** - Keep the same YAML-based configuration while wrapping it with your own branding and logic
- **Programmatic control** - Execute commands programmatically without spawning external processes
- **Native handlers** - Mix shell scripts with Go code for complex operations
- **No external dependencies** - Users don't need to install Lota separately

## Installation

Add Lota as a module dependency:

```bash
go get github.com/quonaro/lota
```

## Quick Start

### Minimal Example

The simplest way to embed Lota is to load a config file and run a command:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/quonaro/lota/engine"
)

func main() {
    ctx := context.Background()

    // Load config from a file
    data, err := os.ReadFile("lota.yml")
    if err != nil {
        fmt.Fprintf(os.Stderr, "read config: %v\n", err)
        os.Exit(1)
    }

    cfg, err := engine.LoadConfig(data)
    if err != nil {
        fmt.Fprintf(os.Stderr, "config: %v\n", err)
        os.Exit(1)
    }

    // Run a command using CLI-style arguments
    err = engine.Run(ctx, cfg, []string{"deploy", "prod", "--force"}, engine.Options{
        Stdout: os.Stdout,
        Stderr: os.Stderr,
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "run: %v\n", err)
        os.Exit(1)
    }
}
```

### With `//go:embed` (Recommended)

The recommended approach uses Go's embed directive and the builder pattern:

```go
package main

import (
    "context"
    "embed"
    "errors"
    "fmt"
    "os"

    "github.com/quonaro/lota/engine"
)

//go:embed lota.yml
var lotaYAML []byte

func main() {
    builder := engine.NewBuilder("myapp", lotaYAML)
    builder.RegisterNative("deploy", deployHandler)
    builder.RegisterNative("infra.status", statusHandler)

    app, err := builder.Build()
    if err != nil {
        fmt.Fprintf(os.Stderr, "config: %v\n", err)
        os.Exit(1)
    }

    if len(os.Args) < 2 {
        app.PrintHelp()
        return
    }

    if err := app.Run(context.Background(), os.Args[1:]); err != nil {
        var groupErr *engine.GroupError
        if errors.As(err, &groupErr) {
            app.PrintGroupHelp(groupErr.Groups)
            return
        }
        fmt.Fprintf(os.Stderr, "run: %v\n", err)
        os.Exit(1)
    }
}

func deployHandler(ctx context.Context, nctx engine.NativeContext) error {
    fmt.Fprintf(nctx.Stdout, "Deploying...\n")
    // Your deployment logic here
    return nil
}

func statusHandler(ctx context.Context, nctx engine.NativeContext) error {
    fmt.Fprintf(nctx.Stdout, "Checking infrastructure status...\n")
    // Your status check logic here
    return nil
}
```

**Key benefits of the builder pattern:**

- **App name is set once** - No need to pass it to every help call
- **Native handlers are scoped** - Each app has its own handler registry
- **Fluent API** - Chain method calls for clean code
- **Type-safe** - Compile-time checking of handler registration

## Basic Usage

### Loading Configuration

#### From `[]byte`

```go
cfg, err := engine.LoadConfig(lotaYAML)
if err != nil {
    // handle error
}
```

#### From file path

```go
cfg, configDir, err := engine.LoadConfigFromPath("./my-tasks.yaml")
if err != nil {
    // handle error
}
```

The `configDir` is the parent directory of the config file, useful for resolving relative paths in your configuration.

### Running Commands

#### Using CLI-style arguments

```go
err := engine.Run(ctx, cfg, []string{"deploy", "prod", "--force"}, engine.Options{
    Stdout: os.Stdout,
    Stderr: os.Stderr,
})
```

#### Using command path

```go
err := engine.RunCommand(ctx, cfg, "deploy.prod", []string{"--force"}, engine.Options{
    Stdout: os.Stdout,
    Stderr: os.Stderr,
})
```

### Displaying Help

#### Top-level help

```go
engine.PrintHelp(cfg, os.Stdout, "myapp")
```

Output:
```
Usage: myapp <command> [args...]

Commands:
  deploy               Deploy the application
  test                 Run the test suite
```

#### Group-specific help

```go
err := engine.Run(ctx, cfg, []string{"admin"}, opts)
var groupErr *engine.GroupError
if errors.As(err, &groupErr) {
    engine.PrintGroupHelp(cfg, groupErr.Groups, os.Stdout, "myapp")
}
```

Output:
```
Usage: myapp admin <command> [args...]

Administration tools

Commands:
  users                Manage users
  db                   Database tools
```

## Advanced Usage

### The App Wrapper

The `App` struct bundles config, options, and native handlers:

```go
app, err := engine.NewApp(lotaYAML, engine.Options{
    Verbose: true,
    DryRun:  false,
    Stdout:  os.Stdout,
    Stderr:  os.Stderr,
})
if err != nil {
    // handle error
}

// Run commands
err = app.Run(ctx, []string{"build"})

// Print help
app.PrintHelp()

// Access config
cfg := app.Config()
```

### Loading from File Path

```go
app, err := engine.NewAppFromPath("./my-tasks.yaml", engine.Options{
    ConfigDir: "./config", // Optional: overrides detected config dir
    Stdout:    os.Stdout,
    Stderr:    os.Stderr,
})
```

### Custom Options

```go
opts := engine.Options{
    Verbose:         true,                    // Enable verbose logging
    DryRun:          false,                   // Print without executing
    ConfigDir:       "./config",              // Base for relative paths
    WorkingDir:      os.Getwd(),              // Current working directory
    Timeout:         30 * time.Second,        // Execution timeout
    Stdout:          os.Stdout,               // Output writer
    Stderr:          os.Stderr,               // Error writer
    PrefixFormatter: customPrefixFormatter,   // Custom dependency prefix
    NativeHandlers:  nativeHandlers,         // Native command handlers
}
```

### Custom Prefix Formatter

When dependencies run in parallel, Lota prefixes each line of output. Customize this:

```go
opts := engine.Options{
    Stdout: os.Stdout,
    Stderr: os.Stderr,
    PrefixFormatter: func(path string, cmd *config.Command, groups []*config.Group) string {
        return fmt.Sprintf("\033[36m[%s]\033[0m ", path)
    },
}
```

### Programmatic Command Lookup

Inspect the configuration before running:

```go
result, err := config.FindCommandByPath(cfg, "infra.docker.up")
if err != nil {
    // handle error
}

fmt.Println("Command:", result.Command.Name)
fmt.Println("Script:", result.Command.Script)
fmt.Println("Description:", result.Command.Desc)

// Run it manually
err = engine.RunCommand(ctx, cfg, "infra.docker.up", nil, opts)
```

## Native Commands

Native commands allow you to execute Go code instead of shell scripts while keeping the declarative YAML configuration.

### Declaring a Native Command

```yaml
deploy:
  desc: Deploy via native Go code
  native: true
  vars:
    - REGION=eu-west-1
  args:
    - "env:str=dev"
```

The `native: true` marker tells Lota to look up a registered Go handler instead of running a shell script.

### Registering Handlers

#### Using the builder (recommended)

```go
builder := engine.NewBuilder("myapp", lotaYAML)

builder.RegisterNative("deploy", func(ctx context.Context, nctx engine.NativeContext) error {
    env := nctx.Args["env"]       // "dev" or "prod"
    region := nctx.Vars["REGION"] // "eu-west-1"

    fmt.Fprintf(nctx.Stdout, "Deploying to %s in %s\n", env, region)

    // Your Go code here:
    // - HTTP requests
    // - Database calls
    // - File operations
    // - Any logic you need

    return nil
})

app, err := builder.Build()
```

#### Using Options

```go
app, err := engine.NewApp(lotaYAML, engine.Options{
    NativeHandlers: map[string]engine.NativeFunc{
        "deploy": func(ctx context.Context, nctx engine.NativeContext) error {
            // handler logic
            return nil
        },
        "admin.users.reset-password": func(ctx context.Context, nctx engine.NativeContext) error {
            // handler logic
            return nil
        },
    },
})
```

### Command Paths

- **Root commands**: Use the command name (`"deploy"`)
- **Nested commands**: Use the full dot-separated path (`"infra.status"`, `"admin.users.reset-password"`)

### NativeContext

```go
type NativeContext struct {
    Vars   map[string]string
    Args   map[string]string
    Stdout io.Writer
    Stderr io.Writer
}
```

| Field    | Description                                 |
| -------- | ------------------------------------------- |
| `Vars`   | Resolved variables (app → group → command) |
| `Args`   | Parsed CLI arguments with defaults applied  |
| `Stdout` | Where to write command output              |
| `Stderr` | Where to write errors and warnings         |

### Native Commands as Dependencies

Native commands can be dependencies of shell commands (and vice versa):

```yaml
build:
  native: true

test:
  script: go test ./...
  depends:
    - build
```

```go
builder.RegisterNative("build", func(ctx context.Context, nctx engine.NativeContext) error {
    // Run Go build programmatically
    return nil
})
```

### Error Handling

If a command is marked `native: true` but no handler was registered:

```
native command "deploy" has no registered handler
```

Register handlers on the `AppBuilder` before calling `Build()`, or pass them via `Options.NativeHandlers`.

## API Reference

### Types

#### `engine.Options`

```go
type Options struct {
    Verbose         bool
    DryRun          bool
    ConfigDir       string
    WorkingDir      string
    Timeout         time.Duration
    Stdout          io.Writer
    Stderr          io.Writer
    PrefixFormatter func(path string, cmd *config.Command, groups []*config.Group) string
    NativeHandlers  map[string]NativeFunc
}
```

| Field             | Description                                                            |
| ----------------- | ---------------------------------------------------------------------- |
| `Verbose`         | Enable verbose logging                                                  |
| `DryRun`          | Print interpolated scripts without executing                           |
| `ConfigDir`       | Base directory for resolving relative paths                             |
| `WorkingDir`      | Caller's current working directory; used for `$CWD` interpolation       |
| `Timeout`         | Maximum execution time (0 = no timeout)                                |
| `Stdout`          | Where command output is written (defaults to `os.Stdout` if nil)       |
| `Stderr`          | Where errors are written (defaults to `os.Stderr` if nil)               |
| `PrefixFormatter` | Optional formatter for dependency output prefixes                     |
| `NativeHandlers`  | Map of full command path to native handler                             |

#### `engine.NativeContext`

```go
type NativeContext struct {
    Vars   map[string]string
    Args   map[string]string
    Stdout io.Writer
    Stderr io.Writer
}
```

#### `engine.NativeFunc`

```go
type NativeFunc func(ctx context.Context, nctx NativeContext) error
```

#### `engine.GroupError`

```go
type GroupError struct {
    Path   string
    Groups []*config.Group
}
```

Returned by `engine.Run` when the resolved path points to a group rather than a command.

### Functions

#### `engine.LoadConfig(data []byte) (*config.AppConfig, error)`

Parses YAML, builds indexes, and validates the configuration.

#### `engine.LoadConfigFromPath(path string) (*config.AppConfig, string, error)`

Reads a config file from the given path, parses it, and returns both the config and its parent directory.

#### `engine.Run(ctx context.Context, cfg *config.AppConfig, args []string, opts Options) error`

CLI-style entrypoint. `args` is the full command line (e.g., `[]string{"deploy", "prod", "--force"}`).

#### `engine.RunCommand(ctx context.Context, cfg *config.AppConfig, path string, cmdArgs []string, opts Options) error`

Programmatic entrypoint. `path` is a dot-separated command path (e.g., `"deploy.prod"`).

#### `engine.PrintHelp(cfg *config.AppConfig, w io.Writer, appName string)`

Writes top-level help to `w`.

#### `engine.PrintGroupHelp(cfg *config.AppConfig, groups []*config.Group, w io.Writer, appName string)`

Writes group-specific help to `w`.

### Builder API

#### `engine.NewBuilder(name string, data []byte) *AppBuilder`

Creates a builder from embedded YAML data.

#### `engine.NewBuilderFromPath(name, path string) *AppBuilder`

Creates a builder from a file path.

#### `(*AppBuilder) RegisterNative(name string, fn NativeFunc) *AppBuilder`

Registers a native Go handler for a command path. Returns the builder for chaining.

#### `(*AppBuilder) WithOptions(opts Options) *AppBuilder`

Sets execution options. Returns the builder for chaining.

#### `(*AppBuilder) Build() (*App, error)`

Parses config, registers natives, and returns the `*App`.

### App API

#### `engine.NewApp(data []byte, opts Options) (*App, error)`

Creates an App from YAML data.

#### `engine.NewAppFromPath(path string, opts Options) (*App, error)`

Creates an App from a file path.

#### `(*App) Run(ctx context.Context, args []string) error`

Runs a command with CLI-style arguments.

#### `(*App) PrintHelp()`

Writes top-level help to the configured stdout.

#### `(*App) PrintGroupHelp(groups []*config.Group)`

Writes group-specific help to the configured stdout.

#### `(*App) Config() *config.AppConfig`

Returns the underlying parsed configuration.

## Examples

See the `examples/` directory for complete working examples:

- **[embedded.yaml](../examples/embedded.yaml)** - Full-featured example with native commands, groups, colors, variables, and arguments
- **[simple-web-project.yaml](../examples/simple-web-project.yaml)** - Web development workflow
- **[devops-infrastructure.yaml](../examples/devops-infrastructure.yaml)** - Docker and Kubernetes operations
- **[go-project.yaml](../examples/go-project.yaml)** - Go build, test, and release workflows
- **[multi-environment.yaml](../examples/multi-environment.yaml)** - Environment-specific configurations

## Troubleshooting

### "command not found" errors

Make sure you called `cfg.BuildIndexes()` after parsing. `engine.LoadConfig` does this for you automatically.

### No output captured

Ensure you set `Stdout` and `Stderr` in `engine.Options`. If left nil, output goes to `os.Stdout`/`os.Stderr` of the host process.

### PTY not used when embedding

If you pass a custom `io.Writer` (e.g., `bytes.Buffer`), Lota cannot allocate a pseudo-terminal because it must tee output into your writer. If you need TTY detection (e.g., for colored child output), pass `os.Stdout`/`os.Stderr` directly.

### Binary size

Lota `engine` itself is tiny (only `yaml.v3` as a non-std dependency). To reduce binary size:

```bash
# Strip debug info and symbol table
go build -ldflags="-s -w" -o myapp ./cmd/myapp

# Compress with UPX (optional, reduces ~50-70%)
upx --best myapp
```

If you only use native commands (no shell scripts), `github.com/creack/pty` is excluded on non-Unix builds automatically via build tags.

### Native command handler not found

```
native command "deploy" has no registered handler
```

Ensure you:
1. Marked the command with `native: true` in YAML
2. Registered a handler with the correct full path (e.g., `"admin.users.reset-password"` for nested commands)
3. Called `Build()` after registering handlers

### Config file not found

When using `NewAppFromPath`, ensure the path is relative to your application's working directory or use an absolute path:

```go
// Relative to current directory
app, err := engine.NewAppFromPath("./config/lota.yml", opts)

// Absolute path
app, err := engine.NewAppFromPath("/etc/myapp/lota.yml", opts)
```

## Comparison with CLI Mode

| Feature               | `cli.Run(ctx)`                | `engine.Run(ctx, cfg, args, opts)` |
| --------------------- | ----------------------------- | ---------------------------------- |
| `os.Args`             | Reads directly                | Accepts explicit `[]string`        |
| `os.Exit`             | Uses `os.Exit` for completion | Never calls `os.Exit`              |
| Output                | Hardcoded `os.Stdout/Stderr`  | Configurable `io.Writer`           |
| Config source         | Filesystem only               | Can use `[]byte` / `io.Reader`     |
| Help/Completion       | Built-in                      | `engine.PrintHelp` (no CLI flags)  |
| Native handlers       | Global                        | Per-app (scoped)                   |
| Use case              | Standalone binary             | Embedded library                   |
