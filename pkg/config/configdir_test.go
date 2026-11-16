package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The old behavior under test (issue 675, private): with HOME unset and no
// GC_CONFIG_DIR, configDir fell back to a literal "~", which Go does not
// expand, so every write created a "~/.config/gc" tree in the current
// directory — auth.json (credentials) included.
func TestConfigDirCheckedFailsClosedWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "") // windows home directory
	t.Setenv("GC_CONFIG_DIR", "")

	dir, err := configDirChecked()
	if err == nil {
		t.Fatalf("configDirChecked() error = nil with no home; dir = %q", dir)
	}
	if dir != "" {
		t.Fatalf("configDirChecked() dir = %q, want empty on failure", dir)
	}
	for _, want := range []string{"HOME", "GC_CONFIG_DIR"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want guidance mentioning %s", err.Error(), want)
		}
	}
}

func TestConfigDirCheckedResolvesUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GC_CONFIG_DIR", "")

	dir, err := configDirChecked()
	if err != nil {
		t.Fatalf("configDirChecked() error = %v with HOME set", err)
	}
	if want := filepath.Join(home, ".config", "gc"); dir != want {
		t.Fatalf("configDirChecked() = %q, want %q", dir, want)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("configDirChecked() = %q, want an absolute path", dir)
	}
}

func TestConfigDirCheckedEnvOverrideWinsWithoutHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("GC_CONFIG_DIR", dir)

	got, err := configDirChecked()
	if err != nil {
		t.Fatalf("configDirChecked() error = %v with GC_CONFIG_DIR set", err)
	}
	if got != dir {
		t.Fatalf("configDirChecked() = %q, want the GC_CONFIG_DIR value %q", got, dir)
	}
}

// Reads in an unresolvable environment must behave as an empty state and
// must not trust a legacy "~" tree sitting in the current directory (the
// residue the old bug left behind).
func TestConfigReadsEmptyAndIgnoresLegacyCwdWhenDirUnresolvable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("GC_CONFIG_DIR", "")
	t.Setenv("GC_BROWSER", "") // Get consults the env override first

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Legacy junk from the old bug: a literal "~" tree with a populated
	// auth.json in the current directory.
	legacy := filepath.Join(dir, "~", ".config", "gc")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyAuth := `{"version":1,"hosts":{"gitcode.com":{"active_user":"ghost","users":{"ghost":{"token":"legacy-junk"}}}}}`
	if err := os.WriteFile(filepath.Join(legacy, "auth.json"), []byte(legacyAuth), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := New()
	auth := cfg.Authentication()
	if user, token := auth.StoredCredential("gitcode.com"); user != "" || token != "" {
		t.Fatalf("StoredCredential() = (%q, %q), want empty — the cwd copy must be ignored", user, token)
	}
	if v, err := cfg.Get("gitcode.com", "browser"); err != nil || v != "" {
		t.Fatalf("Get() = (%q, %v), want empty without error", v, err)
	}
}

// Writes in an unresolvable environment must fail with guidance and must
// not create anything in the current directory.
func TestConfigWritesFailClosedWithoutHomeNoCwdWrite(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("GC_CONFIG_DIR", "")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	cfg := New()
	auth := cfg.Authentication()

	if err := cfg.Set("gitcode.com", "browser", "lynx"); err == nil {
		t.Fatal("Set() error = nil with an unresolvable config dir")
	} else if !strings.Contains(err.Error(), "GC_CONFIG_DIR") {
		t.Fatalf("Set() error = %q, want guidance mentioning GC_CONFIG_DIR", err)
	} else if !errors.Is(err, ErrNoConfigDir) {
		t.Fatalf("Set() error = %v, want errors.Is(err, ErrNoConfigDir)", err)
	}

	// Login reports the would-be change alongside the error; the error is
	// the contract that matters here (nothing was persisted).
	_, err = auth.Login("gitcode.com", "ghost", "token", "https", false)
	if err == nil {
		t.Fatal("Login() error = nil with an unresolvable config dir")
	} else if !strings.Contains(err.Error(), "GC_CONFIG_DIR") {
		t.Fatalf("Login() error = %q, want guidance mentioning GC_CONFIG_DIR", err)
	} else if !errors.Is(err, ErrNoConfigDir) {
		t.Fatalf("Login() error = %v, want errors.Is(err, ErrNoConfigDir)", err)
	}

	if err := cfg.Write(); err == nil {
		t.Fatal("Write() error = nil with an unresolvable config dir")
	} else if !strings.Contains(err.Error(), "GC_CONFIG_DIR") {
		t.Fatalf("Write() error = %q, want guidance mentioning GC_CONFIG_DIR", err)
	} else if !errors.Is(err, ErrNoConfigDir) {
		t.Fatalf("Write() error = %v, want errors.Is(err, ErrNoConfigDir)", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cwd polluted with %d entries: %v", len(entries), entries)
	}
}
