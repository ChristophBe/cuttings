# workstreams

[![Release](https://github.com/ChristophBe/workstreams/actions/workflows/release.yml/badge.svg)](https://github.com/ChristophBe/workstreams/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/ChristophBe/workstreams)](https://github.com/ChristophBe/workstreams/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/ChristophBe/workstreams)](https://goreportcard.com/report/github.com/ChristophBe/workstreams)
[![License: MIT](https://img.shields.io/github/license/ChristophBe/workstreams)](LICENSE)

A CLI tool for creating and managing isolated git working environments based on git worktrees. Each workstream is a separate directory with its own shell session, enabling tools like [Claude Code](https://claude.ai/claude-code) to work on multiple branches in parallel without interference.

## The Problem

When working with AI coding assistants (or simply juggling multiple features), you often need multiple, completely isolated copies of a repository — each on a different branch, each with its own terminal session. Switching branches in a single directory disrupts uncommitted work and forces tools to reload context.

`workstreams` solves this by wrapping git worktrees with a single command that creates the isolated directory *and* drops you into a shell inside it.

## Features

- **Instant isolation** — one command creates a worktree and opens a shell in it
- **Branch flexibility** — creates a new branch if it does not exist, uses an existing one otherwise
- **Zero state** — all state is stored by git itself (`git worktree list`); no daemon or config database
- **Environment injection** — `WORKSTREAM_BRANCH` and `WORKSTREAM_PATH` are set in the shell so prompts and tools know their context
- **Co-located worktrees** — stored at `.worktrees/<branch>/` inside the repository, easy to find and gitignored

## Installation

### Download pre-built binary (recommended)

Download the latest release for your platform from the [GitHub Releases page](https://github.com/ChristophBe/workstreams/releases/latest), extract the archive, and move the binary to a directory on your `PATH`:

```bash
# Example for Linux amd64
tar -xzf workstreams_linux_amd64.tar.gz
mv workstreams /usr/local/bin/
```

Available platforms: `linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64`, `windows_amd64`.

### Via Go install

```bash
go install github.com/ChristophBe/workstreams@latest
```

### Build from source

```bash
git clone https://github.com/ChristophBe/workstreams.git
cd workstreams
make install
```

## Releasing

Create and push a version tag to trigger the release pipeline:

```bash
make tag VERSION=v1.2.3
```

This runs tests and lint in CI before building multi-platform binaries and publishing them to GitHub Releases.

## Usage

### Create a new workstream

```bash
workstreams new feature/my-feature
```

Creates a worktree at `.worktrees/feature/my-feature/`, creates the branch if it does not exist, and opens an interactive shell inside. Type `exit` to return to your original shell. The worktree persists until you explicitly remove it.

### List active workstreams

```bash
workstreams list
# or
workstreams ls
```

Output:

```
BRANCH              PATH                                           TYPE
------              ----                                           ----
main                /path/to/repo                                  main
feature/my-feature  /path/to/repo/.worktrees/feature/my-feature    workstream
```

### Re-open a shell in an existing workstream

```bash
workstreams shell feature/my-feature
```

### Remove a workstream

```bash
workstreams remove feature/my-feature
# or
workstreams rm feature/my-feature
```

Removes the worktree directory. The git branch is preserved so you can re-create the workstream later.

## How It Works

`workstreams new <branch>` is equivalent to:

```bash
git worktree add -b <branch> .worktrees/<branch>/   # (or without -b if branch exists)
WORKSTREAM_BRANCH=<branch> WORKSTREAM_PATH=.worktrees/<branch>/ exec $SHELL
```

The shell is started with `syscall.Exec`, which *replaces* the workstreams process. This means the shell is a first-class process — signals, job control, and `exit` all behave as expected.

## Environment Variables

Variables available inside every workstream shell:

| Variable            | Value                                       |
|---------------------|---------------------------------------------|
| `WORKSTREAM_BRANCH` | Branch name (e.g. `feature/my-feature`)     |
| `WORKSTREAM_PATH`   | Absolute path to the worktree directory     |

You can use these in your shell prompt or in tool configuration to identify the active workstream.

## Parallel Usage with Claude Code

Open a terminal per feature branch:

```bash
# Terminal 1
workstreams new feature/auth-refactor

# Terminal 2
workstreams new feature/new-dashboard

# Each Claude Code session works in its own isolated directory
# with no branch conflicts or file-lock issues
```

## Shell Completion

`workstreams` supports tab-completion for branch names and active workstreams. Quick setup:

```bash
# Bash
workstreams completion bash >> ~/.bashrc

# Zsh
mkdir -p ~/.zsh/completions
workstreams completion zsh > ~/.zsh/completions/_workstreams

# Fish
workstreams completion fish > ~/.config/fish/completions/workstreams.fish
```

After restarting your shell, `workstreams shell <TAB>` and `workstreams remove <TAB>` will suggest active workstreams, and `workstreams new <TAB>` will suggest local branches.

See [docs/shell-completion.md](docs/shell-completion.md) for detailed setup instructions per shell, including Oh My Zsh and notes on Bash versions.

## See Also

- [Feature documentation](docs/features.md) — detailed feature spec, flags, and limitations
- [Shell completion](docs/shell-completion.md) — per-shell setup instructions
- [Contributing guide](CONTRIBUTING.md) — coding guidelines and development workflow
- `git worktree` — the underlying git mechanism
