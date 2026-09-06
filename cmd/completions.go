/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

// completeCuttings returns the branch names of all active (non-main)
// worktrees, with the worktree path as a description. Used by commands that
// operate on existing cuttings (shell, remove).
func (d *Deps) completeCuttings(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	if d.wt == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	trees, err := d.wt.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var completions []string
	for _, t := range worktree.Cuttings(trees) {
		completions = append(completions, t.Branch+"\t"+t.Path)
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeBranches returns all local git branch names. Used by commands that
// accept any branch name (new, run's positional branch argument and its
// deprecated --branch flag, --source flags).
func (d *Deps) completeBranches(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	if d.wt == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	branches, err := d.wt.ListBranches()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return branches, cobra.ShellCompDirectiveNoFileComp
}
