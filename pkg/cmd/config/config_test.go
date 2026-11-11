package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestNewCmdList(t *testing.T) {
	cmd := newCmdList(cmdutil.TestFactory())
	if cmd == nil {
		t.Fatal("newCmdList returned nil")
	}
	if cmd.Use != "list" {
		t.Fatalf("cmd.Use = %q, want %q", cmd.Use, "list")
	}
}

func TestNewCmdListFlagsExist(t *testing.T) {
	cmd := newCmdList(cmdutil.TestFactory())
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("json flag missing")
	}
}

func TestNewCmdClearCache(t *testing.T) {
	cmd := newCmdClearCache(cmdutil.TestFactory())
	if cmd == nil {
		t.Fatal("newCmdClearCache returned nil")
	}
	if cmd.Use != "clear-cache" {
		t.Fatalf("cmd.Use = %q, want %q", cmd.Use, "clear-cache")
	}
}

func TestListConfig(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	_ = cfg.Set(defaultHost, "browser", "firefox")
	_ = cfg.Set(defaultHost, "default_repo", "owner/repo")
	_ = cfg.Write()

	items, err := listConfig(cfg)
	if err != nil {
		t.Fatalf("listConfig() error = %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}
	want := map[string]string{
		"browser":      "config",
		"default_repo": "config",
		"editor":       "default",
		"pager":        "default",
		"update.mode":  "default",
	}
	for _, item := range items {
		if src, ok := want[item.Key]; ok {
			if item.Source != src {
				t.Errorf("key %q: source = %q, want %q", item.Key, item.Source, src)
			}
		}
	}
}

func TestListConfigEnvOverride(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_BROWSER", "chrome")
	cfg := config.New()

	items, _ := listConfig(cfg)
	for _, item := range items {
		if item.Key == "browser" {
			if item.Value != "chrome" {
				t.Fatalf("browser value = %q, want %q", item.Value, "chrome")
			}
			if item.Source != "environment" {
				t.Fatalf("browser source = %q, want %q", item.Source, "environment")
			}
		}
	}
}

func TestListConfigUpdateModeDefault(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()

	items, _ := listConfig(cfg)
	for _, item := range items {
		if item.Key == "update.mode" {
			if item.Value != "notify" {
				t.Fatalf("update.mode value = %q, want %q", item.Value, "notify")
			}
			if item.Source != "default" {
				t.Fatalf("update.mode source = %q, want %q", item.Source, "default")
			}
		}
	}
}

func TestListConfigJSONOutput(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	f := cmdutil.TestFactory()
	cmd := newCmdList(f)
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var items []configListItem
	if err := json.Unmarshal([]byte(buf.String()), &items); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, buf.String())
	}
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}
}

func TestListConfigHumanReadable(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	f := cmdutil.TestFactory()
	cmd := newCmdList(f)
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := buf.String()
	for _, key := range []string{"browser", "editor", "pager", "update.mode", "default_repo"} {
		if !strings.Contains(out, key) {
			t.Errorf("output missing key %q; output=%s", key, out)
		}
	}
}

func TestClearCacheEmpty(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	io, _, out, _ := iostreams.Test()
	if err := clearCache(io); err != nil {
		t.Fatalf("clearCache() error = %v", err)
	}
	if !strings.Contains(out.String(), "No cache to clear") {
		t.Fatalf("output should say no cache; got: %s", out.String())
	}
}

func TestClearCacheWithFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "test.tmp"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	io, _, out, _ := iostreams.Test()
	if err := clearCache(io); err != nil {
		t.Fatalf("clearCache() error = %v", err)
	}
	if !strings.Contains(out.String(), "Cleared 1 cache") {
		t.Fatalf("output should say cleared 1; got: %s", out.String())
	}
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 0 {
		t.Fatalf("cache dir should be empty; has %d entries", len(entries))
	}
}

func TestResolveConfigValueEnv(t *testing.T) {
	t.Setenv("GC_BROWSER", "safari")
	cfg := config.New()
	value, source := resolveConfigValue(cfg, "browser")
	if value != "safari" || source != "environment" {
		t.Fatalf("value=%q source=%q, want safari/environment", value, source)
	}
}

func TestResolveConfigValueConfig(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	_ = cfg.Set(defaultHost, "editor", "vim")
	_ = cfg.Write()
	value, source := resolveConfigValue(cfg, "editor")
	if value != "vim" || source != "config" {
		t.Fatalf("value=%q source=%q, want vim/config", value, source)
	}
}

func TestResolveConfigValueDefault(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	value, source := resolveConfigValue(cfg, "default_repo")
	if value != "" || source != "default" {
		t.Fatalf("value=%q source=%q, want empty/default", value, source)
	}
}

func TestResolveConfigValueUpdateModeDefault(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	value, source := resolveConfigValue(cfg, "update.mode")
	if value != "notify" || source != "default" {
		t.Fatalf("value=%q source=%q, want notify/default", value, source)
	}
}

func TestConfigDirFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)
	got := configDir()
	if got != dir {
		t.Fatalf("configDir() = %q, want %q", got, dir)
	}
}

func TestPluralEntry(t *testing.T) {
	if got := pluralEntry(1); got != "y" {
		t.Errorf("pluralEntry(1) = %q, want %q", got, "y")
	}
	if got := pluralEntry(2); got != "ies" {
		t.Errorf("pluralEntry(2) = %q, want %q", got, "ies")
	}
}

func TestNewCmdConfigHelpDocumentsConfigDir(t *testing.T) {
	// gc config operates on the directory that GC_CONFIG_DIR overrides;
	// it must be discoverable from gc config --help.
	f := cmdutil.TestFactory()
	cmd := NewCmdConfig(f)
	if !strings.Contains(cmd.Long, "GC_CONFIG_DIR") {
		t.Errorf("config Long does not mention GC_CONFIG_DIR: %q", cmd.Long)
	}
}
