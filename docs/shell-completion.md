# Shell Completion

`cuttings` supports tab-completion for all major shells via Cobra's built-in completion system. Once installed, the following are completed dynamically:

| Context | Suggestions |
|---|---|
| `cuttings shell <TAB>` | Active cutting branches |
| `cuttings remove <TAB>` | Active cutting branches |
| `cuttings new <TAB>` | All local git branches |
| `cuttings new --source <TAB>` | All local git branches |
| `cuttings run <TAB>` | All local git branches (only as the first argument) |
| `cuttings run --branch <TAB>` | All local git branches (deprecated flag) |
| `cuttings run --source <TAB>` | All local git branches |

---

## Bash

### Temporary (current session only)

```bash
source <(cuttings completion bash)
```

### Persistent

```bash
cuttings completion bash >> ~/.bashrc
```

On macOS, Bash reads `~/.bash_profile` for login shells, so you may need to append there instead (or source `~/.bashrc` from `~/.bash_profile`):

```bash
cuttings completion bash >> ~/.bash_profile
```

Restart your shell or run `source ~/.bashrc` to apply immediately.

> **Note:** Bash completion requires `bash-completion` v2 or later for the best experience. On macOS the system Bash is version 3 — install a newer Bash and `bash-completion@2` via your package manager.

---

## Zsh

### Temporary (current session only)

```zsh
source <(cuttings completion zsh)
```

### Persistent

Add the completion script to a directory on your `$fpath` and enable `compinit`. The recommended location is `~/.zsh/completions/`:

```zsh
mkdir -p ~/.zsh/completions
cuttings completion zsh > ~/.zsh/completions/_cuttings
```

Then make sure your `~/.zshrc` contains the following **before** any call to `compinit`:

```zsh
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit && compinit
```

Restart your shell or run `exec zsh` to apply.

### Oh My Zsh

If you use Oh My Zsh, place the completion file in the custom completions directory:

```zsh
cuttings completion zsh > "${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/completions/_cuttings"
```

Restart your shell or run `omz reload`.

---

## Fish

```fish
cuttings completion fish > ~/.config/fish/completions/cuttings.fish
```

Fish loads completion files automatically — no further configuration is needed. Start a new Fish session to pick up the completions.

---

## PowerShell

> **Note:** `cuttings` uses `syscall.Exec` for shell spawning, which is not available on Windows. The binary is Unix-only. PowerShell completion is therefore not documented here.

---

## Verifying completions work

After installation, open a new shell session inside a git repository that has active cuttings and test:

```bash
cuttings shell <TAB>     # should list active cutting branches
cuttings new <TAB>       # should list local git branches
```

If nothing appears, ensure the completion script was sourced and that your shell's completion system is initialised correctly.
