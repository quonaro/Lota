# Execution, Architecture & Examples

How Lota executes commands, manages processes, and is structured internally.

---

## Table of Contents

- [Execution Hooks](#execution-hooks)
- [Command Dependencies](#command-dependencies)
- [Parallel Execution](#parallel-execution)
- [Shell Resolution](#shell-resolution)
- [Working Directory Resolution](#working-directory-resolution)
- [Tee Logging](#tee-logging)
- [Graceful Shutdown](#graceful-shutdown)
- [PTY Allocation](#pty-allocation)
- [Native Commands](#native-commands)
- [Architecture](#architecture)
- [Examples Walkthrough](#examples-walkthrough)

---

## Execution Hooks

Each command supports five execution stages. You only use what you need — a simple `script` is enough for most tasks.

### Stage Flow

```
before → script → after → finally
          ↓
    fallback → finally
```

| Stage          | Purpose                                                | Runs on error?                      |
| -------------- | ------------------------------------------------------ | ----------------------------------- |
| **`before`**   | Preparation (compile, check env, start services)       | Skips `script`, triggers `fallback` |
| **`script`**   | Main command                                           | Triggers `fallback`                 |
| **`after`**    | Post-success action (notify, log, cache)               | Triggers `fallback`                 |
| **`fallback`** | Recovery / alternative path (rollback, alert, degrade) | If succeeds, command returns `0`    |
| **`finally`**  | Cleanup (stop containers, remove temp files)           | Always runs                         |

### Return Codes

- `0` if `before` + `script` + `after` all succeeded
- `0` if any stage failed but `fallback` succeeded
- Otherwise: the first error's exit code
- `finally` errors are printed to stderr but do **not** change the return code

### When `before` Fails

If `before` fails, `script` is **not** executed. `fallback` runs (if defined). `finally` runs (if defined).

### When `script` Fails

If `script` fails, `after` is **not** executed. `fallback` runs (if defined). `finally` runs (if defined).

### When `fallback` Fails

If `fallback` also fails, the command returns `fallback`'s exit code. `finally` still runs.

### When `finally` Fails

`finally` errors are printed to stderr as `[finally error]` but do not change the return code.

### Examples

**Basic pipeline:**

```yaml
build:
  before: echo "Compiling..."
  script: go build -o bin/app .
  after: echo "Build complete"
```

**Cleanup with `finally`:**

```yaml
test:
  before: docker-compose up -d test-db
  script: go test ./...
  finally: docker-compose down test-db
```

**Error handling with `fallback`:**

```yaml
deploy:
  before: echo "Starting deploy..."
  script: ./deploy.sh
  after: echo "Deploy successful"
  fallback: ./rollback.sh
  finally: echo "Deploy finished"
```

**Full pipeline — database migration:**

```yaml
db:
  migrate:
    before: |
      echo "Creating backup..."
      pg_dump mydb > /tmp/backup.sql
    script: |
      echo "Running migrations..."
      migrate -path ./migrations -database "$DATABASE_URL" up
    after: echo "Migration complete"
    fallback: |
      echo "Migration failed, restoring backup..."
      psql mydb < /tmp/backup.sql
    finally: rm -f /tmp/backup.sql
```

---

## Command Dependencies

Commands can declare dependencies — other commands that must run first.

### Syntax

```yaml
build:
  script: go build -o bin/app .

test:
  depends:
    - build
  script: go test ./...

deploy:
  depends:
    - build
    - test
  script: ./deploy.sh
```

Dependencies are specified as **full dot-separated paths**:

```yaml
deploy:
  depends:
    - infra.docker.build
    - ci.lint
  script: ./deploy.sh
```

### Dependency Levels

Lota computes dependency **levels** using topological sorting. Dependencies at the same level can run in parallel.

For example, if `deploy` depends on `build` and `test`, and both have no further dependencies, they are at level 0 and run concurrently. `deploy` is at level 1 and runs after both complete.

### Shared Dependencies

If multiple dependencies share a common sub-dependency, it is executed **once** (deduplicated):

```yaml
build:
  script: go build .

lint:
  depends:
    - build
  script: golangci-lint run

test:
  depends:
    - build
  script: go test ./...

ci:
  depends:
    - lint
    - test
  script: echo "CI done"
```

Here `build` runs once (not twice), even though both `lint` and `test` depend on it.

### Circular Dependency Detection

Circular dependencies are detected automatically:

```yaml
a:
  depends:
    - b
  script: echo a

b:
  depends:
    - a      # circular!
  script: echo b
```

```bash
$ lota a
error: circular dependency detected: a → b → a
```

---

## Parallel Execution

When `parallel: true` (the default), dependencies at the same level run concurrently.

### Colored Output Prefixes

Each parallel task gets a colored prefix for its output, making it easy to distinguish:

```bash
[build]  go build -o bin/app .
[lint]   golangci-lint run
[build]  done
[lint]   done
[deploy] ./deploy.sh
```

The prefix color is determined by:
1. The command's `color` field (if set)
2. Inherited color from parent groups (if `inherit_color: true`)
3. A deterministic color derived from the command name (hash-based)

### Sequential Mode

Set `parallel: false` to run dependencies sequentially:

```yaml
ci:
  depends:
    - lint
    - test
  parallel: false
  script: echo "CI done"
```

In sequential mode, Lota prints `=> Running dependency: <path>` before each dependency:

```bash
=> Running dependency: lint
golangci-lint run
=> Running dependency: test
go test ./...
CI done
```

### Error Handling in Parallel Mode

If any parallel dependency fails, Lota cancels all remaining parallel tasks via context cancellation. The first error is reported. Remaining levels are skipped.

---

## Shell Resolution

Lota resolves the shell in a hierarchical manner: **command > group > app > auto-detect > bash**.

### Auto-Detection

On startup, Lota checks the `$SHELL` environment variable. If set, that shell is used. If not set or the binary is not found, Lota falls back to `bash`.

### Per-Level Override

```yaml
shell: zsh           # app-level default

dev:
  shell: bash        # group-level override
  run:
    shell: sh        # command-level override
    script: echo $0  # outputs "sh"
```

### Supported Shells

bash, sh, zsh, dash, ksh, mksh, pdksh, ash, busybox, sash, tcsh, csh, fish

### Shell-Specific Flags

Lota automatically adds the correct flags for each shell:

| Shell                              | Flag |
| ---------------------------------- | ---- |
| bash, zsh, sh, dash, ksh, ash, busybox | `-c` |
| fish                               | `-c` |
| tcsh, csh                          | `-c` |

### OS-Specific Defaults

On Unix systems, the default shell is `bash`. On Windows, the default is `cmd.exe` (though shell scripts are not guaranteed to work on Windows).

> The script content is **shell-specific**. Write scripts for the shell you target. Lota only selects the interpreter — it does not translate between shell languages.

---

## Working Directory Resolution

The working directory for a command is resolved with priority: **command > group > config file directory**.

### Resolution Rules

1. If the command has `dir`, use it (resolved relative to `lota.yml`)
2. Else if any ancestor group has `dir`, use the nearest one (relative to `lota.yml`)
3. Else use the directory containing `lota.yml`

### `$CWD` Special Variable

Use `$CWD` to reference the current working directory (where `lota` was invoked):

```yaml
build:
  dir: $CWD
  script: make build
```

Or combine with a relative path:

```yaml
build:
  dir: $CWD/subdir
  script: make build
```

### Example

```yaml
# lota.yml is at /project/lota.yml

backend:           # group
  dir: ./backend   # → /project/backend
  build:
    script: go build .    # runs in /project/backend
  test:
    dir: ./backend/tests  # → /project/backend/tests
    script: go test ./... # runs in /project/backend/tests

frontend:
  build:
    dir: $CWD             # → wherever the user ran `lota` from
    script: npm run build
```

---

## Tee Logging

Lota can mirror command output to log files while still printing to the terminal.

### How It Works

When `log` is configured, Lota opens the log file(s) and creates a tee writer that writes to both stdout/stderr and the log file simultaneously. On Unix systems, a PTY is allocated to preserve ANSI colors (see [PTY Allocation](#pty-allocation)).

### Additive Inheritance

Logs are **additive**: a command writes to its own log file **plus** all ancestor log files.

```yaml
log:
  path: logs/all.log        # app-level: all commands write here

infra:
  log:
    path: logs/infra.log    # group-level: infra commands write here + all.log
  docker:
    log:
      path: logs/docker.log # command-level: writes here + infra.log + all.log
    up:
      script: docker-compose up -d
```

### `independent: true`

Discards all ancestor logs. The command writes **only** to its own `path`:

```yaml
log:
  path: logs/all.log

infra:
  log:
    path: logs/infra.log
    independent: true       # infra commands skip all.log
  docker:
    log:
      path: logs/docker.log # writes to infra.log + docker.log (not all.log)
```

> `independent: true` is **not allowed at app level**.

### `truncate: true`

Overwrites the log file on each run instead of appending:

```yaml
log:
  path: logs/build.log
  truncate: true
```

`truncate` applies **only** to the `path` declared on the same level. Ancestor log files are not truncated.

### Error Handling

Runtime errors (missing parent directory, permission denied, path is a directory) are printed to stderr as `[log error]` but **do not fail the command**. The command continues to execute and output to the terminal.

---

## Graceful Shutdown

Lota handles `SIGINT` (Ctrl+C) and `SIGTERM` signals for graceful shutdown.

### Signal Propagation

1. User presses Ctrl+C or sends SIGTERM
2. Lota sends `SIGTERM` to the child process group
3. Lota waits a grace period (a few seconds)
4. If the process hasn't exited, Lota sends `SIGKILL`

### Process Groups

On Unix systems, child processes are started with `Setpgid: true`, creating a new process group. This ensures signals are propagated to all child processes, not just the immediate child.

### Terminal Echo

During command execution, Lota disables `^C` echo on the terminal (via `termios` on Linux/macOS). This prevents `^C` from appearing in the output. The terminal state is restored after execution.

### Context Cancellation

Lota uses `context.Context` for cancellation. When a signal is received:
1. The context is cancelled
2. All running commands receive the cancellation
3. Parallel dependencies are cancelled
4. `finally` hooks still run if the execution stage was reached

### Exit Codes

| Code | Meaning                                              |
| ---- | ---------------------------------------------------- |
| `0`  | Success                                              |
| `1`  | Command execution failed (non-zero exit from script) |
| `2`  | Configuration error (parse error, validation error)  |
| `130`| Interrupted by user (SIGINT / Ctrl+C)                |

---

## PTY Allocation

On Unix systems (Linux, macOS), Lota allocates a pseudo-terminal (PTY) for command execution when tee logging is active.

### Why PTY?

Many programs check `isatty()` before printing colored output. When output is piped to a file (as with tee logging), `isatty()` returns false and colors are disabled. PTY allocation tricks the child process into thinking it's connected to a terminal, preserving ANSI colors.

### Fallback

If PTY allocation fails (e.g., insufficient permissions, too many open PTYs), Lota falls back to normal pipes. The command still executes, but colors may be lost in the log file.

### Platform Support

| Platform | PTY Support |
| -------- | ----------- |
| Linux    | Yes         |
| macOS    | Yes         |
| Windows  | No (stubs)  |

On Windows, PTY-related functions are no-ops. Commands run with normal pipes.

---

## Native Commands

Commands marked with `native: true` are executed as Go functions instead of shell scripts. This is only available when embedding Lota as a Go library.

### Configuration

```yaml
server:
  desc: Start HTTP server
  native: true
  args:
    - port:int=8080
    - host:str=0.0.0.0
```

### Go Handler Registration

```go
app, err := engine.NewAppBuilder(cfg).
    Native("server", func(ctx engine.NativeContext) error {
        port := ctx.Args["port"].(int)
        host := ctx.Args["host"].(string)
        fmt.Fprintf(ctx.Stdout, "Starting server on %s:%d\n", host, port)
        return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil)
    }).
    Build()
```

### NativeContext

| Field    | Type                | Description                              |
| -------- | ------------------- | ---------------------------------------- |
| `Vars`   | `map[string]string` | Resolved variables (app + group + cmd)   |
| `Args`   | `map[string]any`    | Parsed arguments (typed)                 |
| `Stdout` | `io.Writer`         | Output writer                            |
| `Stderr` | `io.Writer`         | Error output writer                      |

### Limitations

- Native commands are **not available in CLI mode** — only when embedded
- Native commands do not support `before`/`after`/`fallback`/`finally` hooks
- Native commands do not support `depends` (dependencies)
- Native commands do not support tee logging

See the [Embedding Guide](EMBEDDING.md) for complete details.

---

## Architecture

Lota follows a strict layered architecture with clear separation of concerns.

### Layer Diagram

```
┌─────────────────────────────────────────────────┐
│                   main.go                        │
│  (signal handling, terminal setup, exit codes)   │
└────────────────────┬────────────────────────────┘
                     │
              ┌──────▼──────┐
              │    cli/      │
              │  (orchestrator)│
              └──┬───────┬───┘
                 │       │
          ┌──────▼──┐ ┌──▼──────┐
          │ config/  │ │ engine/  │
          │ (parse)  │ │ (orchestrate)│
          └─────────┘ └──┬──────┘
                        │
                   ┌────▼────┐
                   │ runner/  │
                   │ (execute)│
                   └──────────┘
```

### Packages

#### `config/`

YAML parsing, configuration models, and validation.

- **`types.go`** — Core data structures: `AppConfig`, `Group`, `Command`, `Arg`, `Var`, `LogConfig`, `ImportConfig`
- **`parser.go`** — YAML parsing with `yaml.Node` handling, validation for colors and reserved variable names, suggestions for unknown fields
- **`loader.go`** — Config file discovery (upward search), URL fetching with caching, path resolution
- **`validator.go`** — Orchestrates validation: imports, env expansion, index building, minimum command check
- **`imports.go`** — Import processing and merging with namespacing support
- **`env.go`** — `.env` and YAML file variable loading with prefix/section support
- **`navigation.go`** — Command tree traversal, dependency resolution with cycle detection
- **`http.go`** — HTTP/HTTPS fetching with caching and size limits
- **`yaml.go`** — YAML node flattening to dot-notation keys

#### `runner/`

Command execution, argument parsing, and interpolation.

- **`runner.go`** — Core execution: `ExecuteCommand`, `ExecuteCommandWithPrefix`, shell command splitting, log file management, `PrefixWriter` for parallel output, `ShellError` for non-zero exits
- **`args.go`** — Argument parsing: positional, flag, boolean, wildcard, array types with defaults and validation
- **`interpolator.go`** — Variable interpolation: `$name` and `{{name}}` syntax, type-aware validation, local shell var detection, system env var detection, cycle detection (max depth 10)
- **`resolver.go`** — Scope resolution: `ResolveVars`, `ResolveArgs`, `ResolveDir`, `ResolveShell`, `ResolveLogs` with hierarchical merging
- **`pty.go`** — PTY allocation for ANSI color preservation (Unix only)
- **`runner_unix.go`** — Process group management, terminal control (`IsTerminal`, `DisableSignalEcho`, `RestoreTerminal`)
- **`runner_linux.go`** / **`runner_darwin.go`** — Platform-specific `ioctl` constants
- **`runner_windows.go`** — Windows stubs for process and terminal management

#### `engine/`

High-level orchestration used by both CLI and embedded API.

- **`engine.go`** — `LoadConfig`, `LoadConfigFromPath`, `Run` (CLI entrypoint), `RunCommand` (programmatic entrypoint), dependency level resolution, parallel/sequential execution
- **`app.go`** — `App` and `AppBuilder` for convenient embedded usage
- **`native.go`** — `NativeContext`, `NativeFunc`, native handler lookup and execution
- **`help.go`** — Help message generation with color resolution and formatting
- **`completion.go`** — Shell completion tree building, positional completion hints, script generation for bash/zsh/fish
- **`errors.go`** — `GroupError` for when a user tries to execute a group as a command

#### `cli/`

CLI orchestration — the top-level coordinator.

- **`cli.go`** — Main `Run` function: flag parsing, config loading, command resolution, execution orchestration
- **`flags.go`** — `GlobalFlags` struct, flag parsing and validation, `HandleGlobalFlags` for `--help`/`--version`/`--init`
- **`lookup.go`** — `LoadConfig` and `LoadConfigWithWriter` for loading and validating config
- **`output.go`** — All CLI output: help messages, version display, error reporting, colorization, `InitConfig` for `--init`
- **`suggestions.go`** — "Did you mean?" command suggestions using Levenshtein distance
- **`completion.go`** — Shell completion subcommand, script installation, zsh fpath management

#### `shared/`

- **`constants.go`** — `AppName`, `ConfigFileName` (`lota.yml`), `ConfigFileNameYAML` (`lota.yaml`), version info (`Version`, `Commit`, `BuildTime`)

#### `logger/`

Debug logging with verbose mode support. Used internally by `engine/` and `runner/` for debug output when `-v` / `--verbose` is active.

#### `internal/`

Internal utilities not exposed as public API:

- **`levenshtein/`** — Levenshtein distance calculation for command and flag suggestions
- **`complete/`** — Argument parsing and completion engine (wraps `posener/complete`)

### Layering Rules

- `runner` **must not** import `cli`
- `config` **must not** import `runner`
- `cli` is the top-level orchestrator that binds `config` and `engine`
- `engine` bridges `config` and `runner`

### Design Principles

- **Stateless** — No global variables. State (verbose, dry-run) is passed through context or arguments.
- **Context-aware** — Execution uses `exec.CommandContext` for graceful shutdown.
- **Pure functions** — Interpolation and parsing logic accepts data via arguments, not file I/O. This makes them testable without filesystem access.
- **Error wrapping** — Errors are wrapped with context: `fmt.Errorf("context: %w", err)`. No `panic()` in library code.
- **Cycle detection** — Both variable interpolation (max depth 10) and command dependencies are checked for cycles.

---

## Examples Walkthrough

### simple-web-project.yaml

A basic web development workflow demonstrating env imports, typed args, and conditional execution.

```yaml
vars:
  - import:env .env
  - PORT=3000

args:
  - mode|m:str=dev

dev:
  desc: Start development server
  script: npm run dev -- --port $PORT

build:
  desc: Build for production
  args:
    - minify|:bool
  script: |
    if [ "$minify" = "true" ]; then
      npm run build -- --minify
    else
      npm run build
    fi

test:
  desc: Run tests
  script: npm test
```

**Key features:**
- `import:env .env` loads variables from a `.env` file
- `mode|m:str=dev` — a flag with short alias and default value
- `minify|:bool` — a boolean flag (note: `|` with empty short name makes it a flag-only arg)
- Conditional logic inside the script using shell `if`

### go-project.yaml

Go build, test, and release workflows with variables, arguments, and dependencies.

```yaml
vars:
  - BINARY=app
  - VERSION=$(git describe --tags --always)

build:
  desc: Build binary
  args:
    - os|o:str=linux
    - arch|a:str=amd64
  script: |
    GOOS=$os GOARCH=$arch go build -o bin/$BINARY-$os-$arch .

test:
  desc: Run tests
  depends:
    - build
  script: go test -v ./...

release:
  desc: Create release
  args:
    - tag|t:str
  script: |
    git tag $tag
    git push origin $tag
    goreleaser release
```

**Key features:**
- `$(git describe ...)` — shell command substitution in variable values (executed by the shell at runtime)
- `depends` — `test` depends on `build`
- Multiple flag args with short aliases

### devops-infrastructure.yaml

Docker and Kubernetes operations with nested groups, wildcard args.

```yaml
vars:
  - DOCKER=docker compose

docker:
  desc: Docker operations
  up:
    desc: Start services
    script: $DOCKER up -d
  down:
    desc: Stop services
    script: $DOCKER down
  logs:
    desc: View logs
    args:
      - service:str
      - ...tail
    script: $DOCKER logs "$service" $tail

k8s:
  desc: Kubernetes operations
  apply:
    desc: Apply manifests
    script: kubectl apply -f k8s/
  scale:
    desc: Scale deployment
    args:
      - deployment:str
      - replicas|int=1
    script: kubectl scale deployment $deployment --replicas=$replicas
```

**Key features:**
- Nested groups: `docker` and `k8s` are top-level groups
- Wildcard arg: `...tail` captures all remaining args for `docker logs`
- `replicas|int=1` — typed arg with default (note: `|int` without short name)

### multi-environment.yaml

Environment-specific configurations using imports and conditional execution.

```yaml
vars:
  - import:yaml environments.yaml@dev dev
  - import:yaml environments.yaml@prod prod

args:
  - env|e:str=dev

deploy:
  desc: Deploy application
  script: |
    if [ "$env" = "prod" ]; then
      echo "Deploying to production at $prod.host:$prod.port"
      kubectl apply -f k8s/prod/
    else
      echo "Deploying to dev at $dev.host:$dev.port"
      kubectl apply -f k8s/dev/
    fi
```

**Key features:**
- `import:yaml environments.yaml@dev dev` — imports the `dev` section with `dev.` prefix
- Conditional execution based on the `env` argument
- Accessing nested YAML values: `$dev.host`, `$prod.port`

### embedded.yaml

A configuration designed for embedded Lota usage with native commands, colors, and groups.

```yaml
shell: bash
color: cyan

vars:
  - APP_NAME=myapp

args:
  - env|e:str=dev

server:
  desc: Server commands
  color: green
  start:
    desc: Start server
    native: true
    args:
      - port|int=8080
  stop:
    desc: Stop server
    native: true

db:
  desc: Database commands
  color: yellow
  migrate:
    desc: Run migrations
    script: migrate -path ./migrations -database $DATABASE_URL up
  seed:
    desc: Seed database
    script: |
      echo "Seeding $APP_NAME in $env mode..."
      psql $DATABASE_URL < seed.sql
    log:
      path: logs/seed.log
```

**Key features:**
- `native: true` commands — executed as Go functions when embedded
- Color inheritance: `server` group has `color: green`, `db` has `color: yellow`
- `log` configuration on `seed` command
- App-level `color: cyan` for uncolored groups/commands

See the [Embedding Guide](EMBEDDING.md) for how to use this configuration in a Go application.
