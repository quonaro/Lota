# CLI Usage

Complete reference for the Lota command-line interface.

---

## Table of Contents

- [Global Flags](#global-flags)
- [Config Resolution](#config-resolution)
- [Dry Run Mode](#dry-run-mode)
- [Command Timeout](#command-timeout)
- [Initialization (`--init`)](#initialization---init)
- [Shell Completion](#shell-completion)
- [Command Help](#command-help)
- [Smart Suggestions](#smart-suggestions)
- [Flag Syntax](#flag-syntax)
- [Exit Codes](#exit-codes)

---

## Global Flags

| Flag                                   | Description                                          |
| -------------------------------------- | ---------------------------------------------------- |
| `-h`, `--help`                         | Show help                                            |
| `-V`                                   | Show version only (machine-friendly)                 |
| `--version`                            | Show version with ASCII banner                       |
| `-v`, `--verbose`                      | Enable verbose (debug) output                        |
| `--dry-run`                            | Show commands without executing                      |
| `--init`                               | Create a template `lota.yml` in the current directory |
| `--config <path>`                      | Specify config file, directory, or URL               |
| `-g`                                   | Use system-wide config (`/etc/lota.yml`)             |
| `-u`                                   | Use user config (`~/.local/share/lota.yml`)          |
| `--timeout <duration>`                 | Set a command timeout (e.g. `30s`, `5m`)             |
| `--install-completion`                 | Install shell completion script (auto-detects shell) |
| `--install-completion zsh\|bash\|fish` | Install completion for a specific shell              |
| `--completion-script zsh\|bash\|fish`  | Print completion script to stdout                    |

### Usage Examples

```bash
# Show help
lota --help
lota -h

# Show version
lota -V                   # machine-friendly: "0.1.0"
lota --version            # with ASCII banner

# Verbose output (debug logging)
lota -v build
lota --verbose build

# Dry run
lota build --dry-run

# Specify config
lota --config ./my-config.yaml build
lota --config ./my-project build
lota --config https://example.com/lota.yml build

# System-wide / user config
lota -g build
lota -u build

# Timeout
lota --timeout 5m build
lota --timeout 30s test

# Initialize
lota --init
```

---

## Config Resolution

Lota resolves the configuration file in the following order:

### 1. Explicit `--config`

If `--config` is provided, Lota uses that path. The path can be:

- **File path:** `lota --config ./my-config.yaml build`
- **Directory:** `lota --config ./my-project build` (searches for `lota.yml` or `lota.yaml` inside)
- **URL:** `lota --config https://example.com/lota.yml build` (downloaded and cached)

### 2. System-wide (`-g`)

Uses `/etc/lota.yml` if it exists.

### 3. User-level (`-u`)

Uses `~/.local/share/lota.yml` if it exists.

### 4. Upward Search (default)

If no flag is provided, Lota searches for `lota.yml` or `lota.yaml`:

1. In the current directory
2. In each parent directory, walking upward
3. Stops at the first match, or at the git root (`.git` directory), or at the filesystem root (`/`)

This is critical for monorepos:

```bash
cd backend/src
lota build    # finds lota.yml in project root
```

### URL Caching

Remote configs fetched via `--config <url>` or `imports` with URLs are cached at `/tmp/lota_cache/<sha256>.yml`. The cache persists across runs. To force a re-download, remove the cache file.

---

## Dry Run Mode

Preview what would be executed without actually running it:

```bash
lota build --dry-run
```

Dry run output includes:
- The resolved scripts with all variables and arguments interpolated
- The environment variables that would be set
- The shell that would be used
- The working directory

No commands are executed, no files are created, no log files are written.

---

## Command Timeout

Set a per-run timeout with `--timeout`:

```bash
lota --timeout 30s build      # 30 seconds
lota --timeout 5m test        # 5 minutes
lota --timeout 1h deploy      # 1 hour
```

Supported duration formats: `Ns` (seconds), `Nm` (minutes), `Nh` (hours).

When the timeout expires, Lota sends `SIGTERM` to the running process. If the process doesn't exit within a grace period, `SIGKILL` is sent.

---

## Initialization (`--init`)

Create a template `lota.yml` in the current directory:

```bash
lota --init
```

This creates a `lota.yml` with example commands and comments to help you get started. If `lota.yml` already exists, it will not be overwritten.

---

## Shell Completion

Lota provides built-in shell completion for **bash**, **zsh**, and **fish**.

### Auto-install

```bash
lota --install-completion
```

Lota detects your shell from `$SHELL` and writes the completion script to the standard location:

| Shell | Install Path                                                           |
| ----- | ---------------------------------------------------------------------- |
| bash  | `~/.local/share/bash-completion/completions/lota`                      |
| zsh   | `~/.config/zsh/completions/_lota`                                      |
| fish  | `~/.config/fish/completions/lota.fish`                                 |

For zsh, Lota also updates `~/.zshrc` to add the completion directory to `fpath` (before `compinit` if found). After installation, reload your shell: `exec zsh`.

### Install for a Specific Shell

```bash
lota --install-completion bash
lota --install-completion zsh
lota --install-completion fish
```

### Manual Install (Print to stdout)

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

### What Gets Completed

- Command and group names (including nested)
- Sub-commands within groups
- Global flags (`--help`, `--verbose`, `--dry-run`, etc.)
- Command-specific flags (based on `args` definitions)

### Troubleshooting

If `lota` behaves like a completion engine instead of executing commands:

```bash
unset COMP_LINE COMP_POINT
```

Then regenerate and reinstall the completion script for your shell.

---

## Command Help

### Top-level Help

```bash
lota --help
```

Shows all available top-level commands and groups.

### Group Help

```bash
lota dev --help
lota infra --help
```

Shows all commands and sub-groups within the specified group.

### Command Help

```bash
lota dev run --help
lota build --help
```

Shows the command's description, arguments, options, and (in verbose mode) the script content.

---

## Smart Suggestions

When a command is not found, Lota suggests the closest matches using Levenshtein distance:

```bash
$ lota biuld
command not found: biuld
Did you mean:
  - build
```

Suggestions are also provided for:
- **Unknown YAML fields** — "Did you mean: `script`?" when you typo a field name
- **Unknown flags** — "Did you mean: `--verbose`?" when you pass an unrecognized flag

The suggestion engine uses normalized Levenshtein distance with a 60% similarity threshold. Up to 3 suggestions are shown.

---

## Flag Syntax

### Global Flags

Global flags must appear before the command name:

```bash
lota --verbose build        # correct
lota build --verbose        # --verbose is passed to the command, not global
```

### Command Flags

Command-specific flags (defined via `args`) appear after the command name:

```bash
lota build --output ./dist --verbose
lota build -o ./dist -v
```

### Equals Syntax

Flags accept `--name=value` or `--name value`:

```bash
lota cmd --output=./dist
lota cmd --output ./dist
lota cmd -o=./dist
lota cmd -o ./dist
```

### `--` Separator

Use `--` to signal end of flags. Everything after `--` is treated as positional arguments:

```bash
lota cmd -- --flag-like-value
# '--flag-like-value' is passed as a positional arg, not parsed as a flag
```

### Boolean Negation

Boolean flags support `--!name` for negation:

```bash
lota cmd --verbose        # verbose=true
lota cmd --!verbose       # verbose=false
lota cmd --verbose=false  # verbose=false
```

---

## Exit Codes

| Code | Meaning                                              |
| ---- | ---------------------------------------------------- |
| `0`  | Success                                              |
| `1`  | Command execution failed (non-zero exit from script) |
| `2`  | Configuration error (parse error, validation error)  |
| `130`| Interrupted by user (SIGINT / Ctrl+C)                |

The exit code from a failed shell command is propagated as the Lota exit code. If `fallback` succeeds after a failure, the exit code is `0`.

`SIGINT` (Ctrl+C) triggers graceful shutdown: Lota sends `SIGTERM` to child processes, waits a grace period, then sends `SIGKILL` if needed. The `^C` echo on the terminal is disabled during execution for cleaner output.
