package installupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The old behavior under test (issue 671): with HOME unset and no
// XDG_STATE_HOME/GC_STATE_DIR, stateRoot returned a RELATIVE path
// (.local/state/gitcode-cli), so acquireStateLock's MkdirAll created
// the tree in whatever directory the command ran in.
func TestStateRootAbsoluteWhenXDGSet(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("GC_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if runtime.GOOS == "windows" {
		t.Skip("windows uses LOCALAPPDATA; the Unix fail-closed path is not exercised here")
	}

	root, err := stateRootChecked()
	if err != nil {
		t.Fatalf("stateRootChecked() error = %v with XDG set", err)
	}
	if !filepath.IsAbs(root) {
		t.Fatalf("stateRoot() = %q, want an absolute path (no silent cwd writes)", root)
	}
}

func TestStateRootFailsClosedWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("GC_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	if runtime.GOOS == "windows" {
		t.Skip("windows uses LOCALAPPDATA; the Unix fail-closed path is not exercised here")
	}

	_, err := stateRootChecked()
	if err == nil {
		t.Fatal("stateRootChecked() error = nil, want an error when HOME is unset")
	}
	for _, want := range []string{"HOME", "XDG_STATE_HOME", "GC_STATE_DIR"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want guidance mentioning %s", err.Error(), want)
		}
	}
}

func TestStatePathEmptyWhenUnresolvable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("GC_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	if runtime.GOOS == "windows" {
		t.Skip("windows uses LOCALAPPDATA; the Unix fail-closed path is not exercised here")
	}
	if got := StatePath(nil); got != "" {
		t.Fatalf("StatePath(nil) = %q, want \"\" (no cwd-relative path)", got)
	}
}

func TestMutateStateLockedEmptyPathSilentNoCwdWrite(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("GC_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	if runtime.GOOS == "windows" {
		t.Skip("windows uses LOCALAPPDATA; the Unix fail-closed path is not exercised here")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	called := false
	ok := mutateStateLocked("", func(*updateState) { called = true })
	if ok {
		t.Fatal("mutateStateLocked(\"\") = true, want false (fully silent degradation)")
	}
	if called {
		t.Fatal("mutate callback ran for an empty path; want it skipped entirely")
	}
	// Nothing may appear in the current directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cwd polluted with %d entries: %v", len(entries), entries)
	}
}
