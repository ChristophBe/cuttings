/*
Copyright © 2026 Christoph Becker
*/
package shell_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestRun_Success(t *testing.T) {
	s := shell.NewSpawner()
	dir := t.TempDir()

	if err := s.Run(dir, "test-branch", []string{"true"}); err != nil {
		t.Errorf("Run() unexpected error: %v", err)
	}
}

func TestRun_ExitError(t *testing.T) {
	s := shell.NewSpawner()
	dir := t.TempDir()

	err := s.Run(dir, "test-branch", []string{"false"})
	if err == nil {
		t.Fatal("Run() expected error for failing command, got nil")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error type = %T, want *exec.ExitError", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("ExitCode() = %d, want 1", exitErr.ExitCode())
	}
}

func TestRun_ExitCode(t *testing.T) {
	s := shell.NewSpawner()
	dir := t.TempDir()

	err := s.Run(dir, "test-branch", []string{"sh", "-c", "exit 42"})

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error type = %T, want *exec.ExitError", err)
	}
	if exitErr.ExitCode() != 42 {
		t.Errorf("ExitCode() = %d, want 42", exitErr.ExitCode())
	}
}

func TestRun_WorkingDirectory(t *testing.T) {
	s := shell.NewSpawner()
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "was-here.txt")

	// Write a file in the working directory to prove the command ran there.
	if err := s.Run(dir, "test-branch", []string{"sh", "-c", "touch was-here.txt"}); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("sentinel file not found in working dir %q: %v", dir, err)
	}
}

func TestRun_EnvVarsInjected(t *testing.T) {
	s := shell.NewSpawner()
	dir := t.TempDir()
	out := filepath.Join(dir, "branch.txt")

	if err := s.Run(dir, "my-branch", []string{"sh", "-c", "printf '%s' \"$WORKSTREAM_BRANCH\" > branch.txt"}); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	//nolint:gosec // out is a path within t.TempDir(), not user-controlled input.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "my-branch" {
		t.Errorf("WORKSTREAM_BRANCH = %q, want %q", string(data), "my-branch")
	}
}

func TestRun_CommandWithArgs(t *testing.T) {
	s := shell.NewSpawner()
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")

	if err := s.Run(dir, "test-branch", []string{"sh", "-c", "printf '%s' \"$1\" > args.txt", "--", "hello"}); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	//nolint:gosec // out is a path within t.TempDir(), not user-controlled input.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("args output = %q, want %q", string(data), "hello")
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
