/*
Copyright © 2026 Christoph Becker
*/
package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChristophBe/workstreams/internal/skill"
)

func TestInstall_ClaudeLocal_WritesFilesAndSetsExecBit(t *testing.T) {
	repoRoot := t.TempDir()

	results, err := skill.Install(skill.ScopeLocal, []string{"claude"}, repoRoot, "", false)
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Outcome != skill.OutcomeInstalled {
		t.Fatalf("Install() results = %+v, want one OutcomeInstalled", results)
	}

	dir := filepath.Join(repoRoot, ".claude", "skills", "workstreams-parallel")
	skillMD := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		t.Fatalf("SKILL.md not written: %v", err)
	}

	script := filepath.Join(dir, "scripts", "create-workstream.sh")
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("create-workstream.sh not written: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("create-workstream.sh mode = %v, want executable bit set", info.Mode())
	}
}

func TestInstall_WholeFile_RespectsOverwrite(t *testing.T) {
	repoRoot := t.TempDir()

	if _, err := skill.Install(skill.ScopeLocal, []string{"cursor"}, repoRoot, "", false); err != nil {
		t.Fatalf("first Install() unexpected error: %v", err)
	}

	dest := filepath.Join(repoRoot, ".cursor", "rules", "workstreams-parallel.mdc")
	if err := os.WriteFile(dest, []byte("locally modified"), 0o600); err != nil {
		t.Fatal(err)
	}

	results, err := skill.Install(skill.ScopeLocal, []string{"cursor"}, repoRoot, "", false)
	if err != nil {
		t.Fatalf("second Install() (no overwrite) unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Outcome != skill.OutcomeSkippedExists {
		t.Fatalf("results = %+v, want one OutcomeSkippedExists", results)
	}
	//nolint:gosec // dest is a path within t.TempDir(), not user-controlled input.
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "locally modified" {
		t.Errorf("file was overwritten despite --overwrite not being set")
	}

	results, err = skill.Install(skill.ScopeLocal, []string{"cursor"}, repoRoot, "", true)
	if err != nil {
		t.Fatalf("third Install() (overwrite=true) unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Outcome != skill.OutcomeInstalled {
		t.Fatalf("results = %+v, want one OutcomeInstalled", results)
	}
	//nolint:gosec // dest is a path within t.TempDir(), not user-controlled input.
	data, err = os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "locally modified" {
		t.Errorf("file was not overwritten despite --overwrite being set")
	}
}

func TestInstall_SectionMerge_IdempotentAndPreservesSurroundingContent(t *testing.T) {
	repoRoot := t.TempDir()

	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	preexisting := "# Project instructions\n\nSome unrelated guidance.\n"
	if err := os.WriteFile(agentsPath, []byte(preexisting), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := skill.Install(skill.ScopeLocal, []string{"agents-md"}, repoRoot, "", false); err != nil {
		t.Fatalf("first Install() unexpected error: %v", err)
	}
	//nolint:gosec // agentsPath is a path within t.TempDir(), not user-controlled input.
	first, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), preexisting) {
		t.Errorf("pre-existing content was not preserved: %s", first)
	}
	if strings.Count(string(first), "<!-- workstreams:skill:start -->") != 1 {
		t.Errorf("expected exactly one section marker after first install, got: %s", first)
	}

	// Section-merge targets don't require --overwrite; re-running should be a
	// no-op on content (same bytes), not a duplicated section.
	if _, err := skill.Install(skill.ScopeLocal, []string{"agents-md"}, repoRoot, "", false); err != nil {
		t.Fatalf("second Install() unexpected error: %v", err)
	}
	//nolint:gosec // agentsPath is a path within t.TempDir(), not user-controlled input.
	second, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("running Install twice was not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !strings.Contains(string(second), preexisting) {
		t.Errorf("pre-existing content was lost on second install: %s", second)
	}
}

func TestInstall_UnsupportedGlobalTargets_AreSkippedNotErrored(t *testing.T) {
	home := t.TempDir()

	results, err := skill.Install(skill.ScopeGlobal, []string{"cursor", "copilot"}, "", home, false)
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v, want 2 entries", results)
	}
	for _, r := range results {
		if r.Outcome != skill.OutcomeSkippedUnsupported {
			t.Errorf("target %q outcome = %q, want %q", r.Target, r.Outcome, skill.OutcomeSkippedUnsupported)
		}
		if r.Reason == "" {
			t.Errorf("target %q has no skip reason", r.Target)
		}
	}

	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("home directory = %v, want untouched (empty)", entries)
	}
}

func TestInstall_GlobalScope_NeverTouchesRepoRoot(t *testing.T) {
	home := t.TempDir()
	bogusRepoRoot := filepath.Join(t.TempDir(), "does-not-exist")

	results, err := skill.Install(skill.ScopeGlobal, []string{"claude", "agents-md"}, bogusRepoRoot, home, false)
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}
	for _, r := range results {
		if r.Outcome != skill.OutcomeInstalled {
			t.Errorf("target %q outcome = %q, want %q", r.Target, r.Outcome, skill.OutcomeInstalled)
		}
	}

	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "workstreams-parallel", "SKILL.md")); err != nil {
		t.Errorf("global claude install missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "AGENTS.md")); err != nil {
		t.Errorf("global agents-md install missing: %v", err)
	}
	if _, err := os.Stat(bogusRepoRoot); !os.IsNotExist(err) {
		t.Errorf("bogus repoRoot was created, want it left untouched: %v", err)
	}
}

func TestInstall_AllExpandsToEveryTarget(t *testing.T) {
	repoRoot := t.TempDir()

	results, err := skill.Install(skill.ScopeLocal, nil, repoRoot, "", false)
	if err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}
	if len(results) != len(skill.AllTargets) {
		t.Errorf("results = %d entries, want %d (one per AllTargets)", len(results), len(skill.AllTargets))
	}
}

func TestInstall_UnknownTarget_Errors(t *testing.T) {
	repoRoot := t.TempDir()

	if _, err := skill.Install(skill.ScopeLocal, []string{"nonexistent-agent"}, repoRoot, "", false); err == nil {
		t.Error("Install() with unknown target: expected error, got nil")
	}
}

func TestInstall_LocalScope_RequiresRepoRoot(t *testing.T) {
	if _, err := skill.Install(skill.ScopeLocal, []string{"claude"}, "", "", false); err == nil {
		t.Error("Install() with empty repoRoot for local scope: expected error, got nil")
	}
}

func TestParseScope(t *testing.T) {
	if _, err := skill.ParseScope("local"); err != nil {
		t.Errorf("ParseScope(local) unexpected error: %v", err)
	}
	if _, err := skill.ParseScope("global"); err != nil {
		t.Errorf("ParseScope(global) unexpected error: %v", err)
	}
	if _, err := skill.ParseScope("nonsense"); err == nil {
		t.Error("ParseScope(nonsense): expected error, got nil")
	}
}
