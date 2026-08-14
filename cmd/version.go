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
//	-X 'github.com/ChristophBe/workstreams/cmd.Version=v1.0.0'
//	-X 'github.com/ChristophBe/workstreams/cmd.BuildTime=2026-01-01T00:00:00Z'
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and build time",
	// Override the parent's PersistentPreRunE so this command works outside any git repo.
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("workstreams %s (built %s)\n", Version, BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
