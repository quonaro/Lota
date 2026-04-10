# Lota

A declarative task runner for rapid development. Define commands in a YAML file and run them from the terminal.

## Installation

```bash
go install lota@latest
```

Or build from source:

```bash
go build -o lota .
```

## Quick Start

Create a `lota.yml` in your project root:

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

## Configuration

### Structure

```yaml
vars:           # global environment variables
  - KEY=value

args:           # global argument definitions
  - name:type=default

group-name:     # command group
  desc: ...
  command-name:
    desc: ...
    script: ...

command-name:   # top-level command
  desc: ...
  script: ...
```

### Variables (`vars`)

Variables are injected as environment variables into scripts. They support three scopes with priority: **app < group < command**.

```yaml
vars:
  - DOCKER=docker compose   # app-level

dev:
  vars:
    - DOCKER=docker          # overrides app-level for this group
  run:
    vars:
      - DOCKER=podman        # overrides group-level for this command
    script: $DOCKER up
```

### Arguments (`args`)

Arguments are passed from the CLI and interpolated into scripts via `{{name}}`.

**Format:** `name|short:type=default`

| Part | Description | Example |
|------|-------------|---------|
| `name` | Long name | `output` |
| `\|short` | Short alias (optional) | `\|o` |
| `:type` | Type (optional) | `:str`, `:int`, `:bool`, `:arr` |
| `=default` | Default value (optional) | `=./bin` |

#### Argument Types

**Positional** — passed by position, no flag needed:

```yaml
args:
  - filename:str
  - count:int
script: process {{filename}} {{count}}
```
```bash
lota cmd file.txt 5
```

**Flag** — passed by name using `--flag` or `-f`. Any arg with a short alias (`|short`) or type `bool` becomes a flag:

```yaml
args:
  - output|o:str=./bin
  - verbose|v:bool
script: go build -o {{output}}
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
script: docker exec {{service}} {{cmd}}
```
```bash
lota cmd backend python manage.py shell
# service=backend, cmd="python manage.py shell"
```

**Array** — collects multiple consecutive positional values:

```yaml
args:
  - files:arr[5]   # collect up to 5 values
script: lint {{files}}
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
  - env:str=dev         # available to all commands

deploy:
  args:
    - env:str=prod      # overrides app-level for this group
  run:
    script: ./deploy.sh --env={{env}}
```

### Hooks (`before` / `after`)

```yaml
deploy:
  before: echo "Starting deploy..."
  script: ./deploy.sh
  after: echo "Done."
```

`after` always runs, even if `script` fails.

### Global Flags

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show help |
| `-V`, `--version` | Show version |
| `-v`, `--verbose` | Enable verbose output |

Pass `--help` after a command to see its arguments:

```bash
lota dev run --help
```

## License

MIT
