package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The old behavior under test (issue 663): secureWriteFile wrote in place
// with O_TRUNC, so a reader that opened the file between the truncate and
// the completed write saw an empty or partial file — a torn read. The
// live-command reproduction on origin/main measured 3,414 failed parses in
// 472,798 reads (40s of concurrent 100 KiB rewrites).
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
	var readers sync.WaitGroup
	const numReaders = 4
	parseFailures := make([]int, numReaders)
	for i := 0; i < numReaders; i++ {
		readers.Add(1)
		go func(idx int) {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				data, err := os.ReadFile(path)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						continue
					}
					parseFailures[idx]++
					continue
				}
				var st struct {
					V string `json:"v"`
				}
				if err := json.Unmarshal(data, &st); err != nil {
					parseFailures[idx]++ // torn read: empty or partial file
				}
			}
		}(i)
	}

	var writers sync.WaitGroup
	var writeFailures int
	var mu sync.Mutex
	for w := 0; w < 2; w++ {
		writers.Add(1)
		go func(seed int) {
			defer writers.Done()
			for i := 0; i < 300; i++ {
				var err error
				if (i+seed)%2 == 0 {
					err = secureWriteFile(path, payload('a'), 0o600)
				} else {
					err = secureWriteFile(path, payload('b'), 0o600)
				}
				if err != nil {
					mu.Lock()
					writeFailures++
					mu.Unlock()
				}
			}
		}(w)
	}
	writers.Wait()
	close(stop)
	readers.Wait()

	total := 0
	for _, f := range parseFailures {
		total += f
	}
	if total != 0 {
		t.Fatalf("%d torn/parse-failed reads observed during concurrent rewrites — the write is not atomic", total)
	}
	if writeFailures != 0 {
		t.Fatalf("%d concurrent writes failed", writeFailures)
	}
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
