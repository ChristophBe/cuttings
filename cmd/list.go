/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ChristophBe/workstreams/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active workstreams",
	Long: `Display all git worktrees managed by workstreams, showing the branch name
and the absolute path to each worktree directory.

The main worktree (the original clone) is listed but marked separately.`,
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := worktree.FindRepoRoot()
		if err != nil {
			return err
		}

		trees, err := worktree.List(repoRoot)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "BRANCH\tPATH\tTYPE")
		fmt.Fprintln(w, "------\t----\t----")
		for _, t := range trees {
			kind := "workstream"
			if t.IsMain {
				kind = "main"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Branch, t.Path, kind)
		}
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
