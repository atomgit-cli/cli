//go:build windows

package config

import (
	"os"
	"syscall"
	"time"
)

// lockFile takes an exclusive lock via LockFileEx with
// LOCKFILE_FAIL_IMMEDIATELY and bounded polling up to lockTimeout.
// Byte-range locks are per handle (a second handle in the same process is
// excluded too); closing the handle releases, and process death releases.
func lockFile(f *os.File) error {
	deadline := time.Now().Add(lockTimeout)
	delay := 200 * time.Microsecond
	for {
		var ol syscall.Overlapped
		err := syscall.LockFileEx(
			syscall.Handle(f.Fd()),
			syscall.LOCKFILE_EXCLUSIVE_LOCK|syscall.LOCKFILE_FAIL_IMMEDIATELY,
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
