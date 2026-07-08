# Lota

A configurable task runner for rapid development. Define commands in a YAML file and run them from the terminal.

## Features

- ✨ **Configurable tasks** - Define tasks in YAML, no code needed
- 🔧 **Flexible arguments** - Positional, flags, wildcards, arrays with type validation
- 🔄 **Variable interpolation** - Environment variables with hierarchical scoping
- 🐚 **Shell-aware execution** - Auto-detects shell binary, overridable at any level
- 👁️ **Dry-run mode** - Preview commands before execution
- 🛡️ **Graceful shutdown** - Proper process management on interrupt signals
- 📄 **Env file imports** - Load variables from .env files
- 📊 **YAML config imports** - Import nested YAML configs with dot-notation access
- 📂 **Nested groups** - Organize commands in hierarchical groups
- 📁 **Working directory** - Set `dir` per command or group (relative to `lota.yml`)
- � **Tee logging** - Write stdout/stderr to files while still printing to terminal
- � **Command dependencies** - `depends` for automatic prerequisite execution with cycle detection
- 🔍 **Upward config search** - Find `lota.yml` in parent directories up to the git root

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

## � Documentation

- **[Embedding Guide](docs/EMBEDDING.md)** - How to embed Lota in your Go application
- **[Examples](examples/)** - Complete configuration examples for various use cases

## �🚀 Quick Start

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
| Upward config search      | ✅   | ❌   | ❌   | ❌          |
| Env file imports          | ✅   | ✅   | ❌   | ❌          |
| Shell auto-detection      | ✅   | ❌   | ❌   | ❌          |

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

args: # global argument definitions
  - name:type=default

group-name: # command group
  desc: ...
  color: cyan # Optional: highlight group name in help
  inherit_color: true # Optional: inherit color from parent group
  shell: sh # Optional: override shell for this group
  vars: # group-level variables
    - KEY=value
  args: # group-level arguments
    - name:type=default
  log: # Optional: group-level tee logging
    path: group.log
  command-name:
    desc: ...
    color: green # Optional: highlight command name in help
    script: ...

command-name: # top-level command
  desc: ...
  color: red # Optional: highlight command name in help
  script: ...
  log: # Optional: command-level tee logging
    path: cmd.log
```

### 🔑 Variables (`vars`)

Variables are exported as environment variables into scripts. Both `vars` and `args` share a unified environment pool — CLI args override vars on name collision. They support three scopes with priority: **app < group < command**.

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

Access in scripts: `$app_app_name`, `$app_database_host`

### 🎯 Arguments (`args`)

Arguments are passed from the CLI and exported as environment variables, accessible via `$name` in scripts.

**Format:** `name|short:type=default`

| Part       | Description              | Example                         |
| ---------- | ------------------------ | ------------------------------- |
| `name`     | Long name                | `output`                        |
| `\|short`  | Short alias (optional)   | `\|o`                           |
| `:type`    | Type (optional)          | `:str`, `:int`, `:bool`, `:arr` |
| `=default` | Default value (optional) | `=./bin`                        |

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

> **Reserved Variables:** System environment variable names (PATH, HOME, USER, SHELL, etc.) are reserved and cannot be overridden in `vars`.

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

Priority: **command > group > cwd**. Useful in monorepos where different commands run in different subprojects.

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
**Script fails:** before → script (fail) → fallback → finally → return 1
**After fails:** before → script → after (fail) → fallback → finally → return 1

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

## 🚩 Global Flags

| Flag                                   | Description                                          |
| -------------------------------------- | ---------------------------------------------------- |
| `-h`, `--help`                         | Show help                                            |
| `-V`                                   | Show version only (machine-friendly)                 |
| `--version`                            | Show version with ASCII banner                       |
| `-v`, `--verbose`                      | Enable verbose output                                |
| `--dry-run`                            | Show commands without executing                      |
| `--init`                               | Create a template lota.yml                           |
| `--config`                             | Specify config file or directory                     |
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

## �� Upward Config Search

If `lota.yml` is not found in the current directory, Lota searches upward through parent directories until it finds one or reaches the git root (`.git` directory) or the filesystem root (`/`).

This is critical for monorepos and nested projects where you might run commands from subdirectories:

```bash
cd backend/src
lota build    # finds lota.yml in project root
```

Pass `--help` after a command to see its arguments:

```bash
lota dev run --help
```

## 👁️ Dry Run Mode

Preview what would be executed without actually running it:

```bash
lota build --dry-run
```

## 👨‍💻 Development

### Prerequisites

- Go 1.26+
- Python 3.8+ (for pre-commit)
- cocogitto (cog) - for conventional commits

### Setup Git Hooks

Install git hooks for commit validation and code quality:

```bash
# Install pre-commit
pip install pre-commit

# Install git hooks
./scripts/install-hooks.sh
```

This installs:

- **commit-msg hook** - validates conventional commits via cocogitto
- **post-commit hook** - automatic version tagging
- **pre-commit hooks** - runs go fmt, go vet, go test, and golangci-lint

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

- **config/** - YAML parsing and configuration models
- **runner/** - Command execution, argument parsing, interpolation
- **cli/** - CLI orchestration (only orchestrates, doesn't import runner)
- **shared/** - Constants and shared utilities

Key design principles:

- Stateless - no global variables
- Context-aware execution with graceful shutdown
- Pure functions for interpolation and parsing (testable)
- Clean error handling with wrapped errors

## 📜 License

[Apache License 2.0](LICENSE)
