/*
Copyright © 2026 Christoph Becker
*/

// Package config provides configuration loading for the workstreams CLI.
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
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// Config key constants used in .workstreams.yaml and for env var mapping.
const (
	// KeyWorktreesDir is the config key for the worktrees storage directory.
	KeyWorktreesDir = "worktrees_dir"
	// KeyDefaultBranch is the config key for the default fork branch.
	KeyDefaultBranch = "default_branch"
	// KeyRunCleanupOnSignal is the config key controlling whether "workstreams
	// run" cleans up its worktree when interrupted by a signal (SIGINT/SIGTERM/
	// SIGHUP) or left behind by an uncatchable kill (SIGKILL/crash).
	KeyRunCleanupOnSignal = "run_cleanup_on_signal"

	// ConfigFileName is the name of the config file placed at the repo root.
	ConfigFileName = ".workstreams.yaml"

	// DefaultWorktreesDir is the directory (relative to repo root) used when not configured.
	DefaultWorktreesDir = ".worktrees"
	// DefaultDefaultBranch is the default fork branch (empty = HEAD).
	DefaultDefaultBranch = ""
	// DefaultRunCleanupOnSignal is used when run_cleanup_on_signal is not configured.
	DefaultRunCleanupOnSignal = true
)

// Config holds the resolved workstreams configuration for a repository.
type Config struct {
	// WorktreesDir is the directory (relative to repo root) where worktrees are stored.
	WorktreesDir string
	// DefaultBranch is the branch or commit-ish new workstreams fork from by default.
	// An empty string means HEAD.
	DefaultBranch string
	// RunCleanupOnSignal controls whether "workstreams run" installs signal
	// handling (so SIGINT/SIGTERM/SIGHUP still clean up the worktree) and
	// records a run lock for orphan detection (so a SIGKILL or crash is
	// cleaned up on the next "run" invocation). Set to false to fall back to
	// plain defer-only cleanup, e.g. if the lock files or signal handling
	// interfere with another tool.
	RunCleanupOnSignal bool

	repoRoot string // unexported; retained for FilePath.
}

// FilePath returns the absolute path to the config file for this repository.
func (c *Config) FilePath() string {
	return filepath.Join(c.repoRoot, ConfigFileName)
}

// fileConfig mirrors the recognized .workstreams.yaml keys. Pointer fields let
// Load distinguish "key absent" (fall back to default) from "key present".
type fileConfig struct {
	WorktreesDir       *string `yaml:"worktrees_dir"`
	DefaultBranch      *string `yaml:"default_branch"`
	RunCleanupOnSignal *bool   `yaml:"run_cleanup_on_signal"`
}

// Load reads .workstreams.yaml at repoRoot (if present) and returns a Config.
// A missing config file is not an error — defaults are used instead.
// Environment variables with the WORKSTREAMS_ prefix override file values.
func Load(repoRoot string) (*Config, error) {
	cfg := &Config{
		WorktreesDir:       DefaultWorktreesDir,
		DefaultBranch:      DefaultDefaultBranch,
		RunCleanupOnSignal: DefaultRunCleanupOnSignal,
		repoRoot:           repoRoot,
	}

	path := filepath.Join(repoRoot, ConfigFileName)
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from repoRoot, not raw user input
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		var fc fileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if fc.WorktreesDir != nil {
			cfg.WorktreesDir = *fc.WorktreesDir
		}
		if fc.DefaultBranch != nil {
			cfg.DefaultBranch = *fc.DefaultBranch
		}
		if fc.RunCleanupOnSignal != nil {
			cfg.RunCleanupOnSignal = *fc.RunCleanupOnSignal
		}
	}

	if s, ok := os.LookupEnv("WORKSTREAMS_WORKTREES_DIR"); ok {
		cfg.WorktreesDir = s
	}
	if s, ok := os.LookupEnv("WORKSTREAMS_DEFAULT_BRANCH"); ok {
		cfg.DefaultBranch = s
	}
	if s, ok := os.LookupEnv("WORKSTREAMS_RUN_CLEANUP_ON_SIGNAL"); ok {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("invalid WORKSTREAMS_RUN_CLEANUP_ON_SIGNAL value %q: %w", s, err)
		}
		cfg.RunCleanupOnSignal = b
	}

	return cfg, nil
}
