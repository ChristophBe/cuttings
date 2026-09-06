/*
Copyright © 2026 Christoph Becker
*/

// The tests live in package runlock (not runlock_test) so they can replace the
// process-liveness check, which is what makes orphan detection testable without
// spawning real processes and waiting for them to die.
package runlock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestStore returns a Store rooted at a temporary "git common dir",
// together with the directory its lock files land in.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	base := t.TempDir()
	s := NewStore(func() (string, error) { return base, nil })
	return s, filepath.Join(base, "cuttings", "run-locks")
}

// writeRawLock writes a lock file directly (bypassing Acquire, which always
// records the current process's own PID) so tests can simulate a lock left
// behind by a different, possibly-dead process.
func writeRawLock(t *testing.T, dir, name string, lock map[string]any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
}

func entryCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	return len(entries)
}

func TestAcquire_WritesLockFile(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)

	if err := s.Acquire("some-key", "/repo/.worktrees/some-key"); err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(run-locks): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("run-locks dir has %d entries, want 1", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name())) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatalf("unmarshal lock file: %v", err)
	}
	if lock.Key != "some-key" {
		t.Errorf("lock.Key = %q, want %q", lock.Key, "some-key")
	}
	if lock.PID != os.Getpid() {
		t.Errorf("lock.PID = %d, want %d (current process)", lock.PID, os.Getpid())
	}
	if lock.Path != "/repo/.worktrees/some-key" {
		t.Errorf("lock.Path = %q, want the worktree path", lock.Path)
	}
}

// A branch name containing "/" must not create nested directories: the file
// name is a hash, and the real key lives in the JSON body.
func TestAcquire_BranchNameWithSlash(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)

	if err := s.Acquire("feature/foo", "/repo/.worktrees/feature/foo"); err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(run-locks): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("run-locks dir has %d entries, want 1", len(entries))
	}
	if entries[0].IsDir() {
		t.Error("lock file name created a directory; it should be a flat hashed name")
	}
	if err := s.Release("feature/foo"); err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}
	if n := entryCount(t, dir); n != 0 {
		t.Errorf("run-locks dir has %d entries after Release(), want 0", n)
	}
}

func TestRelease_RemovesLockFile(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)

	if err := s.Acquire("some-key", "/tmp/ws"); err != nil {
		t.Fatalf("Acquire() setup: %v", err)
	}
	if err := s.Release("some-key"); err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}
	if n := entryCount(t, dir); n != 0 {
		t.Errorf("run-locks dir has %d entries after Release(), want 0", n)
	}
}

func TestRelease_NonexistentKey_NotAnError(t *testing.T) {
	t.Parallel()

	s, _ := newTestStore(t)
	if err := s.Release("never-locked"); err != nil {
		t.Errorf("Release() unexpected error for a key that was never locked: %v", err)
	}
}

func TestSweep_NoLocks_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	s, _ := newTestStore(t)

	cleaned, err := s.Sweep(func(string) error {
		t.Error("remove should not be called when there are no locks")
		return nil
	})
	if err != nil {
		t.Fatalf("Sweep() unexpected error: %v", err)
	}
	if len(cleaned) != 0 {
		t.Errorf("Sweep() = %v, want empty", cleaned)
	}
}

func TestSweep_LivePID_LeavesLockAlone(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)
	s.alive = func(int) bool { return true }

	writeRawLock(t, dir, "live.json", map[string]any{
		"key": "still-running", "path": "/tmp/ws", "pid": 4242, "createdAt": time.Now(),
	})

	cleaned, err := s.Sweep(func(string) error {
		t.Error("remove should not be called for a lock whose owner is alive")
		return nil
	})
	if err != nil {
		t.Fatalf("Sweep() unexpected error: %v", err)
	}
	if len(cleaned) != 0 {
		t.Errorf("Sweep() = %v, want empty", cleaned)
	}
	if n := entryCount(t, dir); n != 1 {
		t.Errorf("run-locks dir has %d entries, want the live lock left in place", n)
	}
}

func TestSweep_DeadPID_RemovesWorktreeAndLock(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)
	s.alive = func(int) bool { return false }

	writeRawLock(t, dir, "dead.json", map[string]any{
		"key": "to-sweep", "path": "/tmp/ws", "pid": 4242, "createdAt": time.Now(),
	})

	var removed []string
	cleaned, err := s.Sweep(func(key string) error {
		removed = append(removed, key)
		return nil
	})
	if err != nil {
		t.Fatalf("Sweep() unexpected error: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0] != "to-sweep" {
		t.Errorf("Sweep() = %v, want [to-sweep]", cleaned)
	}
	if len(removed) != 1 || removed[0] != "to-sweep" {
		t.Errorf("remove called with %v, want [to-sweep]", removed)
	}
	if n := entryCount(t, dir); n != 0 {
		t.Errorf("run-locks dir has %d entries after sweep, want 0", n)
	}
}

// A failure removing one orphan must not abort the sweep, and its lock file
// must survive so a later sweep can retry it.
func TestSweep_RemoveFails_ContinuesAndKeepsLock(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)
	s.alive = func(int) bool { return false }

	for _, key := range []string{"bad", "good"} {
		writeRawLock(t, dir, key+".json", map[string]any{
			"key": key, "path": "/tmp/" + key, "pid": 4242, "createdAt": time.Now(),
		})
	}

	boom := errors.New("worktree is dirty")
	cleaned, err := s.Sweep(func(key string) error {
		if key == "bad" {
			return boom
		}
		return nil
	})

	if err == nil {
		t.Fatal("Sweep() error = nil, want the failing orphan reported")
	}
	if !errors.Is(err, boom) {
		t.Errorf("Sweep() error = %v, want it to wrap %v", err, boom)
	}
	if len(cleaned) != 1 || cleaned[0] != "good" {
		t.Errorf("Sweep() = %v, want the other orphan still cleaned", cleaned)
	}
	if n := entryCount(t, dir); n != 1 {
		t.Errorf("run-locks dir has %d entries, want the failed orphan's lock kept for retry", n)
	}
}

func TestSweep_CorruptLockFile_RemovedAndSkipped(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	garbage := filepath.Join(dir, "garbage.json")
	if err := os.WriteFile(garbage, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write garbage lock file: %v", err)
	}

	cleaned, err := s.Sweep(func(string) error {
		t.Error("remove should not be called for a corrupt lock file")
		return nil
	})
	if err != nil {
		t.Fatalf("Sweep() unexpected error: %v", err)
	}
	if len(cleaned) != 0 {
		t.Errorf("Sweep() = %v, want empty (corrupt file is not a valid orphan)", cleaned)
	}
	if _, err := os.Stat(garbage); !os.IsNotExist(err) {
		t.Error("corrupt lock file was not removed")
	}
}

// Foreign files in the directory must be left strictly alone.
func TestSweep_IgnoresNonJSONFiles(t *testing.T) {
	t.Parallel()

	s, dir := newTestStore(t)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	other := filepath.Join(dir, "README")
	if err := os.WriteFile(other, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := s.Sweep(func(string) error { return nil }); err != nil {
		t.Fatalf("Sweep() unexpected error: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a non-lock file was touched: %v", err)
	}
}

// Locks live under the repository's shared .git directory, so every worktree of
// a repository sees the same set.
func TestStore_LockDirectoryLayout(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	s := NewStore(func() (string, error) { return base, nil })

	if err := s.Acquire("k", "/tmp/ws"); err != nil {
		t.Fatalf("Acquire() unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "cuttings", "run-locks")); err != nil {
		t.Errorf("expected locks under <git-common-dir>/cuttings/run-locks: %v", err)
	}
}

// Resolving the repository can fail (e.g. outside a git repo). Every operation
// must surface that rather than panicking or silently using a wrong path.
func TestStore_DirResolutionError(t *testing.T) {
	t.Parallel()

	boom := errors.New("not a git repository")
	s := NewStore(func() (string, error) { return "", boom })

	if err := s.Acquire("k", "/tmp/ws"); !errors.Is(err, boom) {
		t.Errorf("Acquire() error = %v, want %v", err, boom)
	}
	if err := s.Release("k"); !errors.Is(err, boom) {
		t.Errorf("Release() error = %v, want %v", err, boom)
	}
	if _, err := s.Sweep(func(string) error { return nil }); !errors.Is(err, boom) {
		t.Errorf("Sweep() error = %v, want %v", err, boom)
	}
}

func TestProcessAlive(t *testing.T) {
	t.Parallel()

	if !processAlive(os.Getpid()) {
		t.Error("processAlive(own pid) = false, want true")
	}
	for _, pid := range []int{0, -1} {
		if processAlive(pid) {
			t.Errorf("processAlive(%d) = true, want false", pid)
		}
	}
}
