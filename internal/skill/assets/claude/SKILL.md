---
name: workstreams-parallel
description: Split the current task into independent branches and work on them in parallel using the workstreams CLI, from inside a single Claude Code session (no manual terminals). Use when the user wants to parallelize work, branch out into separate workstreams, or says things like "spin up workstreams for X and Y and work on them in parallel" or "parallelize this with workstreams".
---

# Parallel work with workstreams

Orchestrate several independent pieces of work at once by creating a real
`workstreams` workstream per branch and dispatching a background subagent
into each one — all from this session, without asking the user to open
terminals.

## 1. Preconditions

- Confirm the `workstreams` CLI is installed: `command -v workstreams`. If
  missing, stop and tell the user to install it (see this repo's README) —
  don't fall back to raw `git worktree` commands; the point of this skill is
  to use the real CLI so `workstreams list/shell/remove` stay usable
  afterward.
- Confirm you're inside a git repository: `git rev-parse --show-toplevel`.

## 2. Split the work

Work out with the user (or from their request) N genuinely independent
chunks of work — non-overlapping files/areas, so the branches won't collide
on merge — and a branch name per chunk (e.g. `feature/auth-refactor`,
`feature/new-dashboard`). If the split is ambiguous, ask before proceeding.

Note: if this session is already running inside a workstream (`git
rev-parse --show-toplevel` resolves to a `.worktrees/...` path rather than
the original clone), new workstreams you create here will nest under the
*current* worktree's root, not the original repo. That's expected —
`workstreams` supports nested workstreams and always reflects the innermost
`WORKSTREAM_BRANCH`/`WORKSTREAM_PATH` — just don't assume the repo root is
the top-level clone.

## 3. Create a workstream per branch

`workstreams new <branch>` normally ends by dropping into an interactive
shell, which would hang Claude's Bash tool. Use the bundled helper instead,
which drives the real `workstreams` binary non-interactively and prints the
workstream's absolute path on success.

This skill was installed either locally (`.claude/skills/workstreams-parallel/`
under the current project) or globally (`~/.claude/skills/workstreams-parallel/`
in your home directory) — check which one actually exists before running
the script, since the two installs are independent copies:

```bash
skill_dir=".claude/skills/workstreams-parallel"
[ -d "$skill_dir" ] || skill_dir="$HOME/.claude/skills/workstreams-parallel"

"$skill_dir/scripts/create-workstream.sh" <branch> [base-ref]
```

- `base-ref` is optional — pass it to fork a new branch from something other
  than the repo's configured default / HEAD (mirrors `workstreams new`'s
  `--from` flag).
- If a workstream for that branch already exists, the script reuses it and
  still prints its path — safe to re-run.
- Run this **once per branch, sequentially** (not as parallel Bash calls) —
  concurrent `git worktree add` invocations against the same repo can race
  on git's index lock. Collect all the resulting paths before moving on.

## 4. Dispatch the parallel work

Once every workstream path is known, launch one background `Agent` tool call
**per workstream, all in a single message** — that's what makes the work
actually run in parallel. For each agent:

- State the absolute workstream path explicitly in the prompt and instruct
  it to `cd` there before any other Bash command.
- Give it a fully self-contained task description (what to build/fix, any
  relevant context) — it has no memory of this conversation.
- Do **not** pass `isolation: "worktree"` on the Agent call. That creates a
  separate Claude-managed worktree under `.claude/worktrees/`, which would
  duplicate or bypass the `workstreams`-managed directory you just created.
- Prefer `run_in_background: true` (the default) for each, since you're
  fanning out several long-running tasks and should not block on one before
  starting the next.

Tell the user which branches/paths were launched. Results arrive later as
background task notifications — do not poll or predict them.

## 5. Wrap up

When a subagent finishes, summarize its result to the user. To review or
clean up afterward:

- `workstreams list` — see all active workstreams and their paths.
- `workstreams remove <branch>` — delete a workstream's directory once its
  work is merged or abandoned (the branch itself is preserved). This refuses
  worktrees with uncommitted changes — surface that error to the user rather
  than discarding their changes; only remove after they confirm.
