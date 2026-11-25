package config

import (
	"errors"
	"testing"
)

// Issue 677: command surfaces need to distinguish a broken environment
// from a genuinely empty one. ResolutionErr exposes the construction-time
// outcome: ErrNoConfigDir when the config directory could not be resolved,
// nil otherwise. Reads treat the unresolvable directory as an empty state
// (issue 675), so without this method "not logged in" is indistinguishable
// from "stored logins are invisible in this environment".
func TestConfigResolutionErr(t *testing.T) {
	t.Run("unresolvable", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv("USERPROFILE", "")
		t.Setenv("GC_CONFIG_DIR", "")

		err := New().ResolutionErr()
		if err == nil {
			t.Fatal("ResolutionErr() = nil with an unresolvable dir, want ErrNoConfigDir")
		}
		if !errors.Is(err, ErrNoConfigDir) {
			t.Fatalf("ResolutionErr() = %v, want errors.Is(err, ErrNoConfigDir)", err)
		}
	})
	t.Run("resolved via home", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("GC_CONFIG_DIR", "")

		if err := New().ResolutionErr(); err != nil {
			t.Fatalf("ResolutionErr() = %v with HOME set, want nil", err)
		}
	})
	t.Run("resolved via env override", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv("USERPROFILE", "")
		t.Setenv("GC_CONFIG_DIR", t.TempDir())

		if err := New().ResolutionErr(); err != nil {
			t.Fatalf("ResolutionErr() = %v with GC_CONFIG_DIR set, want nil", err)
		}
	})
}
