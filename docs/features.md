# Feature Specification

## Overview

`workstreams` is a command-line tool that manages git worktrees as isolated development environments ("workstreams"). Each workstream is a separate filesystem directory checked out to a specific branch, with an associated interactive shell session. This allows multiple tools — or multiple humans — to work on different branches of the same repository simultaneously without interference.

---

## Commands

### `workstreams new <branch>`

Creates a new workstream for the given branch and opens an interactive shell inside it.

**Behaviour:**
1. Verifies the current directory is inside a git repository (walks up to find `.git`).
2. Checks whether a workstream for `<branch>` already exists at `.worktrees/<branch>/`.
   - If it does, exits with an error and suggests `workstreams shell <branch>`.
3. Checks whether `<branch>` exists in git (local or remote tracking ref).
   - If it does **not** exist: runs `git worktree add -b <branch> .worktrees/<branch>/` (creates branch from HEAD).
   - If it **does** exist: runs `git worktree add .worktrees/<branch>/ <branch>`.
4. Prints the worktree path and a hint to type `exit`.
5. Replaces the current process with `$SHELL` (falls back to `/bin/sh`) via `syscall.Exec`, with `WORKSTREAM_BRANCH` and `WORKSTREAM_PATH` injected.

**Branch name handling:**
Branch names containing `/` (e.g. `feature/foo`) are stored as nested directories: `.worktrees/feature/foo/`. The parent directories are created automatically.

**Flags:** None.

---

### `workstreams list` (alias: `ls`)

Lists all git worktrees in the current repository.

**Output columns:**
| Column   | Description                                    |
|----------|------------------------------------------------|
| `BRANCH` | Branch name checked out in the worktree        |
| `PATH`   | Absolute path to the worktree directory        |
| `TYPE`   | `main` (original clone) or `workstream`        |

**Behaviour:**
- Calls `git worktree list --porcelain` and parses the output.
- Includes the main worktree (the original clone) marked as `main`.
- Worktrees in detached HEAD state show an empty branch.

**Flags:** None.

---

### `workstreams shell <branch>`

Opens an interactive shell in an existing workstream.

**Behaviour:**
1. Verifies the workstream at `.worktrees/<branch>/` exists.
   - If not, exits with an error and suggests `workstreams new <branch>`.
2. Replaces the current process with `$SHELL` via `syscall.Exec`, with `WORKSTREAM_BRANCH` and `WORKSTREAM_PATH` injected.

**Flags:** None.

---

### `workstreams remove <branch>` (alias: `rm`)

Removes the worktree for the given branch. The branch itself is preserved.

**Behaviour:**
1. Verifies the worktree at `.worktrees/<branch>/` exists.
   - If not, exits with an error.
2. Runs `git worktree remove .worktrees/<branch>/`.
   - Fails if the worktree has uncommitted changes. Discard them first with `git -C .worktrees/<branch> checkout -- .`.
3. Prints a confirmation message.

**Flags:** None.

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
