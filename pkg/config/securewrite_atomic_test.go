package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// The old behavior under test (issue 663): secureWriteFile wrote in place
// with O_TRUNC, so a reader that opened the file between the truncate and
// the completed write saw an empty or partial file — a torn read. The
// live-command reproduction on origin/main measured 3,414 failed parses in
// 472,798 reads (40s of concurrent 100 KiB rewrites); the in-package red
// state measured 11,846 torn reads in 0.13s (an independent reviewer
// measured 28,715 on the same base).
func TestSecureWriteFileAtomicUnderConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	payload := func(fill byte) []byte {
		body := strings.Repeat(string(fill), 1<<17) // 128 KiB widens the torn window
		return []byte(`{"v":"` + body + `"}`)
	}
	if err := secureWriteFile(path, payload('a'), 0o600); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	stop := make(chan struct{})
	const numReaders = 4
	const numWriters = 2
	const writesPerWriter = 300
	totalWrites := int64(numWriters * writesPerWriter)
	var readers sync.WaitGroup
	var tornReads, readErrors, writeErrors int64
	for i := 0; i < numReaders; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				data, err := os.ReadFile(path)
				if err != nil {
					// A read error means the entry could not be opened as a
					// complete file. On Windows a transient sharing violation
					// during the rename replacement is platform noise, not a
					// torn read (MoveFileEx semantics; issue 672 will
					// serialize writers). On Unix any read error — including
					// a missing entry — would be a regression: rename
					// replaces the entry atomically and never removes it.
					atomic.AddInt64(&readErrors, 1)
					continue
				}
				var st struct {
					V string `json:"v"`
				}
				if err := json.Unmarshal(data, &st); err != nil {
					// Torn read: successfully read data that is empty or
					// partial. This is the atomicity invariant and must hold
					// on every platform.
					atomic.AddInt64(&tornReads, 1)
				}
			}
		}()
	}

	var writers sync.WaitGroup
	for w := 0; w < numWriters; w++ {
		writers.Add(1)
		go func(seed int) {
			defer writers.Done()
			for i := 0; i < writesPerWriter; i++ {
				var err error
				if (i+seed)%2 == 0 {
					err = secureWriteFile(path, payload('a'), 0o600)
				} else {
					err = secureWriteFile(path, payload('b'), 0o600)
				}
				if err != nil {
					atomic.AddInt64(&writeErrors, 1)
				}
			}
		}(w)
	}
	writers.Wait()
	close(stop)
	readers.Wait()

	// The atomicity invariant: data that was successfully read always parses
	// into one of the two complete payloads. Strict on every platform.
	if tornReads != 0 {
		t.Fatalf("%d torn reads observed during concurrent rewrites — the write is not atomic", tornReads)
	}
	if runtime.GOOS != "windows" {
		// On Unix, rename replaces the directory entry atomically and never
		// removes it, and two writers' renames onto one target cannot fail:
		// any read error or write failure is a regression.
		if readErrors != 0 || writeErrors != 0 {
			t.Fatalf("read errors = %d, write failures = %d — both must be zero on Unix", readErrors, writeErrors)
		}
	} else {
		// Windows: readers of this test open the target without
		// FILE_SHARE_DELETE, so each open handle blocks the writers'
		// replacing renames; with four tight-loop readers the failure rate
		// depends on their handle duty cycle, which cannot be measured from
		// Linux. A bounded assertion (at least one write must succeed) was
		// proposed and rejected in review: its P(all fail)≈0 argument
		// assumed a low duty cycle, and a wrong guess reddens the
		// post-merge mirror CI — the exact incident shape this PR avoids.
		// Total breakage is still caught by the seed write above. Revisit
		// with issue 672 (retry + real Windows baseline data).
		t.Logf("windows: %d read errors / %d of %d writes failed from sharing during concurrent MoveFileEx replacement (platform noise; issue 672 serializes writers)", readErrors, writeErrors, totalWrites)
	}

	// The final content must be one of the two complete payloads (guards
	// against a no-op write regression).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("final read failed: %v", err)
	}
	var st struct {
		V string `json:"v"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("final content does not parse: %v", err)
	}
	if len(st.V) != 1<<17 || (st.V[0] != 'a' && st.V[0] != 'b') {
		t.Fatalf("final content is not one of the two payloads (len=%d, first=%q)", len(st.V), firstByte(st.V))
	}

	// No temp residue after the concurrent run: the directory holds exactly
	// the target file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "auth.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("temp residue left behind after concurrent writes: %v", names)
	}
}

func firstByte(s string) string {
	if len(s) == 0 {
		return ""
	}
	return s[:1]
}

// A successful atomic write must leave nothing behind in the target
// directory besides the target file itself.
func TestSecureWriteFileNoTempResidueOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := secureWriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("secureWriteFile() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("temp residue left behind: %v", names)
	}
}

// A failed write (rename cannot land the payload) must clean up its temp
// file: a leftover .gc-tmp-* file would hold the full credential payload.
func TestSecureWriteFileCleansTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "occupied")
	// A non-empty directory at the target path makes the final rename fail
	// deterministically on every platform.
	if err := os.MkdirAll(filepath.Join(path, "child"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := secureWriteFile(path, []byte(`{"token":"payload"}`), 0o600); err == nil {
		t.Fatal("secureWriteFile() error = nil, want rename failure onto a non-empty directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gc-tmp-") {
			t.Fatalf("temp file %q left behind after a failed write", e.Name())
		}
	}
}
