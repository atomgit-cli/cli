package larkcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The old behavior under test (issue 675, private): larkcli's configDir fell
// back to a literal "~" — a relative path Go does not expand — so writes
// created a "~/.config/gc" tree in the current directory, same as pkg/config.
func TestLarkConfigPathEmptyWhenDirUnresolvable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("GC_CONFIG_DIR", "")

	if got := ConfigPath(); got != "" {
		t.Fatalf("ConfigPath() = %q, want \"\" when no config location can be resolved", got)
	}
}

func TestLarkReadsEmptyAndIgnoresLegacyCwdWhenDirUnresolvable(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("GC_CONFIG_DIR", "")
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Legacy junk from the old bug: a lark.json inside a literal "~" tree in
	// the current directory.
	legacy := filepath.Join(dir, "~", ".config", "gc")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "lark.json"), []byte(`{"default_chat_id":"legacy-junk"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := DefaultChatID(); got != "" {
		t.Fatalf("DefaultChatID() = %q, want empty — the cwd copy must be ignored", got)
	}
}

func TestLarkWritesFailClosedWithoutHomeNoCwdWrite(t *testing.T) {
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

	if err := SaveDefaultChat("chat-123"); err == nil {
		t.Fatal("SaveDefaultChat() error = nil with an unresolvable config dir")
	} else if !strings.Contains(err.Error(), "GC_CONFIG_DIR") {
		t.Fatalf("SaveDefaultChat() error = %q, want guidance mentioning GC_CONFIG_DIR", err)
	}

	if err := ClearDefaultChat(); err == nil {
		t.Fatal("ClearDefaultChat() error = nil with an unresolvable config dir")
	} else if !strings.Contains(err.Error(), "GC_CONFIG_DIR") {
		t.Fatalf("ClearDefaultChat() error = %q, want guidance mentioning GC_CONFIG_DIR", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cwd polluted with %d entries: %v", len(entries), entries)
	}
}
