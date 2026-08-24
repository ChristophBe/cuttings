#!/usr/bin/env bash
# Usage: create-workstream.sh <branch> [base-ref]
#
# Creates (or reuses) a workstreams workstream non-interactively and prints
# its absolute path on stdout.
#
# "workstreams new" normally ends by exec-ing $SHELL to drop into an
# interactive session (internal/shell/shell.go). Pointing $SHELL at the
# "true" binary lets the real command do all of its own work (config
# loading, existing-workstream/-branch detection, "git worktree add ...")
# and then exit 0 immediately instead of hanging.
set -euo pipefail

branch="${1:?usage: create-workstream.sh <branch> [base-ref]}"
base="${2:-}"

if ! command -v workstreams >/dev/null 2>&1; then
  echo "error: workstreams CLI not found on PATH" >&2
  exit 1
fi

# "command -v true" can resolve to the shell builtin ("true", no path),
# which syscall.Exec can't run (it doesn't do PATH lookup). type -P forces
# resolution to an on-disk executable.
true_bin="$(type -P true)"
if [ -z "$true_bin" ]; then
  echo "error: no external 'true' executable found on PATH" >&2
  exit 1
fi

if [ -n "$base" ]; then
  SHELL="$true_bin" workstreams new "$branch" --from "$base" || true
else
  SHELL="$true_bin" workstreams new "$branch" || true
fi

# Resolve the path whether the workstream was just created or already
# existed ("workstreams new" errors out to stderr in the latter case, which
# is fine — we just want the path either way).
path="$(workstreams list | awk -v b="$branch" '$1 == b { print $2 }')"

if [ -z "$path" ]; then
  echo "error: could not resolve workstream path for branch '$branch'" >&2
  exit 1
fi

echo "$path"
