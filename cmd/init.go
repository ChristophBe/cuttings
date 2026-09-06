/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/config"
)

func newInitCmd(d *Deps) *cobra.Command {
	var overwrite bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a .cuttings.yaml config file in the repository root",
		Long: fmt.Sprintf(`Create a .cuttings.yaml configuration file in the git repository root.

The file holds project-level settings such as the worktrees storage directory
and the default branch to fork from when creating new cuttings.

Settings can also be overridden at runtime via environment variables:

  %-30s override %s
  %-30s override %s
  %-30s override %s

The config file is intended to be committed to the repository so the entire
team shares the same settings. Use --overwrite to replace an existing file.`,
			config.EnvKey(config.KeyWorktreesDir), config.KeyWorktreesDir,
			config.EnvKey(config.KeyDefaultBranch), config.KeyDefaultBranch,
			config.EnvKey(config.KeyRunCleanupOnSignal), config.KeyRunCleanupOnSignal,
		),
		Example: "  cuttings init\n  cuttings init --overwrite",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := d.cfg.FilePath()

			if _, err := os.Stat(path); err == nil && !overwrite {
				return fmt.Errorf("config file already exists at %s — use --overwrite to replace it", path)
			}

			if err := os.WriteFile(path, config.DefaultFileContents(), 0o644); err != nil { //nolint:gosec // 0644 is appropriate for a committed config file
				return fmt.Errorf("write config file: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", path)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&overwrite, "overwrite", "o", false, "overwrite an existing config file")
	return cmd
}
