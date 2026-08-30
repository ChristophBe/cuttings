---
title: workstreams
layout: hextra-home
toc: false
description: >-
  workstreams turns git worktrees into isolated, one-command dev environments —
  built for parallel work with AI coding agents like Claude Code.
---

<div class="hx:relative hero-wrap">
<div class="hero-glow"></div>

<div class="hero-grid">
<div class="hero-grid-text">

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

<div class="hx:flex hx:gap-4 hx:flex-wrap">
{{< hextra/hero-button text="Get Started" link="#install" >}}
{{< hextra/hero-button text="View on GitHub" link="https://github.com/ChristophBe/workstreams" >}}
</div>

</div>

<div class="hero-grid-terminal" id="hero-terminal" aria-hidden="true">

```bash {filename="Terminal"}
workstreams new feature/my-feature
# isolated worktree + shell, ready to go
```

```bash {filename="Terminal"}
workstreams run -- go test ./...
# runs it, tears down automatically
```

</div>
</div>
</div>

<script>
(function () {
  var container = document.getElementById('hero-terminal');
  if (!container) return;

  var screens = [].slice.call(container.querySelectorAll('.hextra-code-block'));
  if (!screens.length) return;

  screens.forEach(function (screen, i) {
    screen.style.display = i === 0 ? '' : 'none';
  });

  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    return;
  }

  var HOLD_MS = 2500;
  var SECONDS_PER_CHAR = 0.035;
  var MIN_LINE_DURATION = 0.12;
  var LINE_GAP = 0.15;
  var ERASE_SPEEDUP = 0.5;

  function lines(screen) {
    var code = screen.querySelector('.highlight pre code');
    return code ? [].slice.call(code.querySelectorAll(':scope > .line')) : [];
  }

  function armLine(line, startDelay, speed) {
    var chars = Math.max((line.textContent || '').trim().length, 1);
    var duration = Math.max(chars * SECONDS_PER_CHAR * speed, MIN_LINE_DURATION);
    line.style.transitionProperty = 'clip-path';
    line.style.transitionDelay = startDelay + 's';
    line.style.transitionDuration = duration + 's';
    line.style.transitionTimingFunction = 'steps(' + chars + ', end)';
    return startDelay + duration + LINE_GAP;
  }

  // Force-reflow technique instead of requestAnimationFrame: rAF is paused
  // by the browser whenever the tab isn't the visible/foreground one, which
  // would permanently stall this loop rather than just delaying it. Reading
  // a layout property (offsetWidth) forces a synchronous style flush, so a
  // "set to state A, force flush, set to state B" sequence reliably
  // triggers the transition from A to B without waiting on rAF at all.
  function typeIn(screen, cb) {
    var ls = lines(screen);
    ls.forEach(function (line) {
      line.style.transition = 'none';
      line.style.clipPath = 'inset(0 100% 0 0)';
    });
    void screen.offsetWidth;
    var delay = 0;
    ls.forEach(function (line) {
      delay = armLine(line, delay, 1);
    });
    void screen.offsetWidth;
    ls.forEach(function (line) {
      line.style.clipPath = 'inset(0 0 0 0)';
    });
    window.setTimeout(cb, delay * 1000);
  }

  function eraseOut(screen, cb) {
    var ls = lines(screen).slice().reverse();
    var delay = 0;
    ls.forEach(function (line) {
      delay = armLine(line, delay, ERASE_SPEEDUP);
    });
    void screen.offsetWidth;
    ls.forEach(function (line) {
      line.style.clipPath = 'inset(0 100% 0 0)';
    });
    window.setTimeout(cb, delay * 1000);
  }

  var index = 0;
  function cycle() {
    var screen = screens[index];
    typeIn(screen, function () {
      window.setTimeout(function () {
        eraseOut(screen, function () {
          screen.style.display = 'none';
          index = (index + 1) % screens.length;
          screens[index].style.display = '';
          cycle();
        });
      }, HOLD_MS);
    });
  }

  cycle();
})();
</script>

<hr class="hx:w-full hx:mt-16 hx:mb-16 hx:border-gray-200 hx:dark:border-neutral-800" />

<div class="hx:mb-8">
{{< hextra/hero-section >}}
  Why workstreams?
{{< /hextra/hero-section >}}
</div>

When you're working with AI coding assistants — or simply juggling multiple
features — you often need several completely isolated copies of a repository,
each on a different branch, each with its own terminal session. Switching
branches in a single directory disrupts uncommitted work and forces every
tool, human or AI, to reload context.

`workstreams` wraps [`git worktree`](https://git-scm.com/docs/git-worktree)
in a single command that creates the isolated directory *and* drops you
straight into a shell inside it.

<div class="hx:mb-8"></div>

{{< hextra/feature-grid cols="3" >}}
  {{< hextra/feature-card
    title="Instant isolation"
    subtitle="One command creates a worktree and opens a shell inside it — no manual git worktree juggling."
    icon="lightning-bolt"
  >}}
  {{< hextra/feature-card
    title="Zero state"
    subtitle="All state lives in git itself (git worktree list). No daemon, no config database, nothing to get out of sync."
    icon="database"
  >}}
  {{< hextra/feature-card
    title="One-off commands"
    subtitle="`workstreams run -- <cmd>` spins up a worktree, runs your command, and tears it down automatically — exit code and all. No shell, no manual cleanup."
    icon="play"
  >}}
{{< /hextra/feature-grid >}}

<hr class="hx:w-full hx:mt-16 hx:mb-16 hx:border-gray-200 hx:dark:border-neutral-800" />

<div class="hx:mb-8">
{{< hextra/hero-section >}}
  Built for parallel AI coding agents
{{< /hextra/hero-section >}}
</div>

Tools like [Claude Code](https://claude.ai/claude-code) work best with a
directory of their own. Open one `workstreams` session per feature and every
agent gets a clean, conflict-free copy of the repo — no branch switching, no
file locks, no lost context. Two agents, two worktrees, running at once:

<div class="hx:mb-8"></div>

<div class="hx:grid hx:md:grid-cols-2 hx:gap-4">

```bash {filename="Terminal 1"}
workstreams new feature/auth-refactor
# Claude Code works here, on its own branch,
# in its own directory, with its own shell.
```

```bash {filename="Terminal 2"}
workstreams new feature/new-dashboard
# A second agent (or a second you) works here —
# completely isolated from the session above.
```

</div>

<hr class="hx:w-full hx:mt-16 hx:mb-16 hx:border-gray-200 hx:dark:border-neutral-800" />

<div class="hx:mb-8" id="install">
{{< hextra/hero-section >}}
  Install
{{< /hextra/hero-section >}}
</div>

{{< tabs >}}
{{< tab name="Pre-built binary" >}}
Download the latest release for your platform from
[GitHub Releases](https://github.com/ChristophBe/workstreams/releases/latest),
then extract it and put the binary on your `PATH`. The steps differ slightly
by OS since releases ship as `.tar.gz` for macOS/Linux and `.zip` for Windows:

{{< tabs >}}
{{< tab name="macOS / Linux" >}}
```bash
# Example for Linux amd64 — swap in your platform's archive name
tar -xzf workstreams_linux_amd64.tar.gz
mv workstreams /usr/local/bin/
```
{{< /tab >}}
{{< tab name="Windows" >}}
```powershell
# PowerShell
Expand-Archive workstreams_windows_amd64.zip -DestinationPath .
Move-Item workstreams.exe "$env:USERPROFILE\bin\workstreams.exe"
```

Make sure the destination directory (e.g. `%USERPROFILE%\bin`) is on your
`PATH`.
{{< /tab >}}
{{< /tabs >}}

Available platforms: `linux_amd64`, `linux_arm64`, `darwin_amd64`,
`darwin_arm64`, `windows_amd64`.

→ [Download the latest release](https://github.com/ChristophBe/workstreams/releases/latest)
{{< /tab >}}
{{< tab name="go install" >}}
Requires [Go](https://golang.org/doc/install). This fetches the module,
compiles it, and installs the binary into `$(go env GOPATH)/bin` — make sure
that directory is on your `PATH`.

```bash
go install github.com/ChristophBe/workstreams@latest
```
{{< /tab >}}
{{< tab name="Build from source" >}}
Requires [Go](https://golang.org/doc/install) and `make`.

```bash
git clone https://github.com/ChristophBe/workstreams.git
cd workstreams
make install
```

On native Windows (outside WSL), `make` isn't available by default — run
`go install -trimpath .` from the cloned directory instead.

Building from source to make a change rather than just installing? See the
[Contributing guide](https://github.com/ChristophBe/workstreams/blob/main/CONTRIBUTING.md)
for the full dev setup — tests, linting, and pre-commit hooks.
{{< /tab >}}
{{< /tabs >}}

<hr class="hx:w-full hx:mt-16 hx:mb-16 hx:border-gray-200 hx:dark:border-neutral-800" />

<div class="hx:mb-8">
{{< hextra/hero-section >}}
  Quickstart
{{< /hextra/hero-section >}}
</div>

{{< tabs >}}
{{< tab name="One-off command (recommended)" >}}
Most of the time you don't need an interactive shell at all — you just want
a command to run against a clean copy of the repo:

```bash
workstreams run -- go test ./...
```

This creates a temporary worktree at the current branch's HEAD, runs the
command inside it, and removes the worktree when the command finishes —
whether it succeeds or fails. The command's exit code is propagated back to
your shell, so it's a drop-in wrapper for CI steps or agent scripts. Nothing
to clean up afterward.

Need the worktree to stick around on a named branch instead of a throwaway
detached HEAD?

```bash
workstreams run --branch feature/my-feature -- go test ./...
```

`--branch` creates the branch (and its worktree) if it doesn't exist yet, or
reuses it if it does. Since a reused workstream isn't temporary, you'll be
asked whether to remove it once the command finishes — add `--remove-after`
to skip the prompt and always remove it, e.g. from a script or CI.
{{< /tab >}}
{{< tab name="Interactive session" >}}
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
{{< /tab >}}
{{< /tabs >}}

<hr class="hx:w-full hx:mt-16 hx:mb-16 hx:border-gray-200 hx:dark:border-neutral-800" />

<div class="hx:mb-8">
{{< hextra/hero-section >}}
  Learn more
{{< /hextra/hero-section >}}
</div>

{{< cards cols="4" >}}
  {{< card link="docs/features" title="Full command reference" subtitle="Every command, flag, environment variable, and exit code." icon="terminal" >}}
  {{< card link="docs/shell-completion" title="Shell completion" subtitle="Bash, Zsh, and Fish setup instructions." icon="cursor-click" >}}
  {{< card link="https://github.com/ChristophBe/workstreams/blob/main/CONTRIBUTING.md" title="Contribute" subtitle="Coding guidelines, project layout, and the PR checklist — issues and PRs welcome." icon="heart" >}}
  {{< card link="https://github.com/ChristophBe/workstreams" title="Source on GitHub" subtitle="MIT licensed — issues, PRs, and ⭐ stars all help." icon="github" >}}
{{< /cards >}}
