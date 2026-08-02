# Contributing

Thank you for contributing to `workstreams`. This document covers the development workflow, coding conventions, and expectations for pull requests.

---

## Development Setup

**Prerequisites:**
- Go 1.22 or later — [golang.org/doc/install](https://golang.org/doc/install)
- [pre-commit](https://pre-commit.com/index.html#installation) — see installation instructions for your platform
- [golangci-lint](https://golangci-lint.run/welcome/install/) — see installation instructions for your platform

**Setup:**

```bash
git clone https://github.com/ChristophBe/workstreams.git
cd workstreams
pre-commit install          # install git hooks
go build ./...              # verify the build
go test ./...               # run tests
```

---

## Project Layout

```
workstreams/
├── cmd/                   # Cobra command definitions (thin layer only)
│   └── deps.go            # WorktreeManager and ShellSpawner interface definitions
├── internal/
│   ├── config/            # Configuration loading (returns *Config struct)
│   ├── worktree/          # Git worktree operations (*Manager struct)
│   └── shell/             # Shell spawning (*Spawner struct)
├── docs/                  # Feature and design documentation
├── .golangci.yml          # Linter configuration
├── .pre-commit-config.yaml
├── Makefile
├── CONTRIBUTING.md        # This file
└── README.md
```

**Rules:**
- Business logic lives in `internal/`. The `cmd/` layer only parses arguments, calls internal methods via interfaces, and formats output.
- No package-level `init()` side-effects beyond registering Cobra commands.
- No global mutable state outside of Cobra command variables and the `deps` struct in `cmd/deps.go`.

---

## Coding Guidelines

### Go version and style

- Target the Go version in `go.mod`.
- Follow [Effective Go](https://go.dev/doc/effective_go) and the [Google Go Style Guide](https://google.github.io/styleguide/go/).
- Run `gofmt` and `goimports` before committing (the pre-commit hook does this automatically).

### Error handling

- Wrap errors with context using `fmt.Errorf("...: %w", err)`.
- Return errors from `RunE` in Cobra commands; never call `os.Exit` directly from business logic.
- Use `errors.Is` / `errors.As` for error comparison; define sentinel errors in the package that owns the type.

### Testing

- Unit tests live next to the code they test (`*_test.go` in the same package).
- Use `t.TempDir()` for temporary directories — they are automatically cleaned up.
- Use `t.Setenv()` instead of `os.Setenv` so the test framework restores the environment after each test.
- Integration tests that invoke git commands must clear git hook environment variables (`GIT_DIR`, `GIT_INDEX_FILE`, `GIT_WORK_TREE`) to avoid interference when running from within a git hook.
- Aim for meaningful tests over high line coverage. Test behaviour, not implementation.

### Dependency injection

The codebase follows Go's "accept interfaces, return structs" principle:

- **Interfaces are defined by the consumer.** `cmd/deps.go` defines `WorktreeManager` and `ShellSpawner` — the two interfaces the `cmd` layer needs. Internal packages know nothing about these interfaces.
- **Internal packages return concrete structs.** `config.Load` returns `*config.Config`, `worktree.NewManager` returns `*worktree.Manager`, `shell.NewSpawner` returns `*shell.Spawner`. These types satisfy the interfaces in `cmd/deps.go` without importing the `cmd` package.
- **A single wiring point.** `PersistentPreRunE` in `cmd/root.go` constructs all concrete types and stores them in the package-level `deps` struct. All subcommands read from `deps` — they never import internal packages for their implementations.
- **Testing cmd commands.** Because subcommands depend on interfaces, you can write `cmd` tests using lightweight fake implementations:

  ```go
  type fakeWorktreeManager struct{ exists bool }
  func (f *fakeWorktreeManager) Exists(branch string) bool { return f.exists }
  // … implement other interface methods
  ```

### Documentation

- Every exported function, type, and variable must have a doc comment.
- Package doc comments go in one file per package (typically the file that defines the primary type or the entry point).
- Internal packages (under `internal/`) should still be documented: they may be read by contributors.

### Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short summary>

[optional body]

Co-Authored-By: ...
```

Types: `feat`, `fix`, `test`, `docs`, `chore`, `refactor`.

- Keep the summary line under 72 characters.
- Use the body to explain *why*, not *what* (the diff shows what).

---

## Pre-commit Hooks

The following hooks run on every commit:

| Hook           | What it checks                      |
|----------------|-------------------------------------|
| `go fmt`       | Code is `gofmt`-formatted           |
| `go vet`       | No suspicious constructs            |
| `go build`     | The project compiles                |
| `go test`      | All tests pass                      |
| `golangci-lint`| No lint issues                      |

Run all hooks manually:

```bash
pre-commit run --all-files
```

---

## Pull Requests

1. Fork the repository and create a feature branch.
2. Make your changes with tests.
3. Ensure `pre-commit run --all-files` passes.
4. Open a PR against `main` with a clear description of the change and why it is needed.
5. Link any related issues.

PRs that introduce new CLI flags or commands should include updates to `docs/features.md`.

---

## Makefile Targets

| Target        | Description                          |
|---------------|--------------------------------------|
| `make build`  | Build the binary to `./bin/workstreams` |
| `make install`| Install to `$GOPATH/bin`             |
| `make test`   | Run all tests                        |
| `make lint`   | Run golangci-lint                    |
| `make clean`  | Remove build artefacts               |
