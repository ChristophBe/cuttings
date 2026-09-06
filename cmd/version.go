/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version and BuildTime are injected at build time via ldflags:
//
//	-X 'github.com/ChristophBe/cuttings/cmd.Version=v1.0.0'
//	-X 'github.com/ChristophBe/cuttings/cmd.BuildTime=2026-01-01T00:00:00Z'
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// newVersionCmd takes no Deps: it must work outside a git repository, so it
// never touches the worktree manager or config.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version and build time",
		// Override the parent's PersistentPreRunE so this command works outside any git repo.
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cuttings %s (built %s)\n", Version, BuildTime)
		},
	}
}
