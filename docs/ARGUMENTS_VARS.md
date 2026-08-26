# Arguments, Variables & Interpolation

Deep dive into Lota's argument parsing, variable management, and interpolation system.

---

## Table of Contents

- [Variables](#variables)
  - [Inline Variables](#inline-variables)
  - [Import from `.env` Files](#import-from-env-files)
  - [Import from YAML Files](#import-from-yaml-files)
  - [Reserved System Variables](#reserved-system-variables)
  - [Special Variables](#special-variables)
  - [Variable Scope Priority](#variable-scope-priority)
- [Arguments](#arguments)
  - [Argument Format](#argument-format)
  - [Argument Types](#argument-types)
  - [Positional Arguments](#positional-arguments)
  - [Flag Arguments](#flag-arguments)
  - [Boolean Flags](#boolean-flags)
  - [Wildcard Arguments](#wildcard-arguments)
  - [Array Arguments](#array-arguments)
  - [Optional Arguments](#optional-arguments)
  - [Argument Scopes](#argument-scopes)
  - [Flag Syntax on the CLI](#flag-syntax-on-the-cli)
- [Interpolation](#interpolation)
  - [`$name` Syntax](#name-syntax)
  - [`{{name}}` Syntax (Deprecated)](#name-syntax-deprecated)
  - [Type-Aware Validation](#type-aware-validation)
  - [Local Shell Variables](#local-shell-variables)
  - [System Environment Variables](#system-environment-variables)
  - [Cycle Detection](#cycle-detection)
  - [Missing Variable Handling](#missing-variable-handling)

---

## Variables

Variables are exported as environment variables into command scripts. Both `vars` and `args` share a unified environment pool — CLI args override vars on name collision.

### Inline Variables

```yaml
vars:
  - DOCKER=docker compose
  - BINARY_NAME=app
  - EMPTY_VAR=
```

Format: `name=value`. If no `=`, the value is empty string.

### Import from `.env` Files

```yaml
vars:
  - import:env .env
  - import:env config/prod.env
```

Loads key-value pairs from a `.env` file. The path is relative to the `lota.yml` file location.

**Example `.env` file:**

```env
DATABASE_URL=postgres://localhost:5432/mydb
API_KEY=secret123
DEBUG=true
```

The loaded variables become available as `$DATABASE_URL`, `$API_KEY`, `$DEBUG` in scripts.

### Import from YAML Files

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
- **prefix** (optional) — Add prefix to all imported keys

The YAML file is flattened to dot-notation keys:

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

```yaml
# import:yaml config.yaml@public app
# Resulting variables:
app.app_name: "MyApp"
app.version: "1.0.0"
app.database.host: "localhost"
app.database.port: "5432"
```

Access in scripts: `$app.app_name`, `$app.database.host`

**Without prefix:**

```yaml
# import:yaml config.yaml@public
# Resulting variables:
app_name: "MyApp"
version: "1.0.0"
database.host: "localhost"
database.port: "5432"
```

**Full file (no section):**

```yaml
# import:yaml config.yaml
# Resulting variables:
public.app_name: "MyApp"
public.database.host: "localhost"
private.api_key: "secret123"
```

> The old `!import:yaml` syntax is deprecated but still supported. Use `import:yaml` instead.

### Reserved System Variables

These names cannot be overridden in `vars` — attempting to do so produces an error at parse time:

| Variable     | Why Reserved                        |
| ------------ | ----------------------------------- |
| `PATH`       | System executable search path       |
| `HOME`       | User home directory                 |
| `USER`       | Current username                    |
| `SHELL`      | User's default shell                |
| `LANG`       | System locale                       |
| `LC_ALL`     | Locale override                     |
| `TERM`       | Terminal type                       |
| `PWD`        | Current working directory           |
| `OLDPWD`     | Previous working directory          |
| `HOSTNAME`   | System hostname                     |
| `LOGNAME`    | Login name                          |
| `MAIL`       | Mail file path                      |
| `TMPDIR`     | Temporary directory                 |
| `DISPLAY`    | X11 display                         |
| `XAUTHORITY` | X11 authority file                  |
| `EDITOR`     | Default editor                      |
| `VISUAL`     | Default visual editor               |
| `PAGER`      | Default pager                       |

### Special Variables

- `$CWD` — Resolves to the current working directory (where `lota` was invoked). Used in `dir` fields to run commands from the caller's directory rather than relative to `lota.yml`.

### Variable Scope Priority

Variables are resolved with the following priority (highest to lowest):

```
1. CLI args          (passed on the command line)
2. Command vars      (vars defined on the specific command)
3. Group vars        (vars defined on the containing group, recursively)
4. App vars          (vars defined at the top level)
5. Imported vars     (from .env / YAML files via import:env / import:yaml)
```

When the same variable name is defined at multiple levels, the more specific scope wins:

```yaml
vars:
  - DOCKER=docker compose    # app-level

dev:
  vars:
    - DOCKER=docker          # overrides app-level for this group
  run:
    vars:
      - DOCKER=podman        # overrides group-level for this command
    script: $DOCKER up       # resolves to "podman up"
```

---

## Arguments

Arguments are passed from the CLI and exported as environment variables, accessible via `$name` in scripts.

### Argument Format

```
name|short:type=default
```

| Part       | Description                    | Example                         |
| ---------- | ------------------------------ | ------------------------------- |
| `name`     | Long name                      | `output`                        |
| `\|short`  | Short alias (optional)         | `\|o`                           |
| `:type`    | Type (optional)                | `:str`, `:int`, `:bool`, `:arr` |
| `:type?`   | Optional marker (no default)   | `:str?` — arg not required       |
| `=default` | Default value (optional)       | `=./bin`                        |

**Parsing rules:**

1. If the string starts with `...`, it's a wildcard arg: `...rest`
2. Split by `:` to separate name part from type part
3. Split name part by `|` to get name and optional short alias
4. Split type part by `=` to get type and optional default
5. If type ends with `?`, the arg is optional (not required)
6. If a default value is provided, the arg is automatically optional
7. If type starts with `arr[`, it's an array type with optional max: `arr[5]`

### Argument Types

| Type    | Description                              | Validation                  |
| ------- | ---------------------------------------- | --------------------------- |
| `str`   | String (default if type omitted)         | None                        |
| `int`   | Integer                                  | Must parse as integer       |
| `bool`  | Boolean flag                             | Accepts various formats     |
| `arr`   | Array, collects consecutive positionals  | None                        |
| `arr[N]`| Array with max N elements                | Max N values                |

**Boolean string values accepted:** `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off`, `t`, `f`, `T`, `F`, `TRUE`, `FALSE`, `True`, `False`

### Positional Arguments

Passed by position, no flag needed:

```yaml
args:
  - filename:str
  - count:int
script: process "$filename" "$count"
```

```bash
lota cmd file.txt 5
# filename=file.txt, count=5
```

Positional args are assigned in order. If a positional arg has a default value, it's optional:

```yaml
args:
  - filename:str
  - count:int=1
script: process "$filename" "$count"
```

```bash
lota cmd file.txt        # count=1 (default)
lota cmd file.txt 10     # count=10
```

### Flag Arguments

Any arg with a short alias (`|short`) or type `bool` becomes a flag. Flags are passed by name:

```yaml
args:
  - output|o:str=./bin
  - verbose|v:bool
script: go build -o "$output"
```

```bash
lota cmd --output ./dist
lota cmd -o ./dist
lota cmd --output=./dist
lota cmd -o=./dist
lota cmd --verbose
lota cmd -v
```

### Boolean Flags

Bool args are special — they don't require a value. Presence means `true`:

```bash
lota cmd --verbose          # verbose=true
lota cmd --!verbose         # verbose=false (negation)
lota cmd --verbose=false    # verbose=false (explicit)
lota cmd --verbose=true     # verbose=true (explicit)
```

The `--!name` syntax is a Lota-specific negation shorthand.

### Wildcard Arguments

Captures all remaining positional arguments as a space-separated string:

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

The wildcard arg must be the last positional arg. It captures everything after the preceding positional args.

### Array Arguments

Collects multiple consecutive positional values into a space-separated string:

```yaml
args:
  - files:arr[5]    # collect up to 5 values
script: lint $files
```

```bash
lota cmd a.go b.go c.go
# files="a.go b.go c.go"
```

Without a max (`arr`), collects all remaining positionals:

```yaml
args:
  - files:arr
script: lint $files
```

### Optional Arguments

There are two ways to make an argument optional:

1. **With a default value:** `name:str=default` — if not provided, uses the default
2. **With `?` marker:** `name:str?` — if not provided, uses empty string

```yaml
args:
  - output|o:str=./bin    # optional with default
  - tag|t:str?            # optional, empty if not provided
  - force|f:bool          # required bool flag (must be passed)
```

> Without `?` or a default, typed args are **required**. If not provided, Lota returns an error.

### Argument Scopes

Args can be defined at app, group, or command level. They are merged with priority: **command > group > app**.

```yaml
args:
  - env:str=dev         # available to all commands

deploy:
  args:
    - env:str=prod      # overrides app-level for this group
  run:
    args:
      - force|f:bool    # command-specific arg
    script: ./deploy.sh --env="$env" $force
```

When the same arg name is defined at multiple levels, the more specific scope wins.

### Flag Syntax on the CLI

| Syntax             | Meaning                              |
| ------------------ | ------------------------------------ |
| `--flag value`     | Long flag, space-separated value     |
| `--flag=value`     | Long flag, equals-separated value    |
| `-f value`         | Short flag, space-separated value    |
| `-f=value`         | Short flag, equals-separated value   |
| `--flag`           | Boolean flag (sets to `true`)        |
| `--!flag`          | Boolean negation (sets to `false`)   |
| `--flag=false`     | Boolean explicit false               |
| `--`               | End of flags (rest are positional)   |

---

## Interpolation

Lota interpolates variables and arguments into script strings before execution.

### `$name` Syntax

The primary interpolation syntax. Replaces `$name` with the variable or argument value:

```yaml
vars:
  - APP_NAME=myapp
  - PORT=3000
args:
  - env|e:str=dev
script: echo "Starting $APP_NAME on port $PORT in $env mode"
```

```bash
lota cmd --env prod
# Output: Starting myapp on port 3000 in prod mode
```

**Works in all script stages:** `before`, `script`, `after`, `fallback`, `finally`.

**Works in `dir` and `log.path` fields:**

```yaml
build:
  dir: $CWD/build
  log:
    path: logs/$env-build.log
  script: make build
```

### `{{name}}` Syntax (Deprecated)

The double-brace syntax `{{name}}` is still supported but deprecated:

```yaml
script: echo "Building {{APP_NAME}}"  # deprecated, use $APP_NAME
```

A deprecation warning is printed to stderr when `{{}}` is used for variable interpolation. This syntax will be removed in a future version.

### Type-Aware Validation

When interpolating typed arguments, Lota validates the value:

- **`int`**: Value must be a valid integer. Error if not.
- **`bool`**: Value is normalized to `true` or `false`.
- **`arr`**: Value is space-joined array elements.
- **`str`**: No validation (any string is accepted).

If a required argument is missing and has no default, interpolation produces an error.

### Local Shell Variables

Lota detects local shell variables in scripts (e.g., `FOO=bar` assignments) and does **not** treat them as missing Lota variables:

```yaml
script: |
  FILES=$(ls *.go)
  echo "Found files: $FILES"
```

Here `$FILES` is a local shell variable, not a Lota variable. Lota recognizes it and skips interpolation for it.

### System Environment Variables

Variables that exist in the system environment (e.g., `$HOME`, `$PATH`) are not interpolated by Lota — they are passed through to the shell as-is. This ensures system variables work correctly in scripts.

### Cycle Detection

Lota detects cyclic variable references during interpolation. If variable A references variable B, which references variable A, an error is produced.

**Maximum interpolation depth: 10 levels.** If the depth is exceeded, Lota returns an error:

```
error: maximum interpolation depth (10) exceeded for variable "A"
```

### Missing Variable Handling

If a `$name` reference cannot be resolved (not a Lota variable, not a CLI arg, not a local shell variable, not a system env var), Lota returns an error:

```
error: undefined variable "MISSING_VAR" in script
```

Lota also provides "Did you mean?" suggestions for variables with similar names using Levenshtein distance.
