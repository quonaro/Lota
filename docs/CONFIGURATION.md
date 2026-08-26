# Configuration Reference

Complete reference for every field in `lota.yml` / `lota.yaml`. All fields are optional unless marked **required**.

---

## Table of Contents

- [App-Level Fields](#app-level-fields)
- [Group Fields](#group-fields)
- [Command Fields](#command-fields)
- [Variables (`vars`)](#variables-vars)
- [Arguments (`args`)](#arguments-args)
- [Shell (`shell`)](#shell-shell)
- [Working Directory (`dir`)](#working-directory-dir)
- [Tee Logging (`log`)](#tee-logging-log)
- [Config Imports (`imports`)](#config-imports-imports)
- [Dependencies (`depends`)](#dependencies-depends)
- [Execution Hooks](#execution-hooks)
- [Colors (`color`, `inherit_color`)](#colors-color-inherit_color)
- [Visibility (`show`)](#visibility-show)
- [Native Commands (`native`)](#native-commands-native)
- [Parallel Execution (`parallel`)](#parallel-execution-parallel)

---

## App-Level Fields

Top-level keys in the YAML file. Everything except recognized fields is treated as a command or group.

| Field      | Type        | Default     | Description                                      |
| ---------- | ----------- | ----------- | ------------------------------------------------ |
| `shell`    | string      | auto-detect | Default shell for all commands                   |
| `vars`     | list\[Var\] | `[]`        | Global variables exported to all commands        |
| `args`     | list\[str\] | `[]`        | Global argument definitions                      |
| `log`      | LogConfig   | —           | Global tee logging for all commands              |
| `imports`  | list\[ImportConfig\] | `[]` | Import other config files or URLs         |

**Example:**

```yaml
shell: bash
vars:
  - APP_NAME=myapp
  - import:env .env
args:
  - env|e:str=dev
log:
  path: logs/all.log
imports:
  - url: ./extra.yaml
    namespace: ext
```

> The `log` field at app level does **not** support `independent: true`.

---

## Group Fields

Groups organize commands hierarchically. They can be nested to arbitrary depth.

| Field           | Type        | Default | Description                                           |
| --------------- | ----------- | ------- | ----------------------------------------------------- |
| `desc`          | string      | —       | Description shown in help output                      |
| `dir`           | string      | —       | Working directory for all commands in this group      |
| `color`         | string      | —       | ANSI color name or `#RRGGBB` for help output          |
| `inherit_color` | bool        | `null`  | Inherit color from nearest ancestor with `color` set  |
| `show`          | bool        | `null`  | `false` to hide this group from help output           |
| `shell`         | string      | —       | Override shell for all commands in this group         |
| `vars`          | list\[Var\] | `[]`    | Group-level variables                                 |
| `args`          | list\[str\] | `[]`    | Group-level argument definitions                      |
| `log`           | LogConfig   | —       | Group-level tee logging                               |
| `<command>`     | Command     | —       | A command inside this group (has `script` or `native`)|
| `<sub-group>`   | Group       | —       | A nested group (mapping without `script`/`native`)    |

**How Lota distinguishes commands from groups:** if a mapping node contains a `script` or `native` field, it is parsed as a Command. Otherwise, it is parsed as a Group.

**Example:**

```yaml
infra:
  desc: Infrastructure management
  color: cyan
  inherit_color: true
  shell: bash
  dir: ./infra
  vars:
    - DOCKER=docker compose
  args:
    - env|e:str=dev
  log:
    path: logs/infra.log
  docker:
    desc: Docker operations
    up:
      desc: Start services
      script: $DOCKER up -d
    down:
      desc: Stop services
      script: $DOCKER down
```

---

## Command Fields

Commands are the executable units. A mapping is recognized as a command if it contains `script` or `native: true`.

| Field           | Type        | Default     | Description                                              |
| --------------- | ----------- | ----------- | -------------------------------------------------------- |
| `desc`          | string      | —           | Description shown in help output                         |
| `dir`           | string      | —           | Working directory for this command                       |
| `color`         | string      | —           | ANSI color name or `#RRGGBB` for help output             |
| `inherit_color` | bool        | `null`      | Inherit color from nearest ancestor with `color` set     |
| `show`          | bool        | `null`      | `false` to hide from help output                         |
| `shell`         | string      | —           | Override shell for this command                          |
| `vars`          | list\[Var\] | `[]`        | Command-level variables                                  |
| `args`          | list\[str\] | `[]`        | Command-level argument definitions                       |
| `script`        | string      | —           | Main script to execute                                   |
| `before`        | string      | —           | Pre-execution hook                                       |
| `after`         | string      | —           | Post-success hook                                        |
| `fallback`      | string      | —           | Error recovery hook                                      |
| `finally`       | string      | —           | Always-runs cleanup hook                                 |
| `depends`       | list\[str\] | `[]`        | Commands to run before this one (dot-separated paths)    |
| `parallel`      | bool        | `true`      | Run dependencies in parallel (`false` for sequential)    |
| `native`        | bool        | `false`     | Mark as native Go command (embedded mode only)           |
| `log`           | LogConfig   | —           | Command-level tee logging                                |

**Example:**

```yaml
deploy:
  desc: Deploy to production
  color: hired
  shell: bash
  dir: ./deploy
  vars:
    - REGION=us-east-1
  args:
    - env|e:str=prod
    - force|f:bool
  before: echo "Starting deploy to $REGION..."
  script: ./deploy.sh --env=$env --region=$REGION
  after: echo "Deploy complete"
  fallback: ./rollback.sh
  finally: rm -f /tmp/deploy.lock
  depends:
    - build
    - test
  parallel: false
  log:
    path: logs/deploy.log
    truncate: true
```

---

## Variables (`vars`)

Variables are exported as environment variables into command scripts. They support three scopes with priority: **app < group < command**.

### Inline Variables

```yaml
vars:
  - KEY=value
  - EMPTY_VAR=
```

Format: `name=value`. If no `=`, the value is empty string.

### Import from `.env` files

```yaml
vars:
  - import:env .env
  - import:env config/prod.env
```

Loads key-value pairs from a `.env` file. Path is relative to the `lota.yml` file.

### Import from YAML files

```yaml
vars:
  - import:yaml config.yaml
  - import:yaml config.yaml app
  - import:yaml config.yaml@public
  - import:yaml secrets.yaml@db cfg
```

**Syntax:** `import:yaml <file>[@<section>] [<prefix>]`

- **file** — Path to YAML file (relative to `lota.yml`)
- **section** (optional) — Import only a specific top-level section via `@section`
- **prefix** (optional) — Add prefix to all imported keys (e.g. `app` produces `app.key`)

The YAML file is flattened to dot-notation keys:

```yaml
# config.yaml
public:
  app_name: MyApp
  database:
    host: localhost
    port: 5432
```

```yaml
# import:yaml config.yaml@public app
# Resulting variables:
app.app_name: "MyApp"
app.database.host: "localhost"
app.database.port: "5432"
```

Access in scripts: `$app.app_name`, `$app.database.host`

> The old `!import:yaml` syntax is deprecated but still supported. Use `import:yaml` instead.

### Reserved System Variables

These names cannot be overridden in `vars`:

`PATH`, `HOME`, `USER`, `SHELL`, `LANG`, `LC_ALL`, `TERM`, `PWD`, `OLDPWD`, `HOSTNAME`, `LOGNAME`, `MAIL`, `TMPDIR`, `DISPLAY`, `XAUTHORITY`, `EDITOR`, `VISUAL`, `PAGER`

### Special Variables

- `$CWD` — Resolves to the current working directory (where `lota` was invoked). Used in `dir` fields.

### Variable Scope Priority

```
CLI args > command vars > group vars > app vars > .env / imported vars
```

When the same variable name is defined at multiple levels, the more specific scope wins.

---

## Arguments (`args`)

Arguments are passed from the CLI and exported as environment variables, accessible via `$name` in scripts.

**Format:** `name|short:type=default`

| Part       | Description                    | Example                         |
| ---------- | ------------------------------ | ------------------------------- |
| `name`     | Long name                      | `output`                        |
| `\|short`  | Short alias (optional)         | `\|o`                           |
| `:type`    | Type (optional)                | `:str`, `:int`, `:bool`, `:arr` |
| `:type?`   | Optional marker (no default)   | `:str?` — arg not required       |
| `=default` | Default value (optional)       | `=./bin`                        |

> A default value (`=default`) automatically makes an argument optional. The `?` marker is useful when you want an argument optional **without** a default (empty string if not provided).

### Argument Types

| Type   | Description                          | Example             |
| ------ | ------------------------------------ | ------------------- |
| `str`  | String (default if type omitted)     | `name:str`          |
| `int`  | Integer, validated                   | `count:int`         |
| `bool` | Boolean flag                         | `verbose:bool`      |
| `arr`  | Array, collects consecutive positionals | `files:arr`      |
| `arr[N]` | Array with max N elements          | `files:arr[5]`      |

### Positional Arguments

```yaml
args:
  - filename:str
  - count:int
script: process "$filename" "$count"
```

```bash
lota cmd file.txt 5
```

### Flag Arguments

Any arg with a short alias (`|short`) or type `bool` becomes a flag:

```yaml
args:
  - output|o:str=./bin
  - verbose|v:bool
```

```bash
lota cmd --output ./dist
lota cmd -o ./dist --verbose
lota cmd --output=./dist
```

### Wildcard Arguments

Captures all remaining positional arguments:

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

### Boolean Flags

```bash
lota cmd --verbose          # verbose=true
lota cmd --!verbose         # verbose=false
lota cmd --verbose=false    # verbose=false
```

### `--` Separator

Use `--` to signal end of flags — everything after is treated as positional:

```bash
lota cmd -- --flag-like-value
```

### Argument Scopes

Args can be defined at app, group, or command level. Merged with priority: **command > group > app**.

```yaml
args:
  - env:str=dev

deploy:
  args:
    - env:str=prod  # overrides app-level
  run:
    script: ./deploy.sh --env="$env"
```

---

## Shell (`shell`)

Lota auto-detects the shell binary from the system environment (`$SHELL`). If detection fails, it falls back to `bash`.

Override at any level:

```yaml
shell: zsh       # app-level

dev:
  shell: bash    # group-level
  run:
    shell: sh    # command-level
    script: echo $0
```

**Supported shells:** bash, sh, zsh, dash, ksh, mksh, pdksh, ash, busybox, sash, tcsh, csh, fish

Shell-specific flags are automatically added:
- `bash`, `zsh`, `sh`, `dash`, `ksh`, `ash`, `busybox` → `-c`
- `fish` → `-c`
- `tcsh`, `csh` → `-c`

> The script content is **shell-specific**. Write scripts for the shell you target.

---

## Working Directory (`dir`)

Set the working directory for commands and groups. Paths are resolved relative to the `lota.yml` file location.

```yaml
backend:
  dir: ./backend      # group-level default
  build:
    script: go build .
  test:
    dir: ./backend/tests  # command-level override
    script: go test ./...
```

**Priority:** command > group > config file directory

Use `$CWD` to run from the current working directory (where `lota` was invoked):

```yaml
build:
  dir: $CWD
  script: make build
```

Or combine:

```yaml
build:
  dir: $CWD/subdir
  script: make build
```

---

## Tee Logging (`log`)

Mirror command output to log files while still printing to the terminal.

| Field         | Type   | Default      | Description                                                                                     |
| ------------- | ------ | ------------ | ----------------------------------------------------------------------------------------------- |
| `path`        | string | **required** | Log file path (relative to `lota.yml`). Supports variable interpolation (`$var`).               |
| `truncate`    | bool   | `false`      | If `true`, overwrite the file on each run. If `false`, append.                                  |
| `independent` | bool   | `false`      | If `true`, discard all ancestor logs and write only to this file. **Not allowed at app level.** |

### Inheritance Behavior

Logs support **additive inheritance**: a command writes to its own log file **plus** all ancestor log files.

- `independent: false` (default): writes to own `path` **plus** all ancestor `path`s
- `independent: true`: writes **only** to own `path`; ancestor logs are skipped
- `truncate` applies **only** to the `path` declared on the same level

```yaml
log:
  path: logs/global.log

infra:
  log:
    path: logs/infra.log
    independent: true    # infra commands skip global.log
  docker:
    log:
      path: logs/docker.log  # writes to infra.log + docker.log
    up:
      script: docker-compose up -d
```

Runtime errors (missing parent dir, permission denied, path is a directory) are printed to stderr as `[log error]` but **do not fail the command**.

---

## Config Imports (`imports`)

Import other Lota configuration files and merge them into the current config.

| Field       | Type   | Default     | Description                                                         |
| ----------- | ------ | ----------- | ------------------------------------------------------------------- |
| `url`       | string | **required** | Path to local file (relative to `lota.yml`) or HTTP/HTTPS URL     |
| `namespace` | string | —           | Wraps imported commands/groups under a new top-level group          |

```yaml
imports:
  - url: ./extra.yaml
  - url: https://example.com/shared.yaml
    namespace: shared
```

### Merge Rules

- **Without namespace:** Imported commands and groups merge into root. Local definitions override imported ones with the same name.
- **With namespace:** Imported commands and groups are wrapped under a new top-level group named after the namespace.
- **Variables:** Always merged into root (no namespace wrapping). Local vars override imported vars.
- **Nested imports:** Imported configs cannot have their own `imports` field (silently ignored).

### URL Fetching

- Configs downloaded with a 100MB size limit
- Cached at `/tmp/lota_cache/<sha256>.yml` for subsequent runs
- TLS verification is skipped (use with trusted sources only)

---

## Dependencies (`depends`)

Reference other commands that must run before the current one. Dependencies are specified as full dot-separated paths.

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

### Behavior

- Dependencies execute with **their own context** (shell, vars, dir, default args)
- **Shared dependencies** are executed **once** (deduplication)
- **Circular dependencies** are detected automatically and produce an error
- Independent dependencies run **in parallel by default**

### Parallel vs Sequential

```yaml
ci:
  depends:
    - lint
    - test
  parallel: false    # force sequential execution
  script: echo "CI done"
```

When `parallel: true` (default), dependencies at the same level run concurrently with colored output prefixes. When `false`, dependencies run sequentially with `=> Running dependency: <path>` messages.

---

## Execution Hooks

Five stages per command. You only use what you need — a simple `script` is enough for most tasks.

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

**Return code:** `0` if `before`+`script`+`after` succeeded, **or if `fallback` succeeded after a failure**. Otherwise the first error's exit code. `finally` errors are printed to stderr but do not change the return code.

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

## Colors (`color`, `inherit_color`)

Highlight group and command names in help output.

### Named ANSI Colors

`black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`

High-intensity variants: `hiblack`, `hired`, `higreen`, `hiyellow`, `hiblue`, `himagenta`, `hicyan`, `hiwhite`

### Hex Colors

Any `#RRGGBB` hex value (e.g. `#FF5733`). Works in true-color capable terminals.

### `inherit_color`

| Value  | Behavior                                                        |
| ------ | --------------------------------------------------------------- |
| `null` | No inheritance (default)                                        |
| `true` | Walk up the group chain, use first non-empty `color` found      |

**Resolution priority:** direct `color` > inherited `color` > default

```yaml
dev:
  color: cyan
  frontend:
    inherit_color: true    # inherits cyan
    start:
      inherit_color: true  # inherits cyan
      script: npm run dev
    build:
      color: yellow        # overrides, no inheritance
      script: npm run build
```

---

## Visibility (`show`)

| Value  | Behavior                                              |
| ------ | ----------------------------------------------------- |
| `null` | Visible in help (default)                             |
| `true` | Visible in help                                       |
| `false`| Hidden from help, but still executable               |

```yaml
dev:
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

---

## Native Commands (`native`)

Commands marked with `native: true` are executed as Go functions instead of shell scripts. Only available when embedding Lota as a Go library — the CLI has no native handlers registered.

```yaml
server:
  desc: Start HTTP server
  native: true
  args:
    - port:int=8080
    - host:str=0.0.0.0

admin:
  users:
    reset-password:
      desc: Reset user password
      native: true
      args:
        - username:str
        - password:str
```

See the [Embedding Guide](EMBEDDING.md) for how to register native handlers in Go.

---

## Parallel Execution (`parallel`)

Controls whether dependencies run in parallel or sequentially.

| Value   | Behavior                                                        |
| ------- | --------------------------------------------------------------- |
| `null`  | Parallel (default — same as `true`)                             |
| `true`  | Dependencies at the same level run concurrently                 |
| `false` | Dependencies run sequentially, one after another                |

When parallel, each task gets its own color for prefixed output: either from its `color` field, inherited from a parent group, or a deterministic color derived from the task name.

```bash
[build]  go build -o bin/app .
[lint]   golangci-lint run
[build]  done
[lint]   done
[deploy] ./deploy.sh
```
