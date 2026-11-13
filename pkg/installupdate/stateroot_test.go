package installupdate

import (
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
		t.Skip("windows uses LOCALAPPDATA; the relative fallback is Unix-only")
	}

	root := stateRoot()
	if !filepath.IsAbs(root) {
		t.Fatalf("stateRoot() = %q, want an absolute path (no silent cwd writes)", root)
	}
}

func TestStateRootFailsClosedWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("GC_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	if runtime.GOOS == "windows" {
		t.Skip("windows uses LOCALAPPDATA; the relative fallback is Unix-only")
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
