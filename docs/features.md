# Feature Specification

## Overview

`workstreams` is a command-line tool that manages git worktrees as isolated development environments ("workstreams"). Each workstream is a separate filesystem directory checked out to a specific branch, with an associated interactive shell session. This allows multiple tools — or multiple humans — to work on different branches of the same repository simultaneously without interference.

---

## Commands

<!-- BEGIN GENERATED COMMANDS: run `make generate-docs` to update, do not edit by hand -->

### workstreams init

Create a .workstreams.yaml config file in the repository root

#### Synopsis

Create a .workstreams.yaml configuration file in the git repository root.

The file holds project-level settings such as the worktrees storage directory
and the default branch to fork from when creating new workstreams.

Settings can also be overridden at runtime via environment variables:

  WORKSTREAMS_WORKTREES_DIR         override worktrees_dir
  WORKSTREAMS_DEFAULT_BRANCH        override default_branch
  WORKSTREAMS_RUN_CLEANUP_ON_SIGNAL override run_cleanup_on_signal

The config file is intended to be committed to the repository so the entire
team shares the same settings. Use --overwrite to replace an existing file.

```
workstreams init [flags]
```

#### Examples

```
  workstreams init
  workstreams init --overwrite
```

#### Options

```
  -h, --help        help for init
  -o, --overwrite   overwrite an existing config file
```

### workstreams list (alias: ls)

List all active workstreams

#### Synopsis

Display all git worktrees managed by workstreams, showing the branch name
and the absolute path to each worktree directory.

The main worktree (the original clone) is listed but marked separately.

```
workstreams list [flags]
```

#### Options

```
  -h, --help   help for list
```

### workstreams new

Create a new workstream and open an interactive shell

#### Synopsis

Create a new git worktree for the given branch and open an interactive
shell inside it. If the branch does not exist it will be created.

The worktree is stored at .worktrees/<branch>/ relative to the repository root.
Two environment variables are set inside the shell:

  WORKSTREAM_BRANCH  the name of the branch
  WORKSTREAM_PATH    the absolute path to the worktree directory

Use --from to specify the branch or commit to fork from when creating a new
branch. If omitted, the new branch is created from HEAD.

Exiting the shell removes you from the workstream but does not delete it.
Use "workstreams remove <branch>" to clean up.

```
workstreams new <branch> [flags]
```

#### Examples

```
  workstreams new feature/my-feature
  workstreams new feature/my-feature --from main
```

#### Options

```
  -f, --from string   branch or commit to fork from when creating a new branch (default: HEAD)
  -h, --help          help for new
```

### workstreams remove (alias: rm)

Remove a workstream worktree

#### Synopsis

Remove the git worktree for the given branch. The branch itself is preserved
so you can re-create the workstream later with "workstreams new <branch>".

The command will fail if the worktree has uncommitted changes. Use
"git -C .worktrees/<branch> checkout -- ." to discard them first, or pass
--force to discard them as part of removal.

```
workstreams remove <branch> [flags]
```

#### Examples

```
  workstreams remove feature/my-feature
```

#### Options

```
  -f, --force   remove even if the worktree has uncommitted or untracked changes
  -h, --help    help for remove
```

### workstreams run

Run a command in a temporary workstream, then clean up

#### Synopsis

Create a temporary git worktree, run the given command inside it, then
remove the worktree when the command finishes (whether it succeeds or fails).

Only the worktree directory is removed — no branch is created or deleted.

Without --branch, a detached HEAD worktree is created at the current branch's
HEAD commit (or --from if specified). With --branch, a worktree is created for
that branch (which is also created if it does not exist yet).

Use -- to separate workstreams flags from the command and its arguments:

  workstreams run -- make test
  workstreams run --branch feature/foo -- go test ./...
  workstreams run --from origin/main -- ./scripts/ci.sh

The exit code of the command is propagated to the calling shell.

```
workstreams run -- <command> [args...] [flags]
```

#### Examples

```
  workstreams run -- make test
  workstreams run --branch feature/foo -- go test ./...
```

#### Options

```
  -b, --branch string   branch to create a worktree for (created if it does not exist)
  -f, --from string     commit-ish to base the worktree on (default: HEAD)
  -h, --help            help for run
```

### workstreams shell

Open an interactive shell in an existing workstream

#### Synopsis

Open an interactive shell inside the worktree for the given branch.
The workstream must already exist — use "workstreams new <branch>" to create one.

WORKSTREAM_BRANCH and WORKSTREAM_PATH are set in the spawned shell.

```
workstreams shell <branch> [flags]
```

#### Examples

```
  workstreams shell feature/my-feature
```

#### Options

```
  -h, --help   help for shell
```

### workstreams version

Print the version and build time

```
workstreams version [flags]
```

#### Options

```
  -h, --help   help for version
```

<!-- END GENERATED COMMANDS -->

---

## Environment Variables

### Injected into workstream shells

| Variable            | Value                                                         |
|---------------------|---------------------------------------------------------------|
| `WORKSTREAM_BRANCH` | The branch name (e.g. `feature/my-feature`)                  |
| `WORKSTREAM_PATH`   | Absolute path to the worktree directory                       |

If these variables are already set (e.g. nested workstreams), they are replaced with the innermost values.

### Consumed from the environment

| Variable | Default    | Description                                      |
|----------|------------|--------------------------------------------------|
| `SHELL`  | `/bin/sh`  | The shell binary used when spawning a session    |

---

## Storage Layout

```
<repo-root>/
└── .worktrees/
    ├── feature/
    │   ├── auth-refactor/    # branch: feature/auth-refactor
    │   └── dashboard/        # branch: feature/dashboard
    └── hotfix/
        └── login-fix/        # branch: hotfix/login-fix
```

`.worktrees/` is added to `.gitignore` to prevent worktree directories from being tracked.

---

## Limitations

- **Unix only.** Shell spawning uses `syscall.Exec`, which is not available on Windows.
- **Uncommitted changes block removal.** `git worktree remove` refuses to remove a dirty worktree. Use `git -C .worktrees/<branch> stash` or `checkout -- .` first.
- **Detached HEAD.** If a branch is checked out in detached HEAD state, `workstreams list` will show an empty branch name for that worktree.
- **No daemon.** There is no background process. All state is queried from git on demand.
- **No automatic cleanup.** Removing a workstream does not delete the branch. There is currently no `workstreams prune` command.

---

## Exit Codes

| Code | Meaning                        |
|------|--------------------------------|
| `0`  | Success                        |
| `1`  | User error (bad args, not in a repo, workstream not found, etc.) |
| `2`  | Cobra usage error              |
