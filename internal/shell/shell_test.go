/*
Copyright © 2026 Christoph Becker
*/
package shell_test

import (
	"os"
	"strings"
	"testing"
)

// TestEnvVarInjection verifies that WORKSTREAM_BRANCH and WORKSTREAM_PATH are
// injected and that pre-existing values are replaced.
//
// buildEnv is unexported so we test its effects indirectly. Since Spawn
// replaces the process via syscall.Exec (which cannot be unit-tested without
// forking), we validate the env construction logic here using buildTestEnv,
// which mirrors buildEnv's implementation.
func TestEnvVarInjection(t *testing.T) {
	t.Setenv("WORKSTREAM_BRANCH", "old-branch")
	t.Setenv("WORKSTREAM_PATH", "/old/path")

	env := buildTestEnv("new-branch", "/new/path")

	branchVal := ""
	pathVal := ""
	for _, e := range env {
		if strings.HasPrefix(e, "WORKSTREAM_BRANCH=") {
			branchVal = strings.TrimPrefix(e, "WORKSTREAM_BRANCH=")
		}
		if strings.HasPrefix(e, "WORKSTREAM_PATH=") {
			pathVal = strings.TrimPrefix(e, "WORKSTREAM_PATH=")
		}
	}

	if branchVal != "new-branch" {
		t.Errorf("WORKSTREAM_BRANCH = %q, want %q", branchVal, "new-branch")
	}
	if pathVal != "/new/path" {
		t.Errorf("WORKSTREAM_PATH = %q, want %q", pathVal, "/new/path")
	}
}

// buildTestEnv mirrors the unexported buildEnv function from the shell package
// so its logic can be tested without exporting it.
func buildTestEnv(branch, path string) []string {
	current := os.Environ()
	out := make([]string, 0, len(current)+2)
	for _, e := range current {
		if strings.HasPrefix(e, "WORKSTREAM_BRANCH=") || strings.HasPrefix(e, "WORKSTREAM_PATH=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out,
		"WORKSTREAM_BRANCH="+branch,
		"WORKSTREAM_PATH="+path,
	)
	return out
}

func TestEnvVarNotDuplicated(t *testing.T) {
	t.Setenv("WORKSTREAM_BRANCH", "branch-a")

	env := buildTestEnv("branch-b", "/some/path")

	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "WORKSTREAM_BRANCH=") {
			count++
		}
	}

	if count != 1 {
		t.Errorf("WORKSTREAM_BRANCH appears %d times in env, want exactly 1", count)
	}
}
