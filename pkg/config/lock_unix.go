//go:build !windows

package config

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// lockFile takes an exclusive advisory lock with a bounded wait: LOCK_NB
// polling with backoff up to lockTimeout, then the last error is returned
// so lockConfigDir can fail loudly. flock releases on close and on process
// death; two handles in one process (separate open file descriptions) still
// exclude each other, so in-process concurrency is serialized too.
func lockFile(f *os.File) error {
	deadline := time.Now().Add(lockTimeout)
	delay := 200 * time.Microsecond
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EINTR) && time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay *= 2; delay > 25*time.Millisecond {
			delay = 25 * time.Millisecond
		}
	}
}
