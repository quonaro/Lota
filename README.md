# Lota

A declarative task runner for rapid development. Define commands in a YAML file and run them from the terminal.

## Features

- ✨ **Declarative configuration** - Define tasks in YAML, no code needed
- 🔧 **Flexible arguments** - Positional, flags, wildcards, arrays with type validation
- 🔄 **Variable interpolation** - Environment variables with hierarchical scoping
- 🌍 **Cross-platform** - Automatic shell detection (bash/PowerShell/cmd)
- 👁️ **Dry-run mode** - Preview commands before execution
- 🛡️ **Graceful shutdown** - Proper process management on interrupt signals
- 📄 **Env file imports** - Load variables from .env files
- 📂 **Nested groups** - Organize commands in hierarchical groups

## 📦 Installation

```bash
go install lota@latest
```

Or build from source:

```bash
go build -o lota .
```

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

## 💡 Examples

### Simple Web Project

```yaml
shell: bash

vars:
  - !import:env .env
  - NODE_ENV=development

args:
  - port|p:int=3000

dev:
  desc: Development commands
  install:
    desc: Install dependencies
    script: npm install
  start:
    desc: Start development server
    args:
      - hot|h:bool
    script: |
      if [ "{{hot}}" = "true" ]; then
        npm run dev
      else
        npm start
      fi

build:
  desc: Build for production
  before: npm run clean
  script: npm run build
  after: echo "Build completed successfully"

test:
  desc: Run tests
  args:
    - coverage|c:bool
  script: |
    if [ "{{coverage}}" = "true" ]; then
      npm test -- --coverage
    else
      npm test
    fi
```

### DevOps / Infrastructure

```yaml
shell: bash

vars:
  - DOCKER_COMPOSE=docker-compose
  - KUBECTL=kubectl

infra:
  desc: Infrastructure management
  docker:
    desc: Docker operations
    up:
      desc: Start all services
      script: $DOCKER_COMPOSE up -d
    down:
      desc: Stop all services
      script: $DOCKER_COMPOSE down
    logs:
      desc: View logs
      args:
        - service:str
        - ...tail
      script: $DOCKER_COMPOSE logs -f {{service}} {{tail}}
  k8s:
    desc: Kubernetes operations
    namespace:
      desc: Namespace operations
      create:
        desc: Create namespace
        args:
          - name:str
        script: $KUBECTL create namespace {{name}}
      delete:
        desc: Delete namespace
        args:
          - name:str
        script: $KUBECTL delete namespace {{name}}
    deploy:
      desc: Deploy application
      args:
        - env|e:str=dev
        - dry|d:bool
      script: |
        if [ "{{dry}}" = "true" ]; then
          $KUBECTL apply --dry-run=client -f k8s/{{env}}/
        else
          $KUBECTL apply -f k8s/{{env}}/
        fi
```

### Go Project

```yaml
shell: bash

vars:
  - GOOS=linux
  - GOARCH=amd64
  - BINARY_NAME=app

args:
  - target|t:str=linux/amd64

build:
  desc: Build the application
  args:
    - output|o:str=./bin
    - race|r:bool
  script: |
    if [ "{{race}}" = "true" ]; then
      go build -race -o {{output}}/{{BINARY_NAME}} .
    else
      go build -o {{output}}/{{BINARY_NAME}} .
    fi

test:
  desc: Run tests
  args:
    - verbose|v:bool
    - cover|c:bool
  script: |
    FLAGS=""
    if [ "{{verbose}}" = "true" ]; then
      FLAGS="$FLAGS -v"
    fi
    if [ "{{cover}}" = "true" ]; then
      FLAGS="$FLAGS -cover"
    fi
    go test $FLAGS ./...

release:
  desc: Build release binaries
  before: echo "Building release for {{target}}"
  script: |
    IFS=/ read -r GOOS GOARCH <<< "{{target}}"
    go build -o ./dist/${BINARY_NAME}-${GOOS}-${GOARCH} .
  after: ls -lh ./dist/
```

### Multi-Environment Project

```yaml
shell: bash

vars:
  - !import:env .env.local
  - !import:env .env.shared

args:
  - environment|env:str=dev

db:
  desc: Database operations
  migrate:
    desc: Run database migrations
    script: |
      case {{environment}} in
        dev)   npm run db:migrate:dev ;;
        staging) npm run db:migrate:staging ;;
        prod)  npm run db:migrate:prod ;;
        *)     echo "Unknown environment" ;;
      esac
  seed:
    desc: Seed database with test data
    before: echo "Seeding {{environment}} database..."
    script: npm run db:seed:{{environment}}
    after: echo "Database seeded successfully"

deploy:
  desc: Deployment operations
  staging:
    desc: Deploy to staging
    script: |
      npm run build:staging
      npm run deploy:staging
  production:
    desc: Deploy to production
    args:
      - confirm|c:bool
    script: |
      if [ "{{confirm}}" != "true" ]; then
        echo "Use --confirm to deploy to production"
        exit 1
      fi
      npm run build:prod
      npm run deploy:prod
```

## ⚙️ Configuration

### 📋 Structure

```yaml
shell: bash  # Optional: default shell (auto-detected if omitted)

vars:           # global environment variables
  - KEY=value
  - !import:env .env  # Import from .env file

args:           # global argument definitions
  - name:type=default

group-name:     # command group
  desc: ...
  shell: sh     # Optional: override shell for this group
  vars:         # group-level variables
    - KEY=value
  args:         # group-level arguments
    - name:type=default
  command-name:
    desc: ...
    script: ...

command-name:   # top-level command
  desc: ...
  script: ...
```

### 🔑 Variables (`vars`)

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

#### 📄 Import from .env files

Load variables from environment files:

```yaml
vars:
  - !import:env .env
  - !import:env config/prod.env
```

### 🎯 Arguments (`args`)

Arguments are passed from the CLI and interpolated into scripts via `{{name}}`.

**Format:** `name|short:type=default`

| Part | Description | Example |
|------|-------------|---------|
| `name` | Long name | `output` |
| `\|short` | Short alias (optional) | `\|o` |
| `:type` | Type (optional) | `:str`, `:int`, `:bool`, `:arr` |
| `=default` | Default value (optional) | `=./bin` |

#### 📝 Argument Types

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

### 🐚 Shell Configuration

Lota automatically detects the appropriate shell for your OS:
- **Linux/macOS**: bash
- **Windows**: PowerShell

Override the shell at any level:

```yaml
shell: powershell.exe  # app-level

dev:
  shell: bash          # group-level override
  run:
    shell: sh          # command-level override
    script: echo $0
```

Supported shells: bash, sh, zsh, dash, ksh, mksh, pdksh, ash, busybox, sash, tcsh, csh, fish, powershell.exe, pwsh, powershell, cmd, cmd.exe

### ⚡ Hooks (`before` / `after`)

```yaml
deploy:
  before: echo "Starting deploy..."
  script: ./deploy.sh
  after: echo "Done."
```

`after` always runs, even if `script` fails.

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

## 🚩 Global Flags

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show help |
| `-V`, `--version` | Show version |
| `-v`, `--verbose` | Enable verbose output |
| `--dry-run` | Show commands without executing |
| `--init` | Create a template lota.yml |
| `--config` | Specify config file or directory |

Pass `--help` after a command to see its arguments:

```bash
lota dev run --help
```

## 👁️ Dry Run Mode

Preview what would be executed without actually running it:

```bash
lota build --dry-run
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

## 🆚 Comparison

| Feature | Lota | Make | npm scripts | Just |
|---------|------|------|-------------|------|
| Declarative YAML | ✅ | ❌ | ❌ | ✅ |
| Cross-platform | ✅ | ❌ | ✅ | ✅ |
| Type-safe arguments | ✅ | ❌ | ❌ | ✅ |
| Variable interpolation | ✅ | ✅ | ✅ | ✅ |
| Nested groups | ✅ | ❌ | ❌ | ❌ |
| Env file imports | ✅ | ❌ | ❌ | ❌ |
| Shell auto-detection | ✅ | ❌ | ❌ | ❌ |

## 📜 License

Apache License 2.0
