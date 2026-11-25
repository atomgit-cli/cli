package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// secureWriteFile writes data to path atomically with hardened permission
// semantics (issues 400 and 663).
//
// Atomicity: the payload goes to a freshly created temp file in the target's
// directory (os.CreateTemp: O_EXCL, random name, 0600 — immune to symlink
// planting), is synced, and is renamed over the target. rename never follows
// a final-component symlink — it replaces the directory entry — so data a
// reader successfully reads is always the complete old file or the complete
// new file, never the empty/partial file the previous O_TRUNC in-place
// write exposed mid-rewrite (issue 663; live repro: 3,414 failed parses in
// 472,798 reads on origin/main). On Windows a reader or external process
// holding the target open can transiently fail the replacing rename with a
// sharing violation — a loud, retryable error, never torn data (see the
// spec's credential-write constraints for the full contract).
//
// Permissions: the temp file is chmod'ed to perm before the rename, so the
// final file lands with the requested mode regardless of umask, and the
// hardening targets the exact inode that received the data.
//
// Symlinks: the Lstat refusal is advisory UX continuity from the O_NOFOLLOW
// era (issue 400): dotfiles-style symlink setups keep failing loudly
// instead of having their link silently replaced. It is not load-bearing
// for credential redirection — the worst case in the Lstat→rename gap is
// the link being atomically replaced, never written through. Threat model
// assumptions documented in .loop/deliveries/issue-400-design.md and
// re-reviewed for the rename path in a comment on issue 400:
// parent-directory symlink components redirect the file exactly as they
// did for the in-place write (unchanged, mitigated by the 0700 config
// dir); hard links are replaced rather than followed.
//
// Failure cleanup: a write that cannot land attempts to remove its temp
// file — a leftover would hold the full payload (credentials) at 0600 in
// the config directory, and removal itself can fail (e.g. an external
// process holding the temp open on Windows); both crash and failed-cleanup
// leftovers are covered by the spec's recorded residual.
func secureWriteFile(path string, data []byte, perm os.FileMode) error {
	if li, err := os.Lstat(path); err == nil && li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write to symlink: %s", path)
	}

	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".gc-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tmp := f.Name()

	landed := false
	defer func() {
		if !landed {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", tmp, err)
	}
	// Bounded retry: on Windows a reader or external process (antivirus,
	// indexer) holding the target open fails the replacing rename with a
	// sharing violation that usually clears within milliseconds; two quick
	// retries absorb it (issue 672). On Unix rename failures are
	// deterministic, and retrying only re-fails.
	const renameAttempts = 3
	var renameErr error
	for attempt := 0; attempt < renameAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 25 * time.Millisecond)
		}
		renameErr = os.Rename(tmp, path)
		if renameErr == nil {
			landed = true
			return nil
		}
	}
	// The message names only the target: the temp file is already gone
	// by the time this surfaces (cleaned up below), so naming it would
	// send anyone following the error on a hunt for a file that no
	// longer exists.
	return fmt.Errorf("failed to replace %s after %d attempts: %w", path, renameAttempts, renameErr)
}
