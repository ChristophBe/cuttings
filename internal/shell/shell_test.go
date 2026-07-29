/*
Copyright © 2026 Christoph Becker
*/
package shell_test

import (
	"strings"
	"testing"

	"github.com/ChristophBe/workstreams/internal/shell"
)

// TestEnvVarInjection verifies that WORKSTREAM_BRANCH and WORKSTREAM_PATH are
// injected and that pre-existing values are replaced.
//
// Spawn replaces the process via syscall.Exec and cannot be unit-tested without
// forking, so we validate the env-construction logic via the exported BuildEnv.
func TestEnvVarInjection(t *testing.T) {
	t.Setenv("WORKSTREAM_BRANCH", "old-branch")
	t.Setenv("WORKSTREAM_PATH", "/old/path")

	env := shell.BuildEnv("new-branch", "/new/path")

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

func TestEnvVarNotDuplicated(t *testing.T) {
	t.Setenv("WORKSTREAM_BRANCH", "branch-a")

	env := shell.BuildEnv("branch-b", "/some/path")

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
