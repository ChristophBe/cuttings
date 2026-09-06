/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the cuttings CLI.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/config"
	"github.com/ChristophBe/cuttings/internal/runlock"
	"github.com/ChristophBe/cuttings/internal/shell"
	"github.com/ChristophBe/cuttings/internal/worktree"
)

// newRootCmd builds the command tree, wiring every subcommand to d. Commands
// are constructed here rather than registered from init() functions so that a
// tree can be built more than once per process — which is what lets tests
// build an isolated tree with their own Deps and run in parallel.
func newRootCmd(d *Deps) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cuttings",
		Short: "Grow isolated git worktrees as cuttings",
		Long: `cuttings is a CLI tool for growing and managing isolated git working
environments based on git worktrees.

Each cutting is a separate directory (stored in .worktrees/<branch>/) with
its own shell session, allowing tools like Claude Code to work on multiple
branches in parallel without interfering with each other.

Examples:
  cuttings new feature/my-feature   Take a new cutting and open a shell
  cuttings list                     List all active cuttings
  cuttings shell feature/my-feature Re-open a shell in an existing cutting
  cuttings remove feature/my-feature Uproot a cutting`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return d.resolve()
		},
	}

	rootCmd.AddCommand(
		newNewCmd(d),
		newShellCmd(d),
		newListCmd(d),
		newRemoveCmd(d),
		newPruneCmd(d),
		newRunCmd(d),
		newInitCmd(d),
		newVersionCmd(),
	)
	return rootCmd
}

// resolve populates d from the repository the process was invoked in. It is
// the single wiring point for the real implementations, called once per
// invocation from the root command's PersistentPreRunE.
func (d *Deps) resolve() error {
	repoRoot, err := worktree.FindRepoRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return err
	}
	manager := worktree.NewManager(repoRoot, cfg.WorktreesDir)
	d.cfg = cfg
	d.wt = manager
	d.locks = runlock.NewStore(manager.GitCommonDir)
	sp := shell.NewSpawner()
	d.spawner = sp
	d.runner = sp
	return nil
}

// RootCmd returns the root command, for use by documentation generators. Its
// dependencies are left unresolved — documentation generation only inspects
// the command tree, it never runs a command.
func RootCmd() *cobra.Command {
	return newRootCmd(&Deps{})
}

// ExitCodeError is returned by a command that must terminate the process with
// a specific exit code rather than the usual 1. It exists so `cuttings run` can
// propagate the exit code of the command it ran while still returning normally
// through Cobra — letting every deferred cleanup finish before the process
// goes away.
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// Execute builds the command tree and runs it. This is called by main.main().
//
// Any error maps to exit code 1 (see docs/features.md's Exit Codes table),
// except an ExitCodeError, whose code is used verbatim. Cobra still does the
// error reporting; a command returning an ExitCodeError silences that itself,
// since the code — not a message — is what it has to say.
func Execute() {
	err := newRootCmd(&Deps{}).Execute()
	if err == nil {
		return
	}

	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	os.Exit(1)
}
