# Contributing

Thank you for contributing to `cuttings`. This document covers the development workflow, coding conventions, and expectations for pull requests.

---

## Development Setup

**Prerequisites:**
- Go 1.22 or later — [golang.org/doc/install](https://golang.org/doc/install)
- [pre-commit](https://pre-commit.com/index.html#installation) — see installation instructions for your platform
- [golangci-lint](https://golangci-lint.run/welcome/install/) — see installation instructions for your platform
- [actionlint](https://github.com/rhysd/actionlint#installation) — see installation instructions for your platform

**Setup:**

```bash
git clone https://github.com/ChristophBe/cuttings.git
cd cuttings
pre-commit install          # install git hooks
go build ./...              # verify the build
go test ./...               # run tests
```

---

## Project Layout

```
cuttings/
├── cmd/                   # Cobra command definitions (thin layer only)
│   └── deps.go            # WorktreeManager and ShellSpawner interface definitions
├── internal/
│   ├── config/            # Configuration loading (returns *Config struct)
│   ├── worktree/          # Git worktree operations (*Manager struct)
│   └── shell/             # Shell spawning (*Spawner struct)
├── e2e/                   # Black-box CLI tests (build tag: e2e)
├── docs/                  # Feature and design documentation
├── tools/
│   └── gendocs/           # Regenerates the command reference in docs/features.md
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
- Integration tests that invoke git commands must clear git hook environment variables (`GIT_DIR`, `GIT_INDEX_FILE`, `GIT_WORK_TREE`, `GIT_OBJECT_DIRECTORY`, `GIT_COMMON_DIR`) to avoid interference when running from within a git hook.
- Aim for meaningful tests over high line coverage. Test behaviour, not implementation.

### End-to-end (e2e) tests

- Black-box CLI tests live in `e2e/` and exercise the compiled `cuttings` binary as a subprocess against real, throwaway git repositories — not the Go API directly. `e2e/main_test.go`'s `TestMain` builds the binary once and reuses it across the suite.
- They are gated behind the `e2e` build tag (`//go:build e2e`), so plain `go build ./...`, `go vet ./...`, and `go test ./...` (including the pre-commit `go-test` hook) skip them automatically. Run them explicitly with `make e2e` (or `go test -tags=e2e ./e2e/...`).
- **Every new command, flag, or behavior change must include an e2e scenario in `e2e/`, in addition to unit tests for the underlying `internal/` logic and `cmd/` `RunE` wiring.**
- e2e tests must stay hermetic: each test gets its own throwaway repo and an isolated `$HOME` via the `harness` helper in `e2e/harness_test.go` — never rely on the developer's or CI runner's real environment.
- Commands that spawn an interactive shell (`new`, `shell`) replace the process via `syscall.Exec`, so a real shell would hang waiting for input. Set `SHELL` to the fixture at `e2e/testdata/fakeshell.sh` (a non-interactive script that echoes the injected env vars and exits) when exercising them — see `fakeShellPath()` in `e2e/repo_test.go`.

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

### Release process

Tagging and publishing both live in a single workflow,
`.github/workflows/release.yml`, which triggers on every push to `main`, on
any `v*` tag push, and via manual dispatch. Its jobs run in sequence:

1. **`checks`** — build, unit tests, lint, e2e, and the docs-drift check
   (`checks.yml`, shared with `pr.yml`/`feature.yml`).
2. **`tag`** — only on a push to `main`, after `checks` succeeds. Analyzes
   commits since the last tag using
   [semantic-release](https://github.com/semantic-release/semantic-release)
   (Conventional Commits → semver: `feat` → minor, `fix`/`perf` → patch,
   `BREAKING CHANGE:` → major). If a release is warranted, it runs
   `make tag VERSION=vX.Y.Z`, pushing the new tag.
3. **`goreleaser`** — publishes the release via GoReleaser (`.goreleaser.yaml`).
   It runs either right after `tag` creates a new tag (same workflow run — no
   cross-workflow dispatch needed) or when a tag is pushed directly (e.g. a
   manual `make tag`), in which case `checks` above serves as the safety net
   since a direct tag push isn't otherwise CI-gated. This job:
   - builds and archives binaries for linux/darwin/windows, as before;
   - signs the `darwin` binaries with a Developer ID Application certificate
     and submits them to Apple's notary service via
     [`quill`](https://github.com/anchore/quill) — cross-signing from the
     Linux runner, no macOS runner or Xcode needed. This is what keeps
     Gatekeeper from blocking downloaded macOS binaries;
   - builds `.deb`/`.rpm`/`.apk` packages (`nfpm`) and attaches them to the
     GitHub Release;
   - generates an SPDX SBOM per archive/package (`sboms`, via `syft`);
   - cosign-signs `checksums.txt` keylessly using the workflow's GitHub OIDC
     token (`signs`) — no key material is stored anywhere;
   - pushes an updated Homebrew Cask to `cuttings-cli/homebrew-tap` and a Scoop
     manifest to `cuttings-cli/scoop-bucket`, using the `TAP_GITHUB_TOKEN` repo
     secret (`homebrew_casks` / `scoops`).
4. **`attest-build-provenance`** — after GoReleaser, publishes a GitHub build
   provenance attestation for the binaries so `gh attestation verify` can
   confirm they were built by this workflow from the matching source tag.

Commits that are entirely `docs`/`chore`/`test`/`refactor` don't trigger a
version bump, so nothing gets tagged or released.

#### Required repository secrets

| Secret                | Used for                                                               | How to obtain                                                                             |
|------------------------|-------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|
| `GITHUB_TOKEN`         | Tagging, GitHub Release publishing                                     | Provided automatically by GitHub Actions.                                                    |
| `TAP_GITHUB_TOKEN`     | Pushing to `cuttings-cli/homebrew-tap` and `cuttings-cli/scoop-bucket`  | A fine-grained PAT with Contents: read/write on those two repos, added as a repo secret.     |
| `QUILL_SIGN_P12`       | Signing `darwin` binaries                                               | base64 of a Developer ID Application certificate + private key, exported as `.p12`.          |
| `QUILL_SIGN_PASSWORD`  | Signing `darwin` binaries                                               | The `.p12` export password.                                                                  |
| `QUILL_NOTARY_KEY`     | Notarizing `darwin` binaries                                            | base64 of an App Store Connect API `.p8` private key scoped for notarization.                |
| `QUILL_NOTARY_KEY_ID`  | Notarizing `darwin` binaries                                            | The App Store Connect API key's ID.                                                          |
| `QUILL_NOTARY_ISSUER`  | Notarizing `darwin` binaries                                            | The App Store Connect issuer ID.                                                             |

Cosign signing, SBOM generation, and build-provenance attestation need no
secrets — they authenticate via the workflow's own GitHub Actions OIDC token
(`id-token: write` / `attestations: write` permissions on the job).

Until `cuttings-cli/homebrew-tap` and `cuttings-cli/scoop-bucket` exist and
`TAP_GITHUB_TOKEN` is set, the `homebrew_casks`/`scoops` publish steps will
fail; everything else in the release (archives, packages, signing, SBOMs,
attestations) is unaffected.

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
| `actionlint`   | GitHub Actions workflow files (`.github/workflows/*.yml`) are valid |

Run all hooks manually:

```bash
pre-commit run --all-files
```

`make e2e` intentionally does **not** run on every commit — it builds a fresh binary and spins up real git repositories per test, which is too slow for a commit hook. Instead it runs on pull requests and on pushes to `main` as part of the full check suite (see the reusable `.github/workflows/checks.yml`, composed from the actions in `.github/actions/`). Plain feature-branch pushes run a lighter build+test+lint subset — see `.github/workflows/feature.yml` vs. `.github/workflows/pr.yml`/`.github/workflows/release.yml`.

---

## Pull Requests

1. Fork the repository and create a feature branch.
2. Make your changes with tests.
3. Ensure `pre-commit run --all-files` passes.
4. Open a PR against `main` with a clear description of the change and why it is needed.
5. Link any related issues.

The `## Commands` section of `docs/features.md` is generated from each command's
`Use`/`Short`/`Long`/`Example`/`Flags`. After adding or changing a command, run
`make generate-docs` and commit the result — CI fails the build if the
generated section is out of date. The surrounding sections (Environment
Variables, Storage Layout, Limitations, Exit Codes) are still edited by hand.
PRs that introduce new CLI flags or commands should also include a
corresponding e2e scenario in `e2e/`.

---

## Makefile Targets

| Target        | Description                          |
|---------------|--------------------------------------|
| `make build`  | Build the binary to `./bin/cuttings` |
| `make install`| Install to `$GOPATH/bin`             |
| `make test`   | Run all unit tests                   |
| `make e2e`    | Run end-to-end CLI tests             |
| `make lint`   | Run golangci-lint                    |
| `make generate-docs` | Regenerate the command reference in `docs/features.md` |
| `make clean`  | Remove build artefacts               |
