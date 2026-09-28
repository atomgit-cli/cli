package installupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestDetectDistributionEnvWins(t *testing.T) {
	if got := DetectDistribution(map[string]string{DistributionEnv: "pypi"}, "/opt/homebrew/bin/gc"); got != "pypi" {
		t.Fatalf("env declaration must win, got %q", got)
	}
}

func TestDetectDistributionByBinaryPath(t *testing.T) {
	cases := []struct {
		binary string
		want   string
	}{
		{"/usr/local/lib/node_modules/@gitcode-cli/cli/bin/gc.js", "npm"},
		{"/usr/local/lib/node_modules/@atomgit-cli/cli/bin/gc.js", "npm"},
		{"/usr/local/lib/node_modules/atomgit-cli/bin/gc.js", "npm"},
		{"/home/u/.local/pipx/venvs/gitcode-cli/bin/gitcode", "archive-or-source"},
		{"/opt/homebrew/Cellar/gc/0.14.0/bin/gc", "homebrew"},
		{"/usr/local/Cellar/gc/0.14.0/bin/gc", "homebrew"},
		{"/home/u/.local/bin/gc", "archive-or-source"},
		// Linuxbrew bin directory: matched by the /.linuxbrew/ marker.
		{"/home/linuxbrew/.linuxbrew/bin/gc", "homebrew"},
		// Apple Silicon bin directory: matched by /opt/homebrew/ directly.
		{"/opt/homebrew/bin/gc", "homebrew"},
		// The old "/homebrew/" substring was too broad: user tool
		// directories must not be misdetected as Homebrew.
		{"/home/u/tools/homebrew/gc", "archive-or-source"},
		// Intel Mac bin symlink, unresolved (missing on disk): the
		// fallback path keeps the table-test behavior.
		{"/usr/local/bin/gc", "archive-or-source"},
		// The pypi marker is anchored to the pip site-packages layout; a
		// user directory named gc_cli is not a pip install.
		{"/home/u/gc_cli/bin/gc", "archive-or-source"},
		{"/home/u/.local/pipx/venvs/gitcode-cli/lib/python3.11/site-packages/gc_cli/bin/gc-linux-amd64", "pypi"},
		{"/home/u/venv/lib/python3.12/site-packages/gc_cli/bin/gc-linux-arm64", "pypi"},
		// uv-managed tools.
		{"/home/u/.local/share/uv/tools/gc/bin/gc", "uv"},
		{"/home/u/AppData/Roaming/uv/tools/gc/bin/gc.exe", "uv"},
	}
	for _, tc := range cases {
		if got := DetectDistribution(map[string]string{}, tc.binary); got != tc.want {
			t.Fatalf("DetectDistribution(%q) = %q, want %q", tc.binary, got, tc.want)
		}
	}
}

func TestDetectDistributionResolvesHomebrewBinSymlinks(t *testing.T) {
	root := t.TempDir()
	cellarBin := filepath.Join(root, "usr", "local", "Cellar", "gc", "0.14.0", "bin", "gc")
	if err := os.MkdirAll(filepath.Dir(cellarBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(cellarBin, "binary"); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "usr", "local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "gc")
	if err := os.Symlink(cellarBin, link); err != nil {
		t.Skipf("creating symlinks requires privileges: %v", err)
	}
	if got := DetectDistribution(map[string]string{}, link); got != "homebrew" {
		t.Fatalf("symlinked Homebrew bin must resolve to homebrew, got %q", got)
	}

	// A symlink resolving outside any known layout stays archive-or-source.
	plainDir := filepath.Join(root, "tools")
	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(plainDir, "gc")
	if err := writeFile(plain, "binary"); err != nil {
		t.Fatal(err)
	}
	plainLink := filepath.Join(binDir, "gitcode")
	if err := os.Symlink(plain, plainLink); err != nil {
		t.Skipf("creating symlinks requires privileges: %v", err)
	}
	if got := DetectDistribution(map[string]string{}, plainLink); got != "archive-or-source" {
		t.Fatalf("unrecognized resolved path must stay archive-or-source, got %q", got)
	}
}

func TestDetectDistributionAdjacentManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, ".gitcode-install.json")
	if err := writeFile(manifest, `{"distribution":"npm-bootstrap","version":"0.14.0"}`); err != nil {
		t.Fatal(err)
	}
	if got := DetectDistribution(map[string]string{}, filepath.Join(dir, "gc")); got != "npm-bootstrap" {
		t.Fatalf("adjacent manifest must win over path heuristics, got %q", got)
	}
}
