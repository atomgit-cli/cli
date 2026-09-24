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
	}
	for _, tc := range cases {
		if got := DetectDistribution(map[string]string{}, tc.binary); got != tc.want {
			t.Fatalf("DetectDistribution(%q) = %q, want %q", tc.binary, got, tc.want)
		}
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
