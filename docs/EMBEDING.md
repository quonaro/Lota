# Embedding Lota

Lota can be embedded into other Go binaries as a library. The `lota/engine` package provides a clean API for loading configurations and running commands programmatically, without depending on CLI-specific code like flag parsing, shell completion, or `os.Args`.

## Why embed?

- Ship a single binary with a built-in task runner (e.g., `outless run` = `lota run`).
- Provide a task-driven CLI without maintaining a separate YAML parser or shell executor.
- Keep the same declarative configuration (`lota.yml`) while wrapping it with your own branding and logic.

## Quick start

Install Lota as a module dependency:

```bash
go get github.com/quonaro/lota
```

For a full-featured example of everything supported in embedded mode (native commands, groups, colors, variables, arguments, and more), see [`embedded.example.yaml`](../embedded.example.yaml) in the repository root.

### Minimal example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "lota/engine"
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

### With `//go:embed` (class-like builder)

The recommended way is `AppBuilder` — create it with a name and config, register natives, then build:

```go
package main

import (
    "_embed"
    "context"
    "errors"
    "fmt"
    "os"

    "lota/engine"
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
```

`NewBuilder` takes the app name and YAML data. `RegisterNative` adds Go handlers bound to that `App` only. The name passed to `RegisterNative` must be the full command path (`"deploy"` for root commands, `"infra.status"` for commands inside groups). `Build` parses config, sets defaults, validates uniqueness, and returns the `*App`. No more passing `"myapp"` to every help call.

For simpler cases, `engine.NewApp(data, Options{})` still works. To use native commands without the builder, pass handlers via `Options.NativeHandlers`:

```go
app, err := engine.NewApp(data, engine.Options{
    NativeHandlers: map[string]engine.NativeFunc{
        "deploy": deployHandler,
        "admin.users.reset-password": resetPasswordHandler,
    },
})
```

### Loading from a file path

If your config lives at a known path, use `LoadConfigFromPath`. The path can have **any file name** — `lota.yml`, `commands.yaml`, `tasks.yml`, etc. It returns both the parsed config and its directory, which is useful for `engine.Options.ConfigDir`:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "lota/engine"
)

func main() {
    ctx := context.Background()

    // Any file name works
    cfg, configDir, err := engine.LoadConfigFromPath("./my-tasks.yaml")
    if err != nil {
        fmt.Fprintf(os.Stderr, "config: %v\n", err)
        os.Exit(1)
    }

    if len(os.Args) < 2 {
        engine.PrintHelp(cfg, os.Stdout, "myapp")
        return
    }

    if err := engine.Run(ctx, cfg, os.Args[1:], engine.Options{
        ConfigDir: configDir,
        Stdout:    os.Stdout,
        Stderr:    os.Stderr,
    }); err != nil {
        fmt.Fprintf(os.Stderr, "run: %v\n", err)
        os.Exit(1)
    }
}
```

With the `App` wrapper the same looks even shorter:

```go
app, err := engine.NewAppFromPath("./my-tasks.yaml", engine.Options{})
```

### Embedded help

When Lota is embedded, global CLI flags such as `--init` or `--completion-script` do not exist. Use `engine.PrintHelp` to show only the commands and groups defined in the configuration:

```go
engine.PrintHelp(cfg, os.Stdout, "myapp")
// Output:
// Usage: myapp <command> [args...]
//
// Commands:
//   deploy               Deploy the application
//   test                 Run the test suite
```

### Handling groups

If the user passes a group name instead of a command (e.g., `./outless admin`), `engine.Run` returns a `*engine.GroupError`. You can detect it with `errors.As` and show group-specific help:

```go
err := engine.Run(ctx, cfg, os.Args[1:], opts)
var groupErr *engine.GroupError
if errors.As(err, &groupErr) {
    engine.PrintGroupHelp(cfg, groupErr.Groups, os.Stdout, "myapp")
    return nil
}
```

`PrintGroupHelp` lists the commands and sub-groups inside the target group:

```
Usage: myapp admin <command> [args...]

Administration tools

Commands:
  users                Manage users
  db                   Database tools
```

## API Reference

### `engine.Options`

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
    NativeHandlers  map[string]NativeFunc // full command path -> handler
}
```

| Field             | Description                                                            |
| ----------------- | ---------------------------------------------------------------------- |
| `Verbose`         | Enable verbose logging.                                                |
| `DryRun`          | Print interpolated scripts without executing.                          |
| `ConfigDir`       | Base directory for resolving relative paths (e.g., log files).         |
| `WorkingDir`      | Caller's current working directory; used for `$CWD` interpolation.     |
| `Timeout`         | Maximum execution time (0 = no timeout).                               |
| `Stdout`          | Where command output is written. Defaults to `os.Stdout` if nil.       |
| `Stderr`          | Where errors and warnings are written. Defaults to `os.Stderr` if nil. |
| `PrefixFormatter` | Optional formatter for dependency output prefixes.                     |
| `NativeHandlers`  | Map of full command path to native handler. Used by `engine.Run`.      |

### `engine.LoadConfig(data []byte) (*config.AppConfig, error)`

Parses YAML, builds indexes, and validates the configuration. Returns a ready-to-use `*config.AppConfig`.

### `engine.Run(ctx, cfg, args, opts) error`

CLI-style entrypoint. `args` is the full command line (e.g., `[]string{"deploy", "prod", "--force"}`). It greedily resolves the command path from the config tree, then runs the target command including dependencies.

### `engine.RunCommand(ctx, cfg, path, cmdArgs, opts) error`

Programmatic entrypoint. `path` is a dot-separated command path (e.g., `"deploy.prod"`). `cmdArgs` are the command-specific arguments after the path is resolved.

### `engine.LoadConfigFromPath(path string) (*config.AppConfig, string, error)`

Reads a config file from the given path, parses it, builds indexes, validates it, and returns both the config and its parent directory. The directory is useful for `engine.Options.ConfigDir` when resolving relative paths (e.g., log files or `$CWD`).

### `engine.PrintHelp(cfg *config.AppConfig, w io.Writer, appName string)`

Writes a list of available top-level commands and groups to `w`. Unlike `cli.PrintHelp`, it does not print global CLI flags (`--init`, `--completion-script`, etc.) because those are irrelevant when Lota is embedded.

### `engine.PrintGroupHelp(cfg *config.AppConfig, groups []*config.Group, w io.Writer, appName string)`

Writes the commands and sub-groups inside a specific group to `w`. `groups` is the chain from outermost to innermost (as returned by `config.ResolveCommand` or `engine.GroupError.Groups`). If `groups` is empty, it falls back to `PrintHelp`.

### `engine.GroupError`

Returned by `engine.Run` when the resolved path points to a group rather than a command. Use `errors.As` to detect it and access `Path` and `Groups` for displaying help.

```go
type GroupError struct {
    Path   string
    Groups []*config.Group
}
```

### `engine.NewBuilder(name string, data []byte) *AppBuilder`

Class-like builder for constructing an `App`. Pass the app name and YAML data, then register natives and call `Build()`:

```go
builder := engine.NewBuilder("myapp", lotaYAML)
builder.RegisterNative("deploy", deployHandler)
app, err := builder.Build()
```

### `engine.NewBuilderFromPath(name, path string) *AppBuilder`

Same as `NewBuilder`, but loads config from a file path.

### `(*AppBuilder) RegisterNative(name string, fn NativeFunc) *AppBuilder`

Registers a native Go handler for a command path. For root commands use the command name (`"deploy"`). For commands inside groups use the dot-separated path (`"infra.status"`). Can be called multiple times. Handlers are scoped to the `App` built from this builder, not global.

### `(*AppBuilder) Build() (*App, error)`

Parses config, registers all natives, sets default writers, and returns the `*App`.

### `engine.NewApp(data []byte, opts Options) (*App, error)`

Low-level wrapper when you don't need the builder. Use `NewBuilder` for most cases.

### `engine.NewAppFromPath(path string, opts Options) (*App, error)`

Same as `NewApp`, but loads from a file path and automatically sets `ConfigDir`.

## Comparison with `cli.Run`

|                 | `cli.Run(ctx)`                | `engine.Run(ctx, cfg, args, opts)` |
| --------------- | ----------------------------- | ---------------------------------- |
| `os.Args`       | Reads directly                | Accepts explicit `[]string`        |
| `os.Exit`       | Uses `os.Exit` for completion | Never calls `os.Exit`              |
| Output          | Hardcoded `os.Stdout/Stderr`  | Configurable `io.Writer`           |
| Config source   | Filesystem only               | Can use `[]byte` / `io.Reader`     |
| Help/Completion | Built-in                      | `engine.PrintHelp` (no CLI flags)  |
| Use case        | Standalone binary             | Embedded library                   |

## Advanced: Custom prefix formatter

When dependencies run in parallel, Lota prefixes each line of output with the dependency path. You can customize this prefix (e.g., add ANSI colors):

```go
opts := engine.Options{
    Stdout: os.Stdout,
    Stderr: os.Stderr,
    PrefixFormatter: func(path string, cmd *config.Command, groups []*config.Group) string {
        return fmt.Sprintf("\033[36m[%s]\033[0m", path)
    },
}
```

## Advanced: Programmatic command lookup

If you need to inspect the configuration before running:

```go
result, err := config.FindCommandByPath(cfg, "infra.docker.up")
if err != nil {
    // handle error
}

fmt.Println("Command:", result.Command.Name)
fmt.Println("Script:", result.Command.Script)

// Run it manually
err = engine.RunCommand(ctx, cfg, "infra.docker.up", nil, opts)
```

## Advanced: Native commands

You can declare commands in `lota.yml` that execute Go code instead of shell scripts. This is useful when you need direct programmatic control (e.g., calling internal APIs, databases, or complex logic) while keeping the declarative configuration.

### Declaring a native command

```yaml
deploy:
  desc: Deploy via native Go code
  native: true
  vars:
    - REGION=eu-west-1
  args:
    - "env:str=dev"
```

The `native: true` marker tells Lota to look up a registered Go handler instead of running a shell script. The command can still use `vars`, `args`, `depends`, and `parallel` like any other command.

### Registering a handler

```go
builder := engine.NewBuilder("myapp", lotaYAML)

builder.RegisterNative("deploy", func(ctx context.Context, nctx engine.NativeContext) error {
    env := nctx.Args["env"]       // "dev" or "prod"
    region := nctx.Vars["REGION"] // "eu-west-1"

    fmt.Fprintf(nctx.Stdout, "Deploying to %s in %s\n", env, region)
    // Any Go code: HTTP requests, database calls, etc.
    return nil
})

app, err := builder.Build()
```

For commands inside groups use the full dot-separated path:

```go
builder.RegisterNative("infra.status", statusHandler)
```

### `engine.NativeContext`

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
| `Vars`   | Resolved variables (app → group → command). |
| `Args`   | Parsed CLI arguments with defaults applied. |
| `Stdout` | Where to write command output.              |
| `Stderr` | Where to write errors and warnings.         |

### Error handling

If a command is marked `native: true` but no handler was registered, `engine.Run` returns an error:

```
native command "deploy" has no registered handler
```

Register handlers on the `AppBuilder` before calling `Build()`, or pass them via `Options.NativeHandlers` when using `engine.Run` directly. The registry is per-app, not global.

### Native commands as dependencies

Native commands can be dependencies of shell commands (and vice versa). Lota resolves variables and arguments for each command independently, then calls either the Go handler or the shell executor.

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
    // Run Go build
    return nil
})
```

## Troubleshooting

### "command not found" errors

Make sure you called `cfg.BuildIndexes()` after parsing. `engine.LoadConfig` does this for you.

### No output captured

Ensure you set `Stdout` and `Stderr` in `engine.Options`. If left nil, output goes to `os.Stdout`/`os.Stderr` of the host process.

### PTY not used when embedding

If you pass a custom `io.Writer` (e.g., `bytes.Buffer`), Lota cannot allocate a pseudo-terminal because it must tee output into your writer. If you need TTY detection (e.g., for colored child output), pass `os.Stdout`/`os.Stderr` directly.

### Binary size

Lota `engine` itself is tiny (only `yaml.v3` as a non-std dependency). Most of the binary size comes from your application code. To reduce it:

```bash
# Strip debug info and symbol table
go build -ldflags="-s -w" -o outless ./cmd/outless

# Compress with UPX (optional, reduces ~50-70%)
upx --best outless
```

If you only use native commands (no shell scripts), `github.com/creack/pty` is excluded on non-Unix builds automatically via build tags.
