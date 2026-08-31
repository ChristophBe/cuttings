# CLAUDE.md

This file is the operating contract for Claude Code (and other agents) working in this repo. It
complements [CONTRIBUTING.md](CONTRIBUTING.md) — read that for full contributor docs, coding
style, and the PR checklist. This file states what matters most for autonomous or
semi-autonomous changes, above all: **e2e coverage is mandatory for behavior changes.**

## What this project is

`cuttings` is a Cobra-based CLI that manages git worktrees as isolated development
environments ("cuttings") — see [README.md](README.md) and
[docs/features.md](docs/features.md). It is pure local git + filesystem + shell; there are no
network or GitHub API calls anywhere in the codebase.

## Non-negotiable rules for code changes

- **Dependency injection.** `cmd/` is a thin layer only — it parses arguments, calls `internal/`
  via the interfaces defined in `cmd/deps.go`, and formats output. Business logic belongs in
  `internal/`. Never bypass `cmd/deps.go`'s wiring, which happens once in `PersistentPreRunE` in
  `cmd/root.go`.
- **Error handling.** Wrap errors with `fmt.Errorf("...: %w", err)`. Return errors from `RunE`;
  never call `os.Exit` outside of `cmd/root.go`'s `Execute` and the `exitFn` seam in `cmd/run.go`.
- **E2E coverage is mandatory for behavior changes.** Any change that adds or modifies a command,
  flag, output format, exit code, or config key MUST include:
  1. Unit tests next to the changed code (`cmd/*_test.go`, `internal/**/*_test.go`), and
  2. At least one scenario in `e2e/` that exercises the compiled binary end-to-end.

  A change to `cmd/` or `internal/` behavior without a corresponding `e2e/` update is
  **incomplete** — do not report such work as done.
- **`syscall.Exec` paths.** Changes touching the interactive-shell spawn path (`new`, `shell`,
  `internal/shell.Spawn`) must be verified via e2e using the `SHELL=e2e/testdata/fakeshell.sh`
  fixture (see `e2e/repo_test.go`'s `fakeShellPath()`), not just unit tests of `BuildEnv`. A real
  shell will hang the test since `Spawn` replaces the process via `syscall.Exec`.

## Before considering work done

Run, in order, and fix any failures before reporting completion:

1. `go build ./...`
2. `make test` — unit tests
3. `make e2e` — black-box CLI tests
4. `make lint` — or `golangci-lint run ./...` directly
5. `pre-commit run --all-files`
6. Update `docs/features.md` if the CLI's user-facing surface changed.

## Repo map (pointers — see CONTRIBUTING.md's Project Layout for the full tree)

- `cmd/deps.go` — the interfaces the `cmd` layer depends on (`WorktreeManager`, `ShellSpawner`,
  `CommandRunner`); the single wiring point is `PersistentPreRunE` in `cmd/root.go`.
- `internal/worktree`, `internal/shell`, `internal/config` — business logic, each independently
  unit-tested.
- `e2e/` — black-box CLI tests, gated by the `e2e` build tag so `go test ./...` and pre-commit
  skip them by default. Run with `make e2e`. `e2e/main_test.go` builds the binary once;
  `e2e/harness_test.go` provides the hermetic subprocess-invocation helper; `e2e/repo_test.go`
  provides git repo fixtures.

## Conventions — see CONTRIBUTING.md for

Go version target, Effective Go / Google style guide adherence, Conventional Commits message
format, documentation requirements, and the full PR checklist.

## Known drift (do not "fix" opportunistically without being asked)

- `go.mod` targets Go 1.26.5; CONTRIBUTING.md's prerequisite line says "Go 1.22 or later." This
  predates the e2e work and is out of scope unless explicitly requested.

## Operating rules for the agent itself

- Never commit changes unless explicitly asked.
- Never skip pre-commit hooks (`--no-verify`) or bypass signing.
- Prefer new commits over amending; never force-push without an explicit request.
