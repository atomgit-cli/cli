package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileGetSkipsEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"version":1,"hosts":{"gitcode.com":{"update.mode":"off"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GC_UPDATE_MODE", "banana")
	cfg, ok := New().(*config)
	if !ok {
		t.Fatal("New() must return the built-in config implementation")
	}
	if got, _ := cfg.FileGet("gitcode.com", "update.mode"); got != "off" {
		t.Fatalf("FileGet = %q, want the configured \"off\"", got)
	}
	if got, _ := cfg.Get("gitcode.com", "update.mode"); got != "banana" {
		t.Fatalf("Get = %q, want the environment override", got)
	}
}
