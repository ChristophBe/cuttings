/*
Copyright © 2026 Christoph Becker
*/

// Package skill installs coding-agent instruction files (a Claude Code
// skill, a generic AGENTS.md section, Cursor rules, GitHub Copilot
// instructions) that teach an agent to use workstreams non-interactively
// for parallel, per-branch work. Content is embedded at build time so the
// command works regardless of how the workstreams binary was installed.
package skill

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed assets
var assets embed.FS

// Scope selects where a target's files are installed.
type Scope string

const (
	// ScopeLocal installs into the current repository.
	ScopeLocal Scope = "local"
	// ScopeGlobal installs into the user's home directory.
	ScopeGlobal Scope = "global"
)

// Outcome describes what happened to a single target during Install.
type Outcome string

// Outcome values reported on a Result.
const (
	OutcomeInstalled          Outcome = "installed"
	OutcomeSkippedExists      Outcome = "skipped-exists"
	OutcomeSkippedUnsupported Outcome = "skipped-unsupported"
)

// Result reports what Install did for a single target.
type Result struct {
	Target  string
	Path    string
	Outcome Outcome
	Reason  string
}

// String renders a Result as a single human-readable line.
func (r Result) String() string {
	switch r.Outcome {
	case OutcomeInstalled:
		return fmt.Sprintf("%s: installed at %s", r.Target, r.Path)
	case OutcomeSkippedExists:
		return fmt.Sprintf("%s: already exists at %s — use --overwrite to replace it", r.Target, r.Path)
	case OutcomeSkippedUnsupported:
		return fmt.Sprintf("%s: skipped — %s", r.Target, r.Reason)
	default:
		return fmt.Sprintf("%s: %s", r.Target, r.Outcome)
	}
}

// AllTargets are the recognized target names, in stable output order.
var AllTargets = []string{"claude", "agents-md", "cursor", "copilot"}

const (
	sectionStartMarker = "<!-- workstreams:skill:start -->"
	sectionEndMarker   = "<!-- workstreams:skill:end -->"
)

var sectionPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(sectionStartMarker) + `.*?` + regexp.QuoteMeta(sectionEndMarker))

// embedFile maps one embedded source file to a path relative to a target's
// install directory. go:embed does not preserve file mode, so scripts must
// be marked executable explicitly.
type embedFile struct {
	src        string
	destRel    string
	executable bool
}

// target describes one installable agent format.
//
// For whole-file targets, localPath/globalPath return the *directory* the
// files are copied into. For section-merge targets, they return the exact
// destination *file* whose marked section gets upserted.
type target struct {
	name                    string
	wholeFile               bool
	files                   []embedFile
	sectionSrc              string
	localPath               func(repoRoot string) string
	globalPath              func(home string) string
	unsupportedGlobalReason string
}

var targets = map[string]target{
	"claude": {
		name:      "claude",
		wholeFile: true,
		files: []embedFile{
			{src: "assets/claude/SKILL.md", destRel: "SKILL.md"},
			{src: "assets/claude/scripts/create-workstream.sh", destRel: "scripts/create-workstream.sh", executable: true},
		},
		localPath: func(repoRoot string) string {
			return filepath.Join(repoRoot, ".claude", "skills", "workstreams-parallel")
		},
		globalPath: func(home string) string { return filepath.Join(home, ".claude", "skills", "workstreams-parallel") },
	},
	"agents-md": {
		name:       "agents-md",
		wholeFile:  false,
		sectionSrc: "assets/agents-md/section.md",
		localPath:  func(repoRoot string) string { return filepath.Join(repoRoot, "AGENTS.md") },
		globalPath: func(home string) string { return filepath.Join(home, ".codex", "AGENTS.md") },
	},
	"cursor": {
		name:      "cursor",
		wholeFile: true,
		files: []embedFile{
			{src: "assets/cursor/workstreams-parallel.mdc", destRel: "workstreams-parallel.mdc"},
		},
		localPath:               func(repoRoot string) string { return filepath.Join(repoRoot, ".cursor", "rules") },
		unsupportedGlobalReason: "Cursor has no filesystem location for global rules (configured via the app's User Rules setting)",
	},
	"copilot": {
		name:                    "copilot",
		wholeFile:               false,
		sectionSrc:              "assets/copilot/section.md",
		localPath:               func(repoRoot string) string { return filepath.Join(repoRoot, ".github", "copilot-instructions.md") },
		unsupportedGlobalReason: "GitHub Copilot has no plain-file location for global custom instructions",
	},
}

// ParseScope validates a scope flag value.
func ParseScope(s string) (Scope, error) {
	switch Scope(s) {
	case ScopeLocal, ScopeGlobal:
		return Scope(s), nil
	default:
		return "", fmt.Errorf("unknown scope %q (valid: %s, %s)", s, ScopeLocal, ScopeGlobal)
	}
}

// resolveTargetNames expands "all" and validates/dedupes the requested
// target names, preserving first-seen order.
func resolveTargetNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return AllTargets, nil
	}

	seen := make(map[string]bool, len(names))
	result := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "all" {
			return AllTargets, nil
		}
		if _, ok := targets[n]; !ok {
			return nil, fmt.Errorf("unknown target %q (valid: %s, all)", n, strings.Join(AllTargets, ", "))
		}
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	return result, nil
}

// Install writes the requested targets' files for the given scope.
// repoRoot is required (and used) only for ScopeLocal; home only for
// ScopeGlobal. Targets with no meaningful location for the requested scope
// are reported with OutcomeSkippedUnsupported rather than erroring.
func Install(scope Scope, targetNames []string, repoRoot, home string, overwrite bool) ([]Result, error) {
	names, err := resolveTargetNames(targetNames)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(names))
	for _, name := range names {
		t := targets[name]

		dest, unsupportedReason, err := t.resolveDest(scope, repoRoot, home)
		if err != nil {
			return nil, fmt.Errorf("resolve destination for target %q: %w", name, err)
		}
		if unsupportedReason != "" {
			results = append(results, Result{Target: name, Outcome: OutcomeSkippedUnsupported, Reason: unsupportedReason})
			continue
		}

		var res Result
		if t.wholeFile {
			res, err = installWholeFile(t, dest, overwrite)
		} else {
			res, err = installSection(t, dest)
		}
		if err != nil {
			return nil, fmt.Errorf("install target %q: %w", name, err)
		}
		results = append(results, res)
	}
	return results, nil
}

func (t target) resolveDest(scope Scope, repoRoot, home string) (dest string, unsupportedReason string, err error) {
	switch scope {
	case ScopeLocal:
		if repoRoot == "" {
			return "", "", errors.New("repoRoot is required for local scope")
		}
		return t.localPath(repoRoot), "", nil
	case ScopeGlobal:
		if t.globalPath == nil {
			return "", t.unsupportedGlobalReason, nil
		}
		if home == "" {
			return "", "", errors.New("home is required for global scope")
		}
		return t.globalPath(home), "", nil
	default:
		return "", "", fmt.Errorf("unknown scope %q", scope)
	}
}

func installWholeFile(t target, destDir string, overwrite bool) (Result, error) {
	if !overwrite {
		for _, f := range t.files {
			if _, err := os.Stat(filepath.Join(destDir, f.destRel)); err == nil {
				return Result{Target: t.name, Path: destDir, Outcome: OutcomeSkippedExists}, nil
			}
		}
	}

	for _, f := range t.files {
		destPath := filepath.Join(destDir, f.destRel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil { //nolint:gosec // 0750: group-readable, matches internal/worktree's convention
			return Result{}, fmt.Errorf("create directory: %w", err)
		}
		data, err := assets.ReadFile(f.src)
		if err != nil {
			return Result{}, fmt.Errorf("read embedded asset %q: %w", f.src, err)
		}
		mode := os.FileMode(0o644)
		if f.executable {
			mode = 0o755
		}
		if err := os.WriteFile(destPath, data, mode); err != nil { //nolint:gosec // mode is fixed above, not user-controlled
			return Result{}, fmt.Errorf("write %s: %w", destPath, err)
		}
	}
	return Result{Target: t.name, Path: destDir, Outcome: OutcomeInstalled}, nil
}

func installSection(t target, destPath string) (Result, error) {
	section, err := assets.ReadFile(t.sectionSrc)
	if err != nil {
		return Result{}, fmt.Errorf("read embedded asset %q: %w", t.sectionSrc, err)
	}

	existing, err := os.ReadFile(destPath) //nolint:gosec // destPath is derived from repoRoot/home, not raw user input
	if err != nil && !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("read %s: %w", destPath, err)
	}

	updated := upsertSection(string(existing), string(section))

	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil { //nolint:gosec // 0750: group-readable, matches internal/worktree's convention
		return Result{}, fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(destPath, []byte(updated), 0o644); err != nil { //nolint:gosec // 0644 is appropriate for a committed instructions file
		return Result{}, fmt.Errorf("write %s: %w", destPath, err)
	}
	return Result{Target: t.name, Path: destPath, Outcome: OutcomeInstalled}, nil
}

// upsertSection replaces the marked section in existing with section (which
// itself includes the start/end markers), or appends it if the markers
// aren't present yet. Content outside the markers is left untouched, so
// this is safe to run repeatedly and coexists with unrelated file content.
func upsertSection(existing, section string) string {
	section = strings.TrimRight(section, "\n")

	if sectionPattern.MatchString(existing) {
		return sectionPattern.ReplaceAllLiteralString(existing, section)
	}

	trimmed := strings.TrimRight(existing, "\n")
	if trimmed == "" {
		return section + "\n"
	}
	return trimmed + "\n\n" + section + "\n"
}
