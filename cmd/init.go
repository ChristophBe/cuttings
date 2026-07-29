/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/ChristophBe/workstreams/internal/config"
	"github.com/spf13/cobra"
)

var overwrite bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a .workstreams.yaml config file in the repository root",
	Long: `Create a .workstreams.yaml configuration file in the git repository root.

The file holds project-level settings such as the worktrees storage directory
and the default branch to fork from when creating new workstreams.

Settings can also be overridden at runtime via environment variables:

  WORKSTREAMS_WORKTREES_DIR    override worktrees_dir
  WORKSTREAMS_DEFAULT_BRANCH   override default_branch

The config file is intended to be committed to the repository so the entire
team shares the same settings. Use --overwrite to replace an existing file.`,
	Example: "  workstreams init\n  workstreams init --overwrite",
	RunE: func(_ *cobra.Command, _ []string) error {
		path := deps.cfg.FilePath()

		if _, err := os.Stat(path); err == nil && !overwrite {
			return fmt.Errorf("config file already exists at %s — use --overwrite to replace it", path)
		}

		content := fmt.Sprintf(`# workstreams configuration
# https://github.com/ChristophBe/workstreams

# worktrees_dir: directory (relative to repo root) where worktrees are stored.
# Override with env var: WORKSTREAMS_WORKTREES_DIR
%s: %s

# default_branch: branch to fork from when running "workstreams new" without --from.
# Leave empty to use HEAD.
# Override with env var: WORKSTREAMS_DEFAULT_BRANCH
%s: %q
`,
			config.KeyWorktreesDir, config.DefaultWorktreesDir,
			config.KeyDefaultBranch, config.DefaultDefaultBranch,
		)

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // 0644 is appropriate for a committed config file
			return fmt.Errorf("write config file: %w", err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Created %s\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite an existing config file")
}
