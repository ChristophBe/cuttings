/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/run"
)

// confirmRemoval asks the user whether the reused cutting for branch should be
// removed, reading one line from in. Any answer other than "y"/"yes"
// (including EOF, e.g. no terminal attached) is treated as "no" — the safe
// default that never silently deletes existing work.
func confirmRemoval(out io.Writer, in io.Reader, branch string) bool {
	_, _ = fmt.Fprintf(out, "Remove cutting %q? [y/N]: ", branch)
	line, _ := bufio.NewReader(in).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func newRunCmd(d *Deps) *cobra.Command {
	var (
		branchFlag  string
		source      string
		removeAfter bool
	)

	cmd := &cobra.Command{
		Use:   "run [branch] -- <command> [args...]",
		Short: "Run a command in a temporary cutting, then clear it away",
		Long: `Take a temporary cutting, run the given command inside it, then
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

The exit code of the command is propagated to the calling shell.`,
		Args: func(cmd *cobra.Command, args []string) error {
			dash := cmd.Flags().ArgsLenAtDash()
			if dash > 1 {
				return fmt.Errorf("accepts at most 1 branch argument before \"--\", received %d", dash)
			}
			minArgs := 1
			if dash == 1 {
				minArgs = 2
			}
			if len(args) < minArgs {
				return errors.New("requires a command to run after \"--\"")
			}
			return nil
		},
		Example: "  cuttings run -- make test\n  cuttings run feature/foo -- go test ./...\n" +
			"  cuttings run feature/foo --remove-after -- go test ./...",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return d.completeBranches(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			branch, args, err := splitBranchAndCommand(cmd, branchFlag, args)
			if err != nil {
				return err
			}

			out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
			cleanupOnSignal := d.cfg.RunCleanupOnSignal

			provisioner := run.NewProvisioner(d.wt, d.locks, cleanupOnSignal, out, errOut)
			provisioner.SweepOrphans()

			// sigCh is only ever written to when cleanupOnSignal is true; left
			// unregistered otherwise so run falls back to plain defer-only cleanup
			// (matching Go's default, uncaught signal disposition).
			sigCh := make(chan os.Signal, 1)
			if cleanupOnSignal {
				signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
				defer signal.Stop(sigCh)
			}

			plan, err := provisioner.Provision(run.Spec{
				Branch:        branch,
				Source:        source,
				DefaultBranch: d.cfg.DefaultBranch,
				RemoveAfter:   removeAfter,
			})
			if err != nil {
				return err
			}

			if plan.AutoRemove {
				defer provisioner.Cleanup(plan)
			}

			outcome, runErr := run.Invoke(d.runner, sigCh, plan, args)
			if runErr != nil {
				return runErr
			}

			// A reused cutting that didn't opt into --remove-after is never
			// touched by the deferred cleanup; decide its fate here instead,
			// once the command has actually completed (not merely been
			// interrupted).
			if plan.Reused && !plan.AutoRemove && !outcome.Interrupted {
				if confirmRemoval(out, cmd.InOrStdin(), plan.Key) {
					provisioner.RemoveReused(plan.Key)
				} else {
					_, _ = fmt.Fprintf(out, "Leaving cutting %q in place.\n", plan.Key)
				}
			}

			if outcome.ExitCode != 0 {
				// The exit code IS the message here — the command the user ran
				// already said whatever it had to say. Silence Cobra's error and
				// usage output so a failing `cuttings run` looks exactly like
				// running the command directly.
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitCodeError{Code: outcome.ExitCode}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&branchFlag, "branch", "b", "", "branch to create a worktree for (created if it does not exist; reused if it does)")
	cmd.Flags().StringVarP(&source, "source", "s", "", "commit-ish to base the worktree on (default: HEAD)")
	cmd.Flags().BoolVarP(&removeAfter, "remove-after", "r", false, "when reusing an existing branch's cutting, remove it after the command finishes without prompting")
	_ = cmd.RegisterFlagCompletionFunc("branch", d.completeBranches)
	_ = cmd.RegisterFlagCompletionFunc("source", d.completeBranches)
	_ = cmd.Flags().MarkDeprecated("branch", "use the positional branch argument instead, e.g. \"cuttings run <branch> -- <command>\"")
	return cmd
}

// splitBranchAndCommand separates an optional branch argument (everything
// before "--") from the command to run, reconciling it with the deprecated
// --branch flag.
func splitBranchAndCommand(cmd *cobra.Command, branchFlag string, args []string) (branch string, command []string, err error) {
	var posBranch string
	if dash := cmd.Flags().ArgsLenAtDash(); dash == 1 {
		posBranch = args[0]
		args = args[1:]
	}

	switch {
	case posBranch != "" && branchFlag != "":
		return "", nil, fmt.Errorf("cannot combine the positional branch argument %q with --branch %q; --branch is deprecated, use \"cuttings run %s -- ...\" instead", posBranch, branchFlag, posBranch)
	case posBranch != "":
		return posBranch, args, nil
	}
	return branchFlag, args, nil
}
