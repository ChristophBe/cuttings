---
title: Documentation
description: >-
  Command reference and shell-completion setup for cuttings, the CLI that
  turns git worktrees into isolated dev environments for parallel AI coding
  agents.
toc: false
cascade:
  - target:
      path: /docs/features
    params:
      seoTitle: "Command Reference"
      seoDescription: >-
        Full command reference for cuttings: every git worktree command,
        flag, environment variable, and exit code — new, list, shell,
        remove, and run.
  - target:
      path: /docs/shell-completion
    params:
      seoTitle: "Shell Completion"
      seoDescription: >-
        Enable tab-completion for the cuttings git worktree CLI in Bash,
        Zsh, or Fish — for commands, branch names, and flags.
---

`cuttings` is a git worktree manager built for running AI coding agents in
parallel: every command below takes, lists, or clears away an isolated git
worktree — its own directory and shell — so each agent or branch stays
completely conflict-free.

Everything below is generated straight from `cuttings`'s built-in
`--help` output (see `docs/features.md` in the repo, kept accurate by CI's
docs-check job), so it can never drift out of sync with the CLI itself.

{{< cards >}}
  {{< card link="features" title="Command Reference" subtitle="Every command, flag, environment variable, storage layout, and exit code." icon="terminal" >}}
  {{< card link="shell-completion" title="Shell Completion" subtitle="Tab-completion setup for Bash, Zsh, and Fish." icon="cursor-click" >}}
{{< /cards >}}
