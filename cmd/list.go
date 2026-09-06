/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newListCmd(d *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active cuttings",
		Long: `Display all git worktrees managed by cuttings, showing the branch name
and the absolute path to each worktree directory.

The main worktree (the original clone) is listed but marked separately.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			trees, err := d.wt.List()
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "BRANCH\tPATH\tTYPE")
			_, _ = fmt.Fprintln(w, "------\t----\t----")
			for _, t := range trees {
				kind := "cutting"
				if t.IsMain {
					kind = "main"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", t.Branch, t.Path, kind)
			}
			return w.Flush()
		},
	}
}
