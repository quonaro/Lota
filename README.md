# Lota

A configurable task runner for rapid development. Define commands in a YAML file and run them from the terminal.

## Features

- ✨ **Configurable tasks** — Define tasks and command groups in a single `lota.yml`. No code required: commands, arguments, variables, dependencies and execution hooks are all declared in YAML.
- 🔧 **Flexible arguments** — Support positional arguments, named flags (`-f`/`--flag`), boolean flags with negation (`--!verbose`), wildcard capture (`...args`) and typed arrays (`files:arr[5]`). CLI values are validated and exported as environment variables.
- 🔄 **Variable interpolation** — Inject variables and arguments into scripts with `$name`. Scopes follow app → group → command, so local definitions naturally override global ones.
- 🐚 **Shell-aware execution** — Auto-detects the system shell and falls back to `bash` if unsure. Override the shell per app, group or command. Supports bash, zsh, fish, sh, dash and others.
- 👁️ **Dry-run mode** — Run with `--dry-run` to print the exact scripts and resolved environment without executing them.
- 🛡️ **Graceful shutdown** — `SIGINT`/`SIGTERM` are propagated to child processes with a graceful period before forced termination.
- 📄 **Env file imports** — Load variables from `.env` files via `import:env .env` inside `vars`.
- 📊 **YAML config imports** — Import nested YAML files as variables with `import:yaml <file>[@section] [prefix]` and access flattened keys like `$app.database.host`.
- 📂 **Nested groups** — Organize commands in hierarchical groups (`lota infra docker up`) with arbitrary nesting depth.
- 📁 **Working directory** — Set `dir` per group or command. Relative paths resolve against the config file; use `$CWD` to run from the current directory.
- 📝 **Tee logging** — Mirror command output to log files while still printing to the terminal. Logs are inherited down the group tree and can be made `independent`.
- 🔗 **Command dependencies** — Declare `depends` with full dot-paths. Independent dependencies run in parallel by default, shared ones run once, and circular dependencies are detected automatically.
- 🩺 **Service readiness** — Docker Compose-style `wait` modes (`done`, `ready`, `started`) plus a `ready` probe block, so services can gate dependents on actual health, not just process start.
- 🔍 **Upward config search** — Run commands from any subdirectory: Lota walks up looking for `lota.yml`/`lota.yaml`. A `.git` directory is a soft boundary — Lota still checks one level above it, so a shared config works for nested repos.
- 🎨 **Colored help** — Highlight group and command names with named ANSI colors or hex values (e.g. `#FF5733`).
- ⏱️ **Command timeout** — Set a per-run timeout with `--timeout 5m`.
- 🧩 **Execution hooks** — Build pipelines with `before`, `script`, `after`, `fallback` and `finally` stages.
- 🧱 **Native commands** — Embed Lota as a Go library and register Go functions as handlers for `native: true` commands.
- 🎯 **Shell completion** — Built-in completion for bash, zsh and fish.
- 💡 **Smart suggestions** — Typo a command name? Lota suggests the closest match using Levenshtein distance ("Did you mean: ...?").
- 📥 **Config imports** — Merge other Lota config files or remote URLs with optional namespacing via `imports`.
- 🖥️ **PTY support** — Pseudo-terminal allocation preserves ANSI colors when tee-logging to files.
- 👻 **Hidden commands** — Set `show: false` to keep commands executable but out of help output.

## 📦 Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/quonaro/lota/main/scripts/install.sh | bash
```

Or with specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/quonaro/lota/main/scripts/install.sh | bash -s -- -V v0.1.0
```

Script verifies SHA256 checksum and installs to `~/.local/bin` (or `/usr/local/bin` with sudo).

### Build from Source

Requires Go 1.26+

```bash
go install github.com/quonaro/lota@latest
```

Or manually:

```bash
git clone https://github.com/quonaro/lota.git
cd lota && go build -o lota . && sudo mv lota /usr/local/bin/
```

## 📚 Documentation

- **[Configuration Reference](docs/CONFIGURATION.md)** — Complete reference for every YAML field
- **[CLI Usage](docs/CLI.md)** — All CLI flags, config resolution, completion, exit codes
- **[Arguments, Variables & Interpolation](docs/ARGUMENTS_VARS.md)** — Deep dive into args, vars, and `$name` interpolation
- **[Execution, Architecture & Examples](docs/EXECUTION.md)** — Hooks, dependencies, PTY, architecture, example walkthroughs
- **[Embedding Guide](docs/EMBEDDING.md)** — How to embed Lota in your Go application
- **[Examples](examples/)** — Complete configuration examples for various use cases

## 🚀 Quick Start

Initialize a new configuration:

```bash
lota --init
```

This creates a `lota.yml` in your current directory.

Or create it manually:

```yaml
build:
  desc: Build the application
  script: go build -o bin/app .

dev:
  desc: Development commands
  run:
    desc: Run with hot reload
    script: air
  test:
    desc: Run tests
    script: go test ./...
```

Run a command:

```bash
lota build
lota dev run
lota dev test
```

## Comparison

| Feature                   | Lota | Task | Just | npm scripts |
| ------------------------- | ---- | ---- | ---- | ----------- |
| Declarative YAML          | ✅   | ✅   | ✅   | ❌          |
| Type-safe arguments       | ✅   | ✅   | ✅   | ❌          |
| Variable interpolation    | ✅   | ✅   | ✅   | ✅          |
| Nested groups             | ✅   | ✅   | ❌   | ❌          |
| Working directory (`dir`) | ✅   | ✅   | ❌   | ❌          |
| Command dependencies      | ✅   | ✅   | ✅   | ❌          |
| Service readiness probes  | ✅   | ❌   | ❌   | ❌          |
| Upward config search      | ✅   | ❌   | ❌   | ❌          |
| Env file imports          | ✅   | ✅   | ❌   | ❌          |
| Shell auto-detection      | ✅   | ❌   | ❌   | ❌          |
| Dry-run mode              | ✅   | ✅   | ❌   | ❌          |
| Tee logging               | ✅   | ❌   | ❌   | ❌          |
| Execution hooks           | ✅   | ❌   | ❌   | ❌          |
| Native Go commands        | ✅   | ❌   | ❌   | ❌          |
| Config file imports       | ✅   | ✅   | ❌   | ❌          |

### Syntax Comparison

**Simple build command:**

```yaml
# Lota
build:
  script: go build -o app .

# Task (Taskfile.yml)
build:
  cmds:
    - go build -o app .

# Just
build:
    go build -o app .

# npm scripts
"build": "go build -o app ."
```

**With arguments:**

```yaml
# Lota
dev:
  args:
    - port|p:int=3000
  script: npm start -- --port $port

# Task (Taskfile.yml)
build:
  vars:
    PORT: 3000
  cmds:
    - npm start -- --port {{.PORT}}

# Just
port := "3000"
dev:
    npm start -- --port {{port}}

# npm scripts
"dev": "npm start -- --port ${PORT:-3000}"
```

**With dependencies:**

```yaml
# Lota
test:
  depends:
    - build
  script: go test ./...

# Task (Taskfile.yml)
test:
  deps: [build]
  cmds:
    - go test ./...

# Just
build:
    go build -o app .

test: build
    go test ./...

# npm scripts
"test": "npm run build && go test ./..."
```

## Examples

See the [examples/](examples/) directory for complete working configurations:

- **[embedded.yaml](examples/embedded.yaml)** - Full-featured example with native commands, groups, colors, variables, and arguments
- **[simple-web-project.yaml](examples/simple-web-project.yaml)** - Web development workflow
- **[devops-infrastructure.yaml](examples/devops-infrastructure.yaml)** - Docker and Kubernetes operations
- **[go-project.yaml](examples/go-project.yaml)** - Go build, test, and release workflows
- **[multi-environment.yaml](examples/multi-environment.yaml)** - Environment-specific configurations

## ⚙️ Configuration

### 📋 Structure

```yaml
shell: bash # Optional: default shell (auto-detected if omitted)

vars: # global environment variables
  - KEY=value
  - import:env .env # Import from .env file
  - import:yaml config.yaml # Import from YAML file

args: # global argument definitions
  - name|short:type=default

imports: # optional: import other config files / URLs
  - url: extra.yaml
    namespace: ext # optional; wraps imported commands/groups under `ext:`

group-name: # command group
  desc: ...
  color: cyan # Optional: highlight group name in help
  inherit_color: true # Optional: inherit color from parent group
  show: false # Optional: hide from help output
  shell: sh # Optional: override shell for this group
  dir: ./subdir # Optional: working directory for this group
  vars: # group-level variables
    - KEY=value
  args: # group-level arguments
    - name|short:type=default
  log: # Optional: group-level tee logging
    path: group.log
  nested-group: # nested group
    desc: ...
    command-name: # command inside group
      desc: ...
      color: green # Optional: highlight command name in help
      inherit_color: true # Optional: inherit color from parent group
      show: false # Optional: hide from help output
      shell: sh # Optional: override shell for this command
      dir: ./src # Optional: working directory for this command
      vars: # command-level variables
        - KEY=value
      args: # command-level arguments
        - name|short:type=default
      before: ... # Optional: pre-execution hook
      script: ... # Main script
      after: ... # Optional: post-success hook
      fallback: ... # Optional: error recovery hook
      finally: ... # Optional: always-runs cleanup hook
      depends: # Optional: commands to run before this one
        - other-command
        - name: infra-dev # mapping form with wait mode
          wait: ready # done (default) | ready | started
      parallel: true # Optional: run dependencies in parallel (default: true)
      ready: # Optional: readiness probe used by `wait: ready` dependents
        script: curl -sf http://localhost:9000/health
        interval: 2s # Optional (default: 1s)
        timeout: 5s # Optional per-attempt timeout (default: 5s)
        retries: 30 # Optional (default: 30)
        start_period: 10s # Optional: early failures don't count
      native: true # Optional: mark as native Go command (embedded mode only)
      log: # Optional: command-level tee logging
        path: cmd.log
        truncate: true
        independent: true

command-name: # top-level command
  desc: ...
  color: red # Optional: highlight command name in help
  script: ...
  before: ... # Optional: pre-execution hook
  after: ... # Optional: post-success hook
  fallback: ... # Optional: error recovery hook
  finally: ... # Optional: always-runs cleanup hook
  log: # Optional: command-level tee logging
    path: cmd.log
```

### 🔑 Variables (`vars`)

Variables are exported as environment variables into scripts. Both `vars` and `args` share a unified environment pool — CLI args override vars on name collision. They support three scopes with priority: **app < group < command**.

**Reserved system variables** cannot be overridden in `vars`:

`PATH`, `HOME`, `USER`, `SHELL`, `LANG`, `LC_ALL`, `TERM`, `PWD`, `OLDPWD`, `HOSTNAME`, `LOGNAME`, `MAIL`, `TMPDIR`, `DISPLAY`, `XAUTHORITY`, `EDITOR`, `VISUAL`, `PAGER`

**Special variables:** `$CWD` is reserved for working directory resolution (see [Working Directory](#-working-directory-dir)).

```yaml
vars:
  - DOCKER=docker compose # app-level

dev:
  vars:
    - DOCKER=docker # overrides app-level for this group
  run:
    vars:
      - DOCKER=podman # overrides group-level for this command
    script: $DOCKER up
```

#### 📄 Import from .env files

Load variables from environment files:

```yaml
vars:
  - import:env .env
  - import:env config/prod.env
```

#### 📊 Import from YAML files

Import nested YAML configurations with automatic flattening to dot-notation:

```yaml
vars:
  - import:yaml config.yaml # Import all with original keys
  - import:yaml config.yaml app # Import all with 'app.' prefix
  - import:yaml config.yaml@public # Import only 'public' section
  - import:yaml secrets.yaml@db cfg # Import 'db' section with 'cfg.' prefix
```

**Syntax:** `import:yaml <file>[@<section>] [<prefix>]`

> **Note:** The old `!import:yaml` syntax is deprecated but still supported. Use `import:yaml` instead.

- **file** - Path to YAML file
- **section** (optional) - Import only specific top-level section via `@section`
- **prefix** (optional) - Add prefix to all imported keys

**Example YAML file:**

```yaml
# config.yaml
public:
  app_name: MyApp
  version: 1.0.0
  database:
    host: localhost
    port: 5432

private:
  api_key: secret123
```

**Resulting variables:**

```yaml
# import:yaml config.yaml@public app
vars:
  app.app_name: "MyApp"
  app.version: "1.0.0"
  app.database.host: "localhost"
  app.database.port: "5432"
```

Access in scripts: `$app.app_name`, `$app.database.host`

### 🎯 Arguments (`args`)

Arguments are passed from the CLI and exported as environment variables, accessible via `$name` in scripts.

**Format:** `name|short:type=default`

| Part       | Description                  | Example                         |
| ---------- | ---------------------------- | ------------------------------- |
| `name`     | Long name                    | `output`                        |
| `\|short`  | Short alias (optional)       | `\|o`                           |
| `:type`    | Type (optional)              | `:str`, `:int`, `:bool`, `:arr` |
| `:type?`   | Optional marker (no default) | `:str?` — arg not required      |
| `=default` | Default value (optional)     | `=./bin`                        |

> A default value (`=default`) automatically makes an argument optional. The `?` marker is useful when you want an argument to be optional **without** a default value (it will be empty string if not provided).

#### 📝 Argument Types

**Positional** — passed by position, no flag needed:

```yaml
args:
  - filename:str
  - count:int
script: process "$filename" "$count"
```

```bash
lota cmd file.txt 5
```

**Flag** — passed by name using `--flag` or `-f`. Any arg with a short alias (`|short`) or type `bool` becomes a flag:

```yaml
args:
  - output|o:str=./bin
  - verbose|v:bool
script: go build -o "$output"
```

```bash
lota cmd --output ./dist
lota cmd -o ./dist --verbose
```

**Wildcard** — captures all remaining positional arguments:

```yaml
args:
  - service:str
  - ...cmd
script: docker exec "$service" "$cmd"
```

```bash
lota cmd backend python manage.py shell
# service=backend, cmd="python manage.py shell"
```

**Array** — collects multiple consecutive positional values:

```yaml
args:
  - files:arr[5] # collect up to 5 values
script: lint $files
```

```bash
lota cmd a.go b.go c.go
```

#### Boolean Flags

Bool args support negation via `--!name`:

```bash
lota cmd --verbose          # verbose=true
lota cmd --!verbose         # verbose=false
lota cmd --verbose=false    # verbose=false
```

#### Flag Syntax

Flags can be passed in several forms:

```bash
lota cmd --output ./dist     # space-separated
lota cmd --output=./dist     # equals-separated
lota cmd -o ./dist           # short flag
lota cmd -o=./dist           # short flag, equals-separated
```

Use `--` to signal end of flags — everything after is treated as positional arguments:

```bash
lota cmd -- --flag-like-value
# '--flag-like-value' is passed as a positional arg, not parsed as a flag
```

#### Argument Scopes

Like vars, args can be defined at app, group, or command level and are merged with the same priority (command wins):

```yaml
args:
  - env:str=dev # available to all commands

deploy:
  args:
    - env:str=prod # overrides app-level for this group
  run:
    script: ./deploy.sh --env="$env"
```

> **Deprecation:** Using `{{name}}` for variable and argument interpolation is deprecated. Use `$name` instead. `{{name}}` will be removed in a future version.

### 📥 Config Imports (`imports`)

Import other Lota configuration files and merge them into the current config. Supports local files and remote URLs.

```yaml
imports:
  - url: ./extra.yaml # local file (relative to lota.yml)
  - url: https://example.com/shared.yaml # remote URL
    namespace: shared # wrap imported commands/groups under 'shared'
```

| Field       | Type   | Description                                                                                 |
| ----------- | ------ | ------------------------------------------------------------------------------------------- |
| `url`       | string | Path to local file or HTTP/HTTPS URL                                                        |
| `namespace` | string | Optional. Wraps all imported commands and groups under a new top-level group with this name |

**Merge rules:**

- **Without namespace:** Imported commands and groups merge directly into the root. Local definitions override imported ones with the same name.
- **With namespace:** Imported commands and groups are wrapped under a new top-level group named after the namespace.
- **Variables:** Always merged into root (no namespace wrapping). Local vars override imported vars.
- **Nested imports:** Imported configs cannot have their own `imports` field (silently ignored).

**URL fetching:**

- Configs are downloaded with a 100MB size limit
- Cached at `/tmp/lota_cache/<sha256>.yml` for subsequent runs
- TLS verification is skipped (use with trusted sources only)

### 🐚 Shell Configuration

**Important:** Lota selects the shell interpreter, but the script itself is **shell-specific**. Write scripts for the shell you target.

Lota auto-detects the shell binary from the system environment. If detection fails, it falls back to `bash`.

Override the shell at any level:

```yaml
shell: zsh # app-level

dev:
  shell: bash # group-level override
  run:
    shell: sh # command-level override
    script: echo $0
```

Supported shells: bash, sh, zsh, dash, ksh, mksh, pdksh, ash, busybox, sash, tcsh, csh, fish

### 📁 Working Directory (`dir`)

Set the working directory for commands and groups. The path is resolved relative to the `lota.yml` file location.

```yaml
backend:
  dir: ./backend # group-level default
  build:
    desc: Build backend
    script: go build .
  test:
    desc: Run backend tests
    dir: ./backend/tests # command-level override
    script: go test ./...
```

Priority: **command > group > config file dir**. Use `$CWD` or `$CWD/...` to target the current working directory. Useful in monorepos where different commands run in different subprojects.

### 🔗 Command Dependencies (`depends`)

Reference other commands that must run before the current one. Dependencies are specified as full dot-separated paths.

```yaml
build:
  desc: Build the application
  script: go build -o bin/app .

test:
  desc: Run tests
  depends:
    - build
  script: go test ./...

deploy:
  desc: Deploy to production
  depends:
    - build
    - test
  script: ./deploy.sh
```

Dependencies execute with **their own context** (shell, vars, dir, default args). Circular dependencies are detected automatically and produce an error.

Independent dependencies run **in parallel by default**, with output prefixed by colored task name (like Docker Compose):

```bash
\x1b[35m[build]\x1b[0m  go build -o bin/app .
\x1b[36m[lint]\x1b[0m   golangci-lint run
\x1b[35m[build]\x1b[0m  ✓ done
\x1b[36m[lint]\x1b[0m   ✓ done
\x1b[33m[deploy]\x1b[0m ./deploy.sh
```

Each task gets its own color: either from its `color` field, inherited from a parent group, or a deterministic color derived from the task name.

To force sequential execution, set `parallel: false`:

```yaml
ci:
  depends:
    - lint
    - test
  parallel: false
  script: echo "CI done"
```

> Shared dependencies are executed **once** (deduplication). If `build` is a dependency of both `test` and `deploy`, running `lota deploy` will execute `build` exactly once.

#### Waiting for services (`wait` + `ready`)

A plain dependency is satisfied when its process **exits** — that doesn't work for long-running services. Like Docker Compose's `depends_on: {condition: service_healthy}`, a command can expose a `ready` probe, and dependents can wait for readiness:

```yaml
infra-dev:
  script: docker compose up
  ready:                                # readiness probe of this command
    script: curl -sf http://localhost:9000/minio/health/live
    interval: 2s                        # optional, default 1s
    timeout: 5s                         # optional per-attempt, default 5s
    retries: 30                         # optional, default 30
    # start_period: 10s                 # optional: warm-up failures don't count

backend-dev:
  script: air -c .air.toml
  depends:
    - name: infra-dev
      wait: ready                       # backend starts only when S3 answers
```

Wait modes: `done` (default — process exits 0), `ready` (probe passes; service keeps running), `started` (launch only, no waiting). When `wait` is omitted the mode is **auto**: `ready` if the target has a `ready` block, `done` otherwise — so plain string entries keep working unchanged.

If the service exits before becoming ready or the probe exhausts `retries`, the run fails and all siblings are cancelled. In `--dry-run` mode probes are skipped.
\
> **TODO:** A TUI for interactive task monitoring is under consideration for future versions.

### ⚡ Hooks Tutorial

Lota provides five execution stages per command. You only use what you need — a simple `script` is enough for most tasks.

```
before → script → after → finally
          ↓
    fallback → finally
```

| Stage          | Purpose                                                | Runs on error?                      |
| -------------- | ------------------------------------------------------ | ----------------------------------- |
| **`before`**   | Preparation (compile, check env)                       | Skips `script`, triggers `fallback` |
| **`script`**   | Main command                                           | Triggers `fallback`                 |
| **`after`**    | Post-success action (notify, log)                      | Triggers `fallback`                 |
| **`fallback`** | Recovery / alternative path (rollback, alert, degrade) | If succeeds, command returns `0`    |
| **`finally`**  | Cleanup (stop containers, remove temp files)           | Always runs                         |

> Return code: `0` if `before`+`script`+`after` succeeded, **or if `fallback` succeeded after a failure**. Otherwise the first error's exit code. `finally` errors are printed to stderr but do not change the return code.

#### Example 1: Basic Pipeline

```yaml
build:
  before: echo "Compiling..."
  script: go build -o bin/app .
  after: echo "Build complete"
```

**Happy path:** `before` → `script` → `after` → return 0

**If `script` fails:** `before` → `script` (exit 1) → return 1. `after` is skipped.

#### Example 2: Cleanup with `finally`

Use `finally` for operations that must run regardless of success or failure.

```yaml
test:
  before: docker-compose up -d test-db
  script: go test ./...
  finally: docker-compose down test-db
```

**Any outcome:** `before` → `script` → `finally` → return 0 or 1. The database container is always stopped.

#### Example 3: Error Handling with `fallback`

Use `fallback` to react to failures — rollback, send alerts, write crash reports.

```yaml
deploy:
  before: echo "Starting deploy..."
  script: ./deploy.sh
  after: echo "Deploy successful"
  fallback: ./rollback.sh
  finally: echo "Deploy finished"
```

**Happy path:** before → script → after → finally → return 0
**Script fails:** before → script (fail) → fallback → finally → return 0 (if fallback succeeds)
**After fails:** before → script → after (fail) → fallback → finally → return 0 (if fallback succeeds)

#### Example 4: Full Pipeline — Database Migration

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

| Scenario        | Flow                                                                           |
| --------------- | ------------------------------------------------------------------------------ |
| Success         | before → script → after → finally (backup deleted)                             |
| Migration fails | before → script (fail) → fallback (restore) → finally (backup deleted)         |
| After fails     | before → script → after (fail) → fallback (restore) → finally (backup deleted) |

### 📝 Tee Logging (`log`)

Write command output to log files while still printing to the terminal. Logs support **additive inheritance**: a command writes to its own log file **plus** all ancestor log files, unless `independent: true` breaks the chain.

```yaml
log:
  path: logs/all.log # app-level: all commands inherit this

build:
  desc: Build the application
  script: go build -o bin/app .
  log:
    path: logs/build.log # writes to both all.log and build.log
    truncate: true # overwrite on each run (default: append)

test:
  desc: Run tests
  script: go test ./...
  log:
    path: logs/test.log
    independent: true # writes ONLY to test.log, skips all.log
```

| Field         | Type   | Default      | Description                                                                                     |
| ------------- | ------ | ------------ | ----------------------------------------------------------------------------------------------- |
| `path`        | string | **required** | Log file path (relative to `lota.yml`). Supports variable interpolation (`$var`).               |
| `truncate`    | bool   | `false`      | If `true`, overwrite the file on each run. If `false`, append.                                  |
| `independent` | bool   | `false`      | If `true`, discard all ancestor logs and write only to this file. **Not allowed at app level.** |

**Inheritance behavior:**

- `independent: false` (default): the command writes to its own `path` **plus** all ancestor `path`s.
- `independent: true`: the command writes **only** to its own `path`; ancestor logs are skipped.
- `truncate` applies **only** to the `path` declared on the same level.

```yaml
log:
  path: logs/global.log

infra:
  desc: Infrastructure
  log:
    path: logs/infra.log
    independent: true # infra commands skip global.log
  docker:
    desc: Docker ops
    log:
      path: logs/docker.log # writes to infra.log + docker.log
    up:
      script: docker-compose up -d
```

Runtime errors (missing parent dir, permission denied, path is a directory) are printed to stderr as `[log error]` but **do not fail the command**.

### 📁 Nested Groups

Organize commands in hierarchical groups:

```yaml
infra:
  desc: Infrastructure commands
  docker:
    desc: Docker operations
    up:
      script: docker-compose up
    down:
      script: docker-compose down
  k8s:
    desc: Kubernetes operations
    apply:
      script: kubectl apply -f k8s/
```

```bash
lota infra docker up
lota infra k8s apply
```

### 🎨 Help Colors

Highlight group and command names in `lota help` output using named ANSI colors or hex values:

```yaml
dev:
  desc: Development commands
  color: cyan
  frontend:
    desc: Frontend commands
    inherit_color: true
    start:
      desc: Start dev server
      inherit_color: true
      script: npm run dev
    build:
      desc: Build frontend
      color: yellow
      script: npm run build
```

| Option          | Description                                                                                                                                              |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `color`         | Named ANSI color (`black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, and `hi*` variants) or any `#RRGGBB` hex value (e.g. `#FF5733`) |
| `inherit_color` | `true` to inherit the nearest ancestor `color`. Defaults to `null` (no inheritance)                                                                      |

Color resolution priority: **direct `color` > inherited `color` > default**. `inherit_color: true` walks up the group chain and uses the first non-empty color found. Hex colors work in true-color capable terminals.

### 👻 Hiding Commands (`show`)

Set `show: false` to hide a command or group from the help output. Hidden commands remain executable but don't appear in `lota --help` or group listings:

```yaml
dev:
  desc: Development commands
  run:
    desc: Run dev server
    script: air
  internal-task:
    show: false
    script: ./scripts/internal.sh
```

```bash
lota dev run            # visible in help
lota dev internal-task  # hidden from help, but still works
```

### 🧱 Native Commands (`native`)

Commands marked with `native: true` are executed as Go functions instead of shell scripts. This is only available when embedding Lota as a Go library — the CLI has no native handlers registered.

```yaml
server:
  desc: Start HTTP server
  native: true
  args:
    - port:int=8080
    - host:str=0.0.0.0

admin:
  desc: Admin utilities
  users:
    reset-password:
      desc: Reset user password
      native: true
      args:
        - username:str
        - password:str
```

See the [Embedding Guide](docs/EMBEDDING.md) for how to register native handlers in Go.

## 🚩 Global Flags

| Flag                                   | Description                                          |
| -------------------------------------- | ---------------------------------------------------- |
| `-h`, `--help`                         | Show help                                            |
| `-V`                                   | Show version only (machine-friendly)                 |
| `--version`                            | Show version with ASCII banner                       |
| `-v`, `--verbose`                      | Enable verbose output                                |
| `--dry-run`                            | Show commands without executing                      |
| `--init`                               | Create a template lota.yml                           |
| `--config`                             | Specify config file, directory, or URL               |
| `-g`                                   | Use system-wide config (`/etc/lota.yml`)             |
| `-u`                                   | Use user config (`~/.local/share/lota.yml`)          |
| `--timeout <duration>`                 | Set a command timeout (e.g. `30s`, `5m`)             |
| `--install-completion`                 | Install shell completion script (auto-detects shell) |
| `--install-completion zsh\|bash\|fish` | Install completion for a specific shell              |
| `--completion-script zsh\|bash\|fish`  | Print completion script to stdout                    |

## 🐚 Shell Completion

Lota provides built-in shell completion for **bash**, **zsh**, and **fish**.

### Auto-install

```bash
lota --install-completion
```

Lota detects your shell from `$SHELL` and writes the completion script to the standard location.

### Install for a specific shell

```bash
lota --install-completion bash
lota --install-completion zsh
lota --install-completion fish
```

### Manual install (print to stdout)

**Bash:**

```bash
lota --completion-script bash >> ~/.bashrc
```

**Zsh:**

```bash
lota --completion-script zsh > ~/.config/zsh/completions/_lota
```

**Fish:**

```bash
lota --completion-script fish > ~/.config/fish/completions/lota.fish
```

### Troubleshooting

If `lota` behaves like a completion engine instead of executing commands:

```bash
unset COMP_LINE COMP_POINT
```

Then regenerate and reinstall the completion script for your shell.

## 🔍 Config Resolution

### Upward Config Search

If `lota.yml` is not found in the current directory, Lota searches upward through parent directories until it finds one or reaches the filesystem root (`/`). A `.git` directory is a soft boundary: when encountered, Lota still checks one level above the git root before stopping. This lets a shared `lota.yml` in a workspace folder serve nested repositories.

This is critical for monorepos and nested projects where you might run commands from subdirectories:

```bash
cd backend/src
lota build    # finds lota.yml in project root
```

### Config from URL

Load configuration directly from a remote URL:

```bash
lota --config https://example.com/lota.yml build
```

Remote configs are cached at `/tmp/lota_cache/<sha256>.yml` for subsequent runs.

### Config from Directory

Point `--config` at a directory and Lota will search for `lota.yml` or `lota.yaml` inside it:

```bash
lota --config ./my-project build
```

### System-wide and User Configs

Use `-g` for a system-wide config at `/etc/lota.yml`, or `-u` for a user-level config at `~/.local/share/lota.yml`:

```bash
lota -g build    # use /etc/lota.yml
lota -u build    # use ~/.local/share/lota.yml
```

### Command Help

Pass `--help` after a command to see its arguments:

```bash
lota dev run --help
```

### Smart Suggestions

When a command is not found, Lota suggests the closest matches using Levenshtein distance:

```bash
$ lota biuld
command not found: biuld
Did you mean:
  - build
```

Suggestions are also provided for unknown YAML fields and unknown flags during command execution.

## 👁️ Dry Run Mode

Preview what would be executed without actually running it:

```bash
lota build --dry-run
```

Dry run prints the resolved scripts with all variables and arguments interpolated, and shows the environment that would be set — without executing anything.

## �️ Terminal & PTY

On Unix systems, Lota allocates a pseudo-terminal (PTY) for command execution when output needs to be tee'd to log files. This preserves ANSI colors from child processes that check `isatty`. If PTY allocation fails, Lota falls back to normal pipes.

`SIGINT`/`SIGTERM` are propagated to child processes. Lota sends `SIGTERM` first, waits a grace period, then sends `SIGKILL` if the process hasn't exited. The `^C` echo on the terminal is disabled during execution for cleaner output.

## ��‍💻 Development

### Prerequisites

- Go 1.26+
- Python 3.8+ (for pre-commit)
- cocogitto (cog) - for conventional commits

### Setup Git Hooks

Install git hooks for commit validation and code quality:

```bash
# Install pre-commit
pip install pre-commit

# Install pre-commit hooks
pre-commit install

# Install commit-msg hook (for conventional commits)
pre-commit install --hook-type commit-msg
```

This installs:

- **pre-commit hooks** - runs go fmt, go vet, go test, and golangci-lint
- **commit-msg hook** - validates conventional commits via cocogitto

### Manual Pre-commit

Run pre-commit manually without committing:

```bash
# Run all hooks
pre-commit run --all-files

# Run specific hook
pre-commit run go-fmt --all-files
```

### Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

### Linting

```bash
# Run golangci-lint
golangci-lint run

# Run go vet
go vet ./...

# Format code
gofmt -w -s .
```

## 🏗️ Architecture

Lota follows a strict layered architecture:

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

- **config/** — YAML parsing, configuration models, imports, env file loading, URL fetching, validation
- **runner/** — Command execution, argument parsing, variable interpolation, shell resolution, PTY allocation, tee logging
- **engine/** — High-level orchestration: dependency resolution, parallel execution, native commands, help generation, shell completion
- **cli/** — CLI orchestration: flag parsing, config loading, command lookup, output formatting, suggestions
- **shared/** — Constants (app name, config file names, version info)
- **logger/** — Debug logging with verbose mode support
- **internal/** — Internal utilities (Levenshtein distance for suggestions, terminal helpers)

**Layering rules:**

- `runner` must not import `cli`
- `config` must not import `runner`
- `cli` is the top-level orchestrator that binds `config` and `engine`
- `engine` bridges `config` and `runner`

Key design principles:

- Stateless — no global variables; state (verbose, dry-run) passed through context or arguments
- Context-aware execution with graceful shutdown via `exec.CommandContext`
- Pure functions for interpolation and parsing (testable without file I/O)
- Clean error handling with wrapped errors (`fmt.Errorf("context: %w", err)`)
- Cyclic dependency detection for both variable interpolation (max depth 10) and command dependencies

## 📜 License

[Apache License 2.0](LICENSE)
