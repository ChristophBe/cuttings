/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"os"

	"github.com/ChristophBe/workstreams/internal/config"
	"github.com/ChristophBe/workstreams/internal/shell"
	"github.com/ChristophBe/workstreams/internal/worktree"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "workstreams",
	Short: "Manage isolated git workstream environments",
	Long: `workstreams is a CLI tool for creating and managing isolated git working
environments based on git worktrees.

Each workstream is a separate directory (stored in .worktrees/<branch>/) with
its own shell session, allowing tools like Claude Code to work on multiple
branches in parallel without interfering with each other.

Examples:
  workstreams new feature/my-feature   Create a new workstream and open a shell
  workstreams list                     List all active workstreams
  workstreams shell feature/my-feature Re-open a shell in an existing workstream
  workstreams remove feature/my-feature Remove a workstream`,
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
		deps.spawner = shell.NewSpawner()
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
