/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/workstreams/internal/skill"
	"github.com/ChristophBe/workstreams/internal/worktree"
)

var (
	skillScope     string
	skillTargets   []string
	skillOverwrite bool
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install coding-agent instructions for using workstreams in parallel",
	Long: `Install instruction files that teach a coding agent to use workstreams
non-interactively for parallel, per-branch work.

Supported targets:

  claude      Claude Code skill (.claude/skills/workstreams-parallel/)
  agents-md   Generic AGENTS.md section, read by Codex CLI and others
  cursor      Cursor project rules (.cursor/rules/)
  copilot     GitHub Copilot repository instructions (.github/)

--scope local (the default) installs into the current git repository.
--scope global installs into your home directory, for use across every
project. Not every target has a meaningful global location (Cursor and
Copilot have none) — those are skipped for --scope global rather than
erroring.

claude and cursor are whole files; installing over an existing one requires
--overwrite. agents-md and copilot only update a marked section within the
target file, leaving the rest of it untouched, so --overwrite does not apply
to them.`,
	Example: "  workstreams skill\n  workstreams skill -t claude\n  workstreams skill -s global -t claude,agents-md",
	// Override the parent's PersistentPreRunE: --scope global must work
	// outside any git repo, so repo-root resolution happens lazily in RunE
	// below, only when --scope local actually needs it.
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	RunE: func(_ *cobra.Command, _ []string) error {
		scope, err := skill.ParseScope(skillScope)
		if err != nil {
			return err
		}

		var repoRoot string
		if scope == skill.ScopeLocal {
			repoRoot, err = worktree.FindRepoRoot()
			if err != nil {
				return fmt.Errorf("--scope local requires running inside a git repository: %w", err)
			}
		}

		home, err := os.UserHomeDir()
		if err != nil && scope == skill.ScopeGlobal {
			return fmt.Errorf("resolve home directory: %w", err)
		}

		results, err := skill.Install(scope, skillTargets, repoRoot, home, skillOverwrite)
		if err != nil {
			return err
		}

		for _, r := range results {
			_, _ = fmt.Fprintln(os.Stdout, r.String())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.Flags().StringVarP(&skillScope, "scope", "s", string(skill.ScopeLocal), "install scope: local or global")
	skillCmd.Flags().StringSliceVarP(&skillTargets, "target", "t", skill.AllTargets, "comma-separated targets: "+strings.Join(skill.AllTargets, ", ")+", all")
	skillCmd.Flags().BoolVarP(&skillOverwrite, "overwrite", "o", false, "overwrite existing whole-file targets (claude, cursor)")
}
