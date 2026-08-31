/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"github.com/spf13/cobra"
)

// completeCuttings returns the branch names of all active (non-main)
// worktrees, with the worktree path as a description. Used by commands that
// operate on existing cuttings (shell, remove).
func completeCuttings(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	if deps.wt == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	trees, err := deps.wt.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var completions []string
	for _, t := range trees {
		if t.IsMain || t.Branch == "" {
			continue
		}
		completions = append(completions, t.Branch+"\t"+t.Path)
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeBranches returns all local git branch names. Used by commands that
// accept any branch name (new, run --branch, --source flags).
func completeBranches(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	if deps.wt == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	branches, err := deps.wt.ListBranches()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return branches, cobra.ShellCompDirectiveNoFileComp
}
