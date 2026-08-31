/*
Copyright © 2026 Christoph Becker
*/
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ChristophBe/cuttings/internal/config"
)

func TestLoad_NoFile(t *testing.T) {
	dir := t.TempDir()

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() with no config file: unexpected error: %v", err)
	}

	if got := cfg.WorktreesDir; got != config.DefaultWorktreesDir {
		t.Errorf("WorktreesDir = %q, want %q", got, config.DefaultWorktreesDir)
	}
	if got := cfg.DefaultBranch; got != config.DefaultDefaultBranch {
		t.Errorf("DefaultBranch = %q, want %q", got, config.DefaultDefaultBranch)
	}
	if got := cfg.RunCleanupOnSignal; got != config.DefaultRunCleanupOnSignal {
		t.Errorf("RunCleanupOnSignal = %v, want %v", got, config.DefaultRunCleanupOnSignal)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	content := "worktrees_dir: .ws\ndefault_branch: main\n"
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if got := cfg.WorktreesDir; got != ".ws" {
		t.Errorf("WorktreesDir = %q, want %q", got, ".ws")
	}
	if got := cfg.DefaultBranch; got != "main" {
		t.Errorf("DefaultBranch = %q, want %q", got, "main")
	}
}

func TestLoad_RunCleanupOnSignal_FromFile(t *testing.T) {
	dir := t.TempDir()
	content := "run_cleanup_on_signal: false\n"
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.RunCleanupOnSignal {
		t.Error("RunCleanupOnSignal = true, want false (from config file)")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	// Write a file with one value, then override with an env var.
	content := "worktrees_dir: .from-file\n"
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CUTTINGS_WORKTREES_DIR", ".from-env")
	t.Setenv("CUTTINGS_DEFAULT_BRANCH", "develop")
	t.Setenv("CUTTINGS_RUN_CLEANUP_ON_SIGNAL", "false")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if got := cfg.WorktreesDir; got != ".from-env" {
		t.Errorf("WorktreesDir = %q, want env override %q", got, ".from-env")
	}
	if got := cfg.DefaultBranch; got != "develop" {
		t.Errorf("DefaultBranch = %q, want env override %q", got, "develop")
	}
	if cfg.RunCleanupOnSignal {
		t.Error("RunCleanupOnSignal = true, want false (from env override)")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(":\tinvalid: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := config.Load(dir); err == nil {
		t.Error("Load() with invalid YAML: expected error, got nil")
	}
}

func TestFilePath(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	want := filepath.Join(dir, config.ConfigFileName)
	if got := cfg.FilePath(); got != want {
		t.Errorf("FilePath() = %q, want %q", got, want)
	}
}
