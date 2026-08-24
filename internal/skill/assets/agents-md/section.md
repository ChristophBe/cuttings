<!-- workstreams:skill:start -->
## Parallel work with `workstreams`

This project uses the [`workstreams`](https://github.com/ChristophBe/workstreams)
CLI to give each independent branch its own isolated git worktree
(`.worktrees/<branch>/` by default). When asked to split work across several
independent branches:

1. Confirm the CLI is installed: `command -v workstreams`. If it isn't,
   say so rather than falling back to raw `git worktree` commands — using
   the real CLI keeps `workstreams list`/`shell`/`remove` usable afterward.
2. `workstreams new <branch>` normally opens an interactive shell and would
   hang a non-interactive session. Create the worktree without it by
   pointing `$SHELL` at the `true` binary so the command does its real work
   (branch/config handling, `git worktree add ...`) and then exits
   immediately instead of blocking:

   ```bash
   SHELL="$(type -P true)" workstreams new <branch> [--from <base-ref>]
   ```

   (`command -v true` can resolve to a shell builtin with no filesystem
   path, which breaks this trick — `type -P true` forces an on-disk path.)
   If the workstream already exists this command errors out; get its path
   from `workstreams list` instead.
3. Work through each created workstream directory in turn — `cd` into it
   before making changes there. If this tool supports running multiple
   concurrent sessions, you can instead start one per workstream directory
   for true parallelism.
4. `workstreams list` shows every active workstream and its path.
   `workstreams remove <branch>` deletes one once its work is merged or
   abandoned (the branch itself is preserved); it refuses worktrees with
   uncommitted changes, so surface that rather than discarding changes.
<!-- workstreams:skill:end -->
