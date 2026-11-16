package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// lockTimeout bounds how long acquiring the config-dir write lock waits
// before failing loudly: a live gc process holding the lock longer than
// this is stuck, and waiting forever would wedge every other invocation.
// Tests override it.
var lockTimeout = 10 * time.Second

// tempSweepAge is the age gate for the stale .gc-tmp-* sweep at lock
// acquisition: an in-flight temp lives milliseconds, so anything this old
// is a crash or failed-cleanup leftover. The gate also protects
// version-skew writers that predate the lock (an old binary writing
// lock-free while a new binary sweeps).
var tempSweepAge = 60 * time.Second

// lockFileName is the cross-process write lock inside the config dir.
const lockFileName = ".gc-lock"

// lockConfigDir acquires the directory-level cross-process write lock for
// the gc config directory and returns the release function. The lock file
// is created if missing (0600); MkdirAll + Chmod(0700) happen here, so
// every read-modify-write path passes through one dir-ensuring entry point
// (issue 672; the state writers keep their own idempotent MkdirAll copies
// as defense in depth). After acquiring, stale .gc-tmp-* leftovers older
// than tempSweepAge are removed. Closing the lock file releases the
// platform lock; process death also releases it — a crashed holder cannot
// deadlock later runs.
func lockConfigDir(dir string) (release func(), err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, lockFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %s: %w", path, err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("could not acquire the config lock at %s within %s (another gc process may be stuck holding it — the lock releases when that process exits, so find and finish or kill it): %w", path, lockTimeout, err)
	}
	sweepStaleTemps(dir)
	return func() { _ = f.Close() }, nil
}

// LockConfigDir acquires the directory-level cross-process write lock for
// dir and returns the release function. It blocks up to ~10 seconds
// (lockTimeout), then fails loudly with an error naming the lock path.
// Side effects: it creates dir (0700, MkdirAll + Chmod) if missing, creates
// the .gc-lock file (0600) inside it, and removes .gc-tmp-* entries older
// than ~60 seconds in that directory (stale crash/cleanup residue). Release
// closes the lock file, which drops the lock — calling release more than
// once is safe; process death also releases. An empty dir fails with the
// raw MkdirAll error, so callers must resolve the config directory first
// (ResolveConfigDir / the per-method unresolvable checks).
//
// Issue 672: one lock guards all three files (auth.json, config.json,
// lark.json) — they share the directory, the resolution sentinel, and the
// write funnel, so sibling read-modify-write paths (pkg/larkcli) must not
// race outside the lock. Acquire the lock only AFTER the unresolvable-dir
// outcome is decided: the lock file lives in the config dir, and acquiring
// it first would mask ErrNoConfigDir with a lock-creation error.
func LockConfigDir(dir string) (release func(), err error) {
	return lockConfigDir(dir)
}

// sweepStaleTemps removes .gc-tmp-* entries older than tempSweepAge.
// Prefix-matched only (never .nfs* or anything else); failures are
// best-effort per the recorded residual.
func sweepStaleTemps(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-tempSweepAge)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, ".gc-tmp-") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}
