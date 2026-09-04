/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the cuttings CLI.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/config"
	"github.com/ChristophBe/cuttings/internal/shell"
	"github.com/ChristophBe/cuttings/internal/worktree"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
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
		repoRoot, err := worktree.FindRepoRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(repoRoot)
		if err != nil {
			return err
		}
		deps.cfg = cfg
		deps.wt = worktree.NewManager(repoRoot, cfg.WorktreesDir)
		sp := shell.NewSpawner()
		deps.spawner = sp
		deps.runner = sp
		return nil
	},
}

// RootCmd returns the root command, for use by documentation generators.
func RootCmd() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// A returned error always maps to exit code 1 (see docs/features.md's Exit
// Codes table). The "run" command is the sole exception: it propagates its
// child command's exit code directly via exitFn/os.Exit before RunE returns,
// so that path never reaches here.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
