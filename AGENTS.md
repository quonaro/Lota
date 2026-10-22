# AGENTS.md

Guidance for AI agents and contributors working on **Lota** — a declarative, YAML-driven task runner written in Go.

## Project overview

- **Module:** `github.com/quonaro/lota` · **Go:** 1.26+ (see `go.mod`)
- **Binary entrypoint:** `main.go` (+ `main_unix.go` / `main_windows.go` for terminal handling)
- **Library entrypoint:** `engine/` — embeddable API (`engine.Run`, `Options`, `NativeFunc`) documented in `docs/EMBEDDING.md`
- **User docs:** `README.md`, `docs/` (CONFIGURATION, CLI, ARGUMENTS_VARS, EXECUTION, EMBEDDING) — keep them in sync when changing behavior
- **Examples:** `examples/` — update or add when introducing new config fields

## Build, test, verify

```bash
go build -o lota .          # local binary
go test -race ./...         # full test suite (CI runs this)
golangci-lint run           # lint (config: .golangci.yml)
gofmt -w -s .               # formatting (pre-commit enforced)
go vet ./...                # part of golangci-lint
```

Cross-compiled release builds (linux/windows/darwin × amd64/arm64 → `dist/` with checksums):

```bash
./scripts/build.sh          # or: lota build  (self-hosted via lota.yaml)
```

Version/commit are injected via ldflags into `shared.Version` / `shared.Commit` / `shared.BuildTime` — never hardcode them.

**Enabled linters:** `govet`, `errcheck`, `staticcheck`, `unused`, `ineffassign`, `gocritic`. Code must pass `go test -race ./...` and `golangci-lint run` before commit (both are pre-commit hooks and CI jobs).

## Architecture & layering

Strict dependency direction — do not introduce reverse imports:

```
main → cli → engine → runner → config
                  ↘ internal/*, logger, shared
```

| Package    | Role | May import |
|------------|------|------------|
| `config`   | YAML parsing, domain models (`AppConfig`, `Group`, `Command`, `Arg`, `Var`), validation, imports, `.env` | `internal/levenshtein`, `logger`, `shared` — **never** `runner`/`engine`/`cli` |
| `runner`   | Execution: interpolation, shell spawn, PTY, tee logging, hooks, signals | `config`, `internal/*`, `logger` — **never** `cli`/`engine` |
| `engine`   | Public embedding API; orchestrates config + runner | `config`, `runner`, `internal/*`, `logger` |
| `cli`      | Standalone CLI: flags, lookup, completion, suggestions, help output | everything below it |
| `shared`   | Constants + version vars only | nothing internal |
| `internal/`| `complete` (shell completion), `levenshtein`, `logger` | leaf packages |

### Invariants

- **Domain models carry data only.** `config` types hold what came from YAML. Interpolation, validation-at-run, and execution logic live in `runner`/`engine`.
- **Stateless:** no global mutable state. `Verbose`, `DryRun`, dirs, IO streams travel via `runner.RunOptions` / `engine.Options` and `context.Context`.
- **Errors:** no `panic`. Wrap with `fmt.Errorf("context: %w", err)`. `runner.ShellError` carries the child exit code — preserve it.
- **Processes:** only `exec.CommandContext`. `SIGINT`/`SIGTERM` must propagate to children with a graceful period before kill (`gracefulWait`).
- **Dry-run:** every code path in `runner` must honor `DryRun` — print the resolved command/env, execute nothing (including log file creation).
- **Hooks:** `before` fails → `script` skipped; `after` runs whenever the stage was reached; `fallback` on failure; `finally` always.
- **Variable priority:** CLI args → local vars → group vars (recursive upward) → app vars → `.env`.
- **Interpolation:** recursion depth capped at 10 — detect and report variable cycles.
- **`yaml.Node`:** always check `Kind` before reading `Value`.
- **Fail fast:** any config error aborts the run before any script executes.

### Conventions

- Preallocate slices/maps when size is predictable (`make(T, 0, n)`); avoid allocations in interpolation hot paths.
- Interfaces only when there are 2+ real implementations — no speculative abstraction.
- Platform-specific code goes in `*_unix.go` / `*_linux.go` / `*_darwin.go` / `*_windows.go` files (see `runner/`).
- Pure, testable functions: parsing/interpolation take data as arguments, never read files internally. Every package has `*_test.go` — add tests alongside changes.

## Git workflow

- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/) — enforced by `cog` (`cog.toml`, `commit-msg` hook runs `cog verify`). No bot signatures or co-author trailers.
- **Pre-commit:** `pre-commit install` enables gofmt, `go vet`, `go test -race ./...`, `golangci-lint`, plus whitespace/YAML checks.
- **Releases:** tag `v*` → `.github/workflows/release.yml` builds via `scripts/build.sh`, publishes GitHub release, syncs AUR.
- **Self-hosting:** `lota.yaml` in repo root defines `lota build`, `lota install`, `lota push` tasks — prefer them over raw commands when a task exists.
