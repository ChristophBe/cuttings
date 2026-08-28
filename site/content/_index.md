---
title: workstreams
layout: hextra-home
toc: false
---

{{< hextra/hero-badge >}}
  {{< icon name="claude" attributes="height=14" >}}
  <span>Built for the AI coding-agent era</span>
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Isolated git worktrees,&nbsp;<br class="hx:sm:block hx:hidden" />one command away
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Give every branch — and every AI coding agent — its own directory&nbsp;<br class="hx:sm:block hx:hidden" />and its own shell. No daemon, no state, just git.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6 hx:flex hx:gap-4 hx:flex-wrap">
{{< hextra/hero-button text="Get Started" link="#install" >}}
{{< hextra/hero-button text="View on GitHub" link="https://github.com/ChristophBe/workstreams" >}}
</div>

<div class="hx:mt-6"></div>

## Why workstreams?

When you're working with AI coding assistants — or simply juggling multiple
features — you often need several completely isolated copies of a repository,
each on a different branch, each with its own terminal session. Switching
branches in a single directory disrupts uncommitted work and forces every
tool, human or AI, to reload context.

`workstreams` wraps [`git worktree`](https://git-scm.com/docs/git-worktree)
in a single command that creates the isolated directory *and* drops you
straight into a shell inside it.

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Instant isolation"
    subtitle="One command creates a worktree and opens a shell inside it — no manual git worktree juggling."
    icon="lightning-bolt"
  >}}
  {{< hextra/feature-card
    title="Branch flexibility"
    subtitle="Creates a new branch if it doesn't exist yet, or reuses an existing one — workstreams adapts to what's already there."
    icon="switch-horizontal"
  >}}
  {{< hextra/feature-card
    title="Zero state"
    subtitle="All state lives in git itself (git worktree list). No daemon, no config database, nothing to get out of sync."
    icon="database"
  >}}
  {{< hextra/feature-card
    title="Environment injection"
    subtitle="WORKSTREAM_BRANCH and WORKSTREAM_PATH are set inside the shell, so prompts and tools always know their context."
    icon="variable"
  >}}
  {{< hextra/feature-card
    title="Co-located worktrees"
    subtitle="Stored at `.worktrees/<branch>/` inside the repo — easy to find, and gitignored automatically."
    icon="folder-tree"
  >}}
  {{< hextra/feature-card
    title="Shell completion"
    subtitle="Tab-complete branch names and active workstreams in Bash, Zsh, and Fish."
    icon="terminal"
  >}}
{{< /hextra/feature-grid >}}

## Built for parallel AI coding agents

Tools like [Claude Code](https://claude.ai/claude-code) work best with a
directory of their own. Open one `workstreams` session per feature and every
agent gets a clean, conflict-free copy of the repo — no branch switching, no
file locks, no lost context.

{{< tabs >}}
{{< tab name="Terminal 1" >}}
```bash
workstreams new feature/auth-refactor
# Claude Code works here, on its own branch,
# in its own directory, with its own shell.
```
{{< /tab >}}
{{< tab name="Terminal 2" >}}
```bash
workstreams new feature/new-dashboard
# A second agent (or a second you) works here —
# completely isolated from the session above.
```
{{< /tab >}}
{{< /tabs >}}

## Install {#install}

{{< tabs >}}
{{< tab name="Pre-built binary" >}}
Download the latest release for your platform from
[GitHub Releases](https://github.com/ChristophBe/workstreams/releases/latest),
extract the archive, and move the binary onto your `PATH`:

```bash
# Example for Linux amd64
tar -xzf workstreams_linux_amd64.tar.gz
mv workstreams /usr/local/bin/
```

Available platforms: `linux_amd64`, `linux_arm64`, `darwin_amd64`,
`darwin_arm64`, `windows_amd64`.
{{< /tab >}}
{{< tab name="go install" >}}
```bash
go install github.com/ChristophBe/workstreams@latest
```
{{< /tab >}}
{{< tab name="Build from source" >}}
```bash
git clone https://github.com/ChristophBe/workstreams.git
cd workstreams
make install
```
{{< /tab >}}
{{< /tabs >}}

## Quickstart

{{% steps %}}

### Create a workstream

```bash
workstreams new feature/my-feature
```

Creates a worktree at `.worktrees/feature/my-feature/`, creates the branch if
it doesn't exist, and opens an interactive shell inside it. Type `exit` to
return to your original shell — the worktree persists until you remove it.

### List active workstreams

```bash
workstreams list
```

```text
BRANCH              PATH                                           TYPE
------              ----                                           ----
main                /path/to/repo                                  main
feature/my-feature  /path/to/repo/.worktrees/feature/my-feature    workstream
```

### Re-open a shell in an existing workstream

```bash
workstreams shell feature/my-feature
```

### Remove it when you're done

```bash
workstreams remove feature/my-feature
```

Removes the worktree directory. The branch itself is preserved, so you can
re-create the workstream later.

{{% /steps %}}

## Learn more

{{< cards >}}
  {{< card link="docs/features" title="Full command reference" subtitle="Every command, flag, environment variable, and exit code." icon="terminal" >}}
  {{< card link="docs/shell-completion" title="Shell completion" subtitle="Bash, Zsh, and Fish setup instructions." icon="cursor-click" >}}
  {{< card link="https://github.com/ChristophBe/workstreams" title="Source on GitHub" subtitle="MIT licensed. Issues and contributions welcome." icon="github" >}}
{{< /cards >}}
