/*
Copyright © 2026 Christoph Becker
*/

// Package config provides Viper-based configuration loading for the workstreams CLI.
// Configuration is read from a .workstreams.yaml file at the git repository root
// and can be overridden by environment variables with the WORKSTREAMS_ prefix.
//
// Usage:
//
//	cfg, err := config.Load(repoRoot)
//	if err != nil { ... }
//	fmt.Println(cfg.WorktreesDir)
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

// Config holds the resolved workstreams configuration for a repository.
type Config struct {
	// WorktreesDir is the directory (relative to repo root) where worktrees are stored.
	WorktreesDir string
	// DefaultBranch is the branch or commit-ish new workstreams fork from by default.
	// An empty string means HEAD.
	DefaultBranch string

	repoRoot string // unexported; retained for FilePath.
}

// FilePath returns the absolute path to the config file for this repository.
func (c *Config) FilePath() string {
	return filepath.Join(c.repoRoot, ConfigFileName)
}

// Load reads .workstreams.yaml at repoRoot (if present) and returns a Config.
// A missing config file is not an error — defaults are used instead.
// Environment variables with the WORKSTREAMS_ prefix override file values.
func Load(repoRoot string) (*Config, error) {
	v := viper.New()
	v.SetDefault(KeyWorktreesDir, DefaultWorktreesDir)
	v.SetDefault(KeyDefaultBranch, DefaultDefaultBranch)

	v.SetEnvPrefix("WORKSTREAMS")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	v.SetConfigFile(filepath.Join(repoRoot, ConfigFileName))
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return &Config{
		WorktreesDir:  v.GetString(KeyWorktreesDir),
		DefaultBranch: v.GetString(KeyDefaultBranch),
		repoRoot:      repoRoot,
	}, nil
}
