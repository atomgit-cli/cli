//go:build windows

package config

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive lock via LockFileEx with
// LOCKFILE_FAIL_IMMEDIATELY and bounded polling up to lockTimeout.
// Byte-range locks are per handle (a second handle in the same process is
// excluded too); closing the handle releases, and process death releases.
// LockFileEx and the LOCKFILE_* constants live in golang.org/x/sys/windows
// — the standard library syscall package does not export them.
func lockFile(f *os.File) error {
	deadline := time.Now().Add(lockTimeout)
	delay := 200 * time.Microsecond
	for {
		var ol windows.Overlapped
		err := windows.LockFileEx(
			windows.Handle(f.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
			0, 1, 0, &ol,
		)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay *= 2; delay > 25*time.Millisecond {
			delay = 25 * time.Millisecond
		}
	}
}
