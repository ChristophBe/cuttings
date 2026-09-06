/*
Copyright © 2026 Christoph Becker
*/

// Package runlock records in-progress `cuttings run` invocations on disk so a
// later invocation can detect and clean up a worktree left behind by a parent
// process that died uncatchably (SIGKILL, crash, `kill -9`) before its own
// deferred cleanup could run.
//
// A Store is pure filesystem state keyed by a run key; it knows nothing about
// git. Removing an orphan's worktree is the caller's job, passed to Sweep as a
// function.
//
// Usage:
//
//	s := runlock.NewStore(manager.GitCommonDir)
//	if err := s.Acquire(key, path); err != nil { ... }
//	defer s.Release(key)
package runlock

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Lock is the on-disk record of one in-progress run.
type Lock struct {
	Key       string    `json:"key"`
	Path      string    `json:"path"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store persists Locks as JSON files in one per-repository directory.
type Store struct {
	// gitCommonDir resolves the repository's shared .git directory. It is a
	// function, not a string, so that constructing a Store never fails or
	// shells out: the lookup happens on first use, and a repository that
	// cannot be resolved only affects the lock bookkeeping rather than every
	// command.
	gitCommonDir func() (string, error)
	// alive reports whether a PID is still running. A field so tests can
	// simulate live and dead owners without spawning real processes.
	alive func(pid int) bool
}

// NewStore returns a Store keeping its lock files under the "cuttings/run-locks"
// subdirectory of the directory gitCommonDir reports. The directory is created
// on first Acquire.
func NewStore(gitCommonDir func() (string, error)) *Store {
	return &Store{gitCommonDir: gitCommonDir, alive: processAlive}
}

// dir returns the directory holding this repository's lock files.
func (s *Store) dir() (string, error) {
	base, err := s.gitCommonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "cuttings", "run-locks"), nil
}

// fileName derives a filesystem-safe, deterministic file name for key (which
// may contain "/" for branch names). The real key is stored in the lock file's
// JSON body.
func fileName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x.json", sum[:8])
}

// Acquire records that a run for key (a worktree key, e.g. "cut-run-<ts>" or a
// branch name) is in progress at path, owned by the current process. It must be
// paired with a later Release call. Acquire is best-effort infrastructure for
// orphan detection: a caller should log a failure here rather than abort the
// run, since the worst case is that Sweep simply can't find this run later.
func (s *Store) Acquire(key, path string) error {
	dir, err := s.dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // 0750: group-readable, matches worktree dirs.
		return fmt.Errorf("create run-locks directory: %w", err)
	}

	data, marshalErr := json.MarshalIndent(Lock{
		Key:       key,
		Path:      path,
		PID:       os.Getpid(),
		CreatedAt: time.Now(),
	}, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("marshal run lock: %w", marshalErr)
	}

	if err := os.WriteFile(filepath.Join(dir, fileName(key)), data, 0o600); err != nil {
		return fmt.Errorf("write run lock: %w", err)
	}
	return nil
}

// Release removes the lock file previously written by Acquire for key. A
// missing lock file is not an error.
func (s *Store) Release(key string) error {
	dir, err := s.dir()
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, fileName(key))); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove run lock: %w", err)
	}
	return nil
}

// Sweep finds locks left behind by processes that died without running their
// own cleanup (e.g. killed with SIGKILL, or crashed) — the owning PID is no
// longer alive. For each such orphan it calls remove with the lock's key and,
// if that succeeds, deletes the stale lock file; it returns the keys it cleaned
// up. A lock whose recorded PID is still alive is left untouched. Errors from
// an individual orphan are collected and returned via errors.Join without
// stopping the sweep of the remaining entries; affected lock files are left in
// place so a later sweep can retry them.
func (s *Store) Sweep(remove func(key string) error) ([]string, error) {
	dir, err := s.dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read run-locks directory: %w", err)
	}

	var cleaned []string
	var errs []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lockPath := filepath.Join(dir, entry.Name())

		//nolint:gosec // path built from a directory entry we just listed.
		data, err := os.ReadFile(lockPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("read lock %s: %w", entry.Name(), err))
			continue
		}
		var lock Lock
		if err := json.Unmarshal(data, &lock); err != nil {
			// Corrupt/foreign file — remove it so it doesn't wedge future sweeps.
			_ = os.Remove(lockPath)
			continue
		}
		if s.alive(lock.PID) {
			continue
		}

		if removeErr := remove(lock.Key); removeErr != nil {
			errs = append(errs, fmt.Errorf("remove orphaned cutting %q: %w", lock.Key, removeErr))
			continue
		}
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove stale lock for %q: %w", lock.Key, err))
			continue
		}
		cleaned = append(cleaned, lock.Key)
	}
	return cleaned, errors.Join(errs...)
}

// processAlive reports whether a process with the given PID currently exists.
// Unix-only (this codebase already assumes Unix via syscall.Exec in package
// shell). Sending signal 0 performs no action but still returns an error
// (typically ESRCH) if the process does not exist.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid) // on Unix this always succeeds
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
