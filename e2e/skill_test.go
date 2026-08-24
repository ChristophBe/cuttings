//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkill_LocalAllTargets_InstallsEverySupportedTarget(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("skill")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "claude: installed at")
	requireContains(t, r.stdout, "agents-md: installed at")
	requireContains(t, r.stdout, "cursor: installed at")
	requireContains(t, r.stdout, "copilot: installed at")

	skillDir := filepath.Join(dir, ".claude", "skills", "workstreams-parallel")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("expected claude SKILL.md installed: %v", err)
	}
	scriptPath := filepath.Join(skillDir, "scripts", "create-workstream.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("expected claude create-workstream.sh installed: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected create-workstream.sh to be executable, mode = %v", info.Mode())
	}

	requireContains(t, readFile(t, filepath.Join(dir, "AGENTS.md")), "<!-- workstreams:skill:start -->")

	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "workstreams-parallel.mdc")); err != nil {
		t.Fatalf("expected cursor rule installed: %v", err)
	}

	requireContains(t, readFile(t, filepath.Join(dir, ".github", "copilot-instructions.md")), "<!-- workstreams:skill:start -->")
}

func TestSkill_SingleTarget_OnlyInstallsThatTarget(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("skill", "-t", "claude")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "claude: installed at")
	requireNotContains(t, r.stdout, "agents-md:")

	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "workstreams-parallel", "SKILL.md")); err != nil {
		t.Fatalf("expected claude target installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("expected AGENTS.md not created when only -t claude was requested, stat err = %v", err)
	}
}

func TestSkill_AlreadyExists_RequiresOverwrite(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	skillFile := filepath.Join(dir, ".claude", "skills", "workstreams-parallel", "SKILL.md")
	requireExitCode(t, h.run("skill", "-t", "claude"), 0)
	before := readFile(t, skillFile)

	r := h.run("skill", "-t", "claude")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "already exists")
	requireContains(t, r.stdout, "--overwrite")
	if readFile(t, skillFile) != before {
		t.Fatalf("SKILL.md content changed despite missing --overwrite")
	}

	r2 := h.run("skill", "-t", "claude", "-o")
	requireExitCode(t, r2, 0)
	requireContains(t, r2.stdout, "claude: installed at")
}

func TestSkill_SectionMerge_PreservesSurroundingContent(t *testing.T) {
	dir := initRepo(t)
	agentsMD := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsMD, []byte("# My Custom Notes\n\nDon't touch this.\n"), 0o600); err != nil {
		t.Fatalf("seed AGENTS.md: %v", err)
	}

	h := newHarness(t, dir)
	requireExitCode(t, h.run("skill", "-t", "agents-md"), 0)

	content := readFile(t, agentsMD)
	requireContains(t, content, "Don't touch this.")
	requireContains(t, content, "<!-- workstreams:skill:start -->")

	// Re-running is idempotent and still preserves the surrounding content.
	requireExitCode(t, h.run("skill", "-t", "agents-md"), 0)
	requireContains(t, readFile(t, agentsMD), "Don't touch this.")
}

func TestSkill_GlobalScope_InstallsUnderHomeAndSkipsUnsupportedTargets(t *testing.T) {
	dir := realPath(t, t.TempDir()) // deliberately not a git repo
	h := newHarness(t, dir)

	r := h.run("skill", "-s", "global")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "claude: installed at")
	requireContains(t, r.stdout, "agents-md: installed at")
	requireContains(t, r.stdout, "cursor: skipped —")
	requireContains(t, r.stdout, "copilot: skipped —")

	if _, err := os.Stat(filepath.Join(h.home, ".claude", "skills", "workstreams-parallel", "SKILL.md")); err != nil {
		t.Fatalf("expected claude installed under $HOME: %v", err)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".codex", "AGENTS.md")); err != nil {
		t.Fatalf("expected agents-md installed under $HOME/.codex: %v", err)
	}
}

func TestSkill_LocalScope_OutsideRepo_Fails(t *testing.T) {
	dir := realPath(t, t.TempDir())
	h := newHarness(t, dir)

	r := h.run("skill")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "requires running inside a git repository")
}
