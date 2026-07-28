/*
Copyright © 2026 Christoph Becker
*/

// Package config provides Viper-based configuration loading for the workstreams CLI.
// Configuration is read from a .workstreams.yaml file at the git repository root
// and can be overridden by environment variables with the WORKSTREAMS_ prefix.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config key constants used in .workstreams.yaml and for env var mapping.
const (
	// KeyWorktreesDir is the config key for the worktrees storage directory.
	KeyWorktreesDir = "worktrees_dir"
	// KeyDefaultBranch is the config key for the default fork branch.
	KeyDefaultBranch = "default_branch"

	// ConfigFileName is the name of the config file placed at the repo root.
	ConfigFileName = ".workstreams.yaml"

	// DefaultWorktreesDir is the directory (relative to repo root) used when not configured.
	DefaultWorktreesDir = ".worktrees"
	// DefaultDefaultBranch is the default fork branch (empty = HEAD).
	DefaultDefaultBranch = ""
)

var v *viper.Viper

// Load initialises the Viper instance, sets defaults, wires env-var overrides,
// and reads the config file at <repoRoot>/.workstreams.yaml.
// A missing config file is not an error — defaults are used instead.
func Load(repoRoot string) error {
	v = viper.New()
	v.SetDefault(KeyWorktreesDir, DefaultWorktreesDir)
	v.SetDefault(KeyDefaultBranch, DefaultDefaultBranch)

	v.SetEnvPrefix("WORKSTREAMS")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	v.SetConfigFile(filepath.Join(repoRoot, ConfigFileName))
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// WorktreesDir returns the configured worktrees directory (relative to repo root).
func WorktreesDir() string { return v.GetString(KeyWorktreesDir) }

// DefaultBranch returns the configured default fork branch, or "" if not set.
func DefaultBranch() string { return v.GetString(KeyDefaultBranch) }

// FilePath returns the absolute path to the config file for the given repo root.
func FilePath(repoRoot string) string { return filepath.Join(repoRoot, ConfigFileName) }
