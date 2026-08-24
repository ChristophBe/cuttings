//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfig_FilePrecedence verifies .workstreams.yaml values (worktrees_dir,
// default_branch) are honored end-to-end when no flags/env vars override them.
func TestConfig_FilePrecedence(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "develop")
	commitFile(t, dir, "develop-only.txt", "develop content\n", "add develop-only file")
	runGit(t, dir, "checkout", "main")

	writeConfig(t, dir, "worktrees_dir: custom-dir\ndefault_branch: develop\n")

	h := newHarness(t, dir)
	newWorkstream(t, h, "foo")

	wantPath := filepath.Join(dir, "custom-dir", "foo")
	if _, err := os.Stat(filepath.Join(wantPath, "develop-only.txt")); err != nil {
		t.Fatalf("expected worktree at %s forked from develop (config default_branch): %v", wantPath, err)
	}
}

// TestConfig_EnvVarOverridesFile verifies WORKSTREAMS_* environment
// variables take precedence over .workstreams.yaml, per Viper's
// AutomaticEnv behavior in internal/config.
func TestConfig_EnvVarOverridesFile(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "develop")
	commitFile(t, dir, "develop-only.txt", "develop content\n", "add develop-only file")
	runGit(t, dir, "checkout", "main")

	writeConfig(t, dir, "worktrees_dir: custom-dir\ndefault_branch: develop\n")

	h := newHarness(t, dir).
		withEnv("WORKSTREAMS_WORKTREES_DIR", "env-dir").
		withEnv("WORKSTREAMS_DEFAULT_BRANCH", "main")
	newWorkstream(t, h, "bar")

	wantPath := filepath.Join(dir, "env-dir", "bar")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected worktree at env-var-overridden path %s: %v", wantPath, err)
	}
	if _, err := os.Stat(filepath.Join(wantPath, "develop-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected worktree forked from main (env override), not develop; stat err = %v", err)
	}
}

// writeConfig writes a .workstreams.yaml file with the given content at dir.
func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".workstreams.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .workstreams.yaml: %v", err)
	}
}
