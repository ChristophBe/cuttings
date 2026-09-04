# Feature Specification

## Overview

In horticulture, a *cutting* is a piece taken from a plant that roots in its own soil and grows into a fully independent plant, while still sharing the same genetic material as the parent. `cuttings` is a command-line tool built on that idea: it takes a cutting from your repository — a git worktree, in git's own terms — and grows it into its own isolated filesystem directory with its own interactive shell, checked out to a specific branch. This lets multiple tools — or multiple humans — work on different branches of the same repository simultaneously without interference.

---

## Commands

<!-- BEGIN GENERATED COMMANDS: run `make generate-docs` to update, do not edit by hand -->

### cuttings init

Create a .cuttings.yaml config file in the repository root

#### Synopsis

Create a .cuttings.yaml configuration file in the git repository root.

The file holds project-level settings such as the worktrees storage directory
and the default branch to fork from when creating new cuttings.

Settings can also be overridden at runtime via environment variables:

  CUTTINGS_WORKTREES_DIR         override worktrees_dir
  CUTTINGS_DEFAULT_BRANCH        override default_branch
  CUTTINGS_RUN_CLEANUP_ON_SIGNAL override run_cleanup_on_signal

The config file is intended to be committed to the repository so the entire
team shares the same settings. Use --overwrite to replace an existing file.

```
cuttings init [flags]
```

#### Examples

```
  cuttings init
  cuttings init --overwrite
```

#### Options

```
  -h, --help        help for init
  -o, --overwrite   overwrite an existing config file
```

### cuttings list (alias: ls)

List all active cuttings

#### Synopsis

Display all git worktrees managed by cuttings, showing the branch name
and the absolute path to each worktree directory.

The main worktree (the original clone) is listed but marked separately.

```
cuttings list [flags]
```

#### Options

```
  -h, --help   help for list
```

### cuttings new

Take a new cutting and open an interactive shell

#### Synopsis

Take a new git worktree for the given branch and open an interactive
shell inside it. If the branch does not exist it will be created.

The worktree is stored at .worktrees/<branch>/ relative to the repository root.
Two environment variables are set inside the shell:

  CUTTING_BRANCH  the name of the branch
  CUTTING_PATH    the absolute path to the worktree directory

Use --source to specify the branch or commit to fork from when creating a new
branch. If omitted, the new branch is created from HEAD.

Exiting the shell removes you from the cutting but does not delete it.
Use "cuttings remove <branch>" to clean up.

```
cuttings new <branch> [flags]
```

#### Examples

```
  cuttings new feature/my-feature
  cuttings new feature/my-feature --source main
```

#### Options

```
  -h, --help            help for new
  -s, --source string   branch or commit to fork from when creating a new branch (default: HEAD)
```

### cuttings remove (alias: rm)

Uproot a cutting

#### Synopsis

Uproot the git worktree for the given branch. The branch itself is preserved
so you can take the same cutting again later with "cuttings new <branch>".

The command will fail if the worktree has uncommitted changes. Use
"git -C .worktrees/<branch> checkout -- ." to discard them first, or pass
--force to discard them as part of removal.

```
cuttings remove <branch> [flags]
```

#### Examples

```
  cuttings remove feature/my-feature
```

#### Options

```
  -f, --force   remove even if the worktree has uncommitted or untracked changes
  -h, --help    help for remove
```

### cuttings run

Run a command in a temporary cutting, then clear it away

#### Synopsis

Take a temporary cutting, run the given command inside it, then
clear it away when the command finishes (whether it succeeds or fails).

Only the worktree directory is removed — no branch is created or deleted.

Without a branch argument, a detached HEAD worktree is created at the current
branch's HEAD commit (or --source if specified). With a branch argument, a
worktree is created for that branch (which is also created if it does not
exist yet).

If the branch names a cutting that already exists, its worktree is reused
in place (nothing is created) instead of failing. Since a reused cutting
isn't temporary, it is not removed automatically: once the command finishes,
you are asked whether to remove it. Use --remove-after to skip that prompt
and always remove it, e.g. from a script or CI.

Use -- to separate the branch (if any) and cuttings flags from the command
and its arguments:

  cuttings run -- make test
  cuttings run feature/foo -- go test ./...
  cuttings run --source origin/main -- ./scripts/ci.sh
  cuttings run feature/foo --remove-after -- go test ./...

The --branch/-b flag is deprecated; use the positional branch argument shown
above instead.

The exit code of the command is propagated to the calling shell.

```
cuttings run [branch] -- <command> [args...] [flags]
```

#### Examples

```
  cuttings run -- make test
  cuttings run feature/foo -- go test ./...
  cuttings run feature/foo --remove-after -- go test ./...
```

#### Options

```
  -h, --help            help for run
  -r, --remove-after    when reusing an existing branch's cutting, remove it after the command finishes without prompting
  -s, --source string   commit-ish to base the worktree on (default: HEAD)
```

### cuttings shell

Open an interactive shell in an existing cutting

#### Synopsis

Open an interactive shell inside the worktree for the given branch.
The cutting must already exist — use "cuttings new <branch>" to create one.

CUTTING_BRANCH and CUTTING_PATH are set in the spawned shell.

```
cuttings shell <branch> [flags]
```

#### Examples

```
  cuttings shell feature/my-feature
```

#### Options

```
  -h, --help   help for shell
```

### cuttings version

Print the version and build time

```
cuttings version [flags]
```

#### Options

```
  -h, --help   help for version
```

<!-- END GENERATED COMMANDS -->

---

## Environment Variables

### Injected into cutting shells

| Variable            | Value                                                         |
|---------------------|---------------------------------------------------------------|
| `CUTTING_BRANCH` | The branch name (e.g. `feature/my-feature`)                  |
| `CUTTING_PATH`   | Absolute path to the worktree directory                       |

If these variables are already set (e.g. nested cuttings), they are replaced with the innermost values.

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
- **Detached HEAD.** If a branch is checked out in detached HEAD state, `cuttings list` will show an empty branch name for that worktree.
- **No daemon.** There is no background process. All state is queried from git on demand.
- **No automatic cleanup.** Removing a cutting does not delete the branch. There is currently no `cuttings prune` command.

---

## Exit Codes

| Code | Meaning                        |
|------|--------------------------------|
| `0`  | Success                        |
| `1`  | User error (bad args, not in a repo, cutting not found, etc.) |
| `2`  | Cobra usage error              |
