/*
Copyright © 2026 Christoph Becker
*/

// Package config provides configuration loading for the cuttings CLI.
// Configuration is read from a .cuttings.yaml file at the git repository root
// and can be overridden by environment variables with the CUTTINGS_ prefix.
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
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config key constants used in .cuttings.yaml and for env var mapping.
const (
	// KeyWorktreesDir is the config key for the worktrees storage directory.
	KeyWorktreesDir = "worktrees_dir"
	// KeyDefaultBranch is the config key for the default fork branch.
	KeyDefaultBranch = "default_branch"
	// KeyRunCleanupOnSignal is the config key controlling whether "cuttings
	// run" cleans up its worktree when interrupted by a signal (SIGINT/SIGTERM/
	// SIGHUP) or left behind by an uncatchable kill (SIGKILL/crash).
	KeyRunCleanupOnSignal = "run_cleanup_on_signal"

	// ConfigFileName is the name of the config file placed at the repo root.
	ConfigFileName = ".cuttings.yaml"

	// DefaultWorktreesDir is the directory (relative to repo root) used when not configured.
	DefaultWorktreesDir = ".worktrees"
	// DefaultDefaultBranch is the default fork branch (empty = HEAD).
	DefaultDefaultBranch = ""
	// DefaultRunCleanupOnSignal is used when run_cleanup_on_signal is not configured.
	DefaultRunCleanupOnSignal = true
)

// Config holds the resolved cuttings configuration for a repository.
type Config struct {
	// WorktreesDir is the directory (relative to repo root) where worktrees are stored.
	WorktreesDir string
	// DefaultBranch is the branch or commit-ish new cuttings fork from by default.
	// An empty string means HEAD.
	DefaultBranch string
	// RunCleanupOnSignal controls whether "cuttings run" installs signal
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

// EnvPrefix is the prefix of the environment variables that override
// .cuttings.yaml values.
const EnvPrefix = "CUTTINGS_"

// EnvKey returns the environment variable that overrides the given config
// key, e.g. EnvKey(KeyWorktreesDir) == "CUTTINGS_WORKTREES_DIR".
func EnvKey(key string) string {
	return EnvPrefix + strings.ToUpper(key)
}

// DefaultFileContents returns the contents of a freshly initialized
// .cuttings.yaml: every recognized key at its default value, documented
// alongside the environment variable that overrides it. It lives here rather
// than in the init command so the template cannot drift from the keys and
// defaults it describes.
func DefaultFileContents() []byte {
	return []byte(fmt.Sprintf(`# cuttings configuration
# https://github.com/ChristophBe/cuttings

# %[1]s: directory (relative to repo root) where worktrees are stored.
# Override with env var: %[2]s
%[1]s: %[3]s

# %[4]s: branch to fork from when running "cuttings new" without --source.
# Leave empty to use HEAD.
# Override with env var: %[5]s
%[4]s: %[6]q

# %[7]s: whether "cuttings run" installs signal handling
# (SIGINT/SIGTERM/SIGHUP) and orphan detection (SIGKILL/crash) so its
# temporary worktree is still cleaned up when the process is killed. Set to
# false to fall back to plain defer-only cleanup.
# Override with env var: %[8]s
%[7]s: %[9]t
`,
		KeyWorktreesDir, EnvKey(KeyWorktreesDir), DefaultWorktreesDir,
		KeyDefaultBranch, EnvKey(KeyDefaultBranch), DefaultDefaultBranch,
		KeyRunCleanupOnSignal, EnvKey(KeyRunCleanupOnSignal), DefaultRunCleanupOnSignal,
	))
}

// fileConfig mirrors the recognized .cuttings.yaml keys. Pointer fields let
// Load distinguish "key absent" (fall back to default) from "key present".
type fileConfig struct {
	WorktreesDir       *string `yaml:"worktrees_dir"`
	DefaultBranch      *string `yaml:"default_branch"`
	RunCleanupOnSignal *bool   `yaml:"run_cleanup_on_signal"`
}

// Load reads .cuttings.yaml at repoRoot (if present) and returns a Config.
// A missing config file is not an error — defaults are used instead.
// Environment variables with the CUTTINGS_ prefix override file values.
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

	if s, ok := os.LookupEnv(EnvKey(KeyWorktreesDir)); ok {
		cfg.WorktreesDir = s
	}
	if s, ok := os.LookupEnv(EnvKey(KeyDefaultBranch)); ok {
		cfg.DefaultBranch = s
	}
	cleanupEnv := EnvKey(KeyRunCleanupOnSignal)
	if s, ok := os.LookupEnv(cleanupEnv); ok {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("invalid %s value %q: %w", cleanupEnv, s, err)
		}
		cfg.RunCleanupOnSignal = b
	}

	return cfg, nil
}
