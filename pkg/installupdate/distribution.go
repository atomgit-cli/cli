// Package installupdate: installation-channel detection shared by doctor
// install and the explicit update command.
package installupdate

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DistributionEnv is the environment variable npm wrappers set to declare
// their installation channel.
const DistributionEnv = "GITCODE_CLI_DISTRIBUTION"

// DetectDistribution resolves the installation channel of the running
// binary: an explicit environment declaration wins, then the bootstrap
// manifest next to the binary, then path-based heuristics (npm, PyPI,
// Homebrew, system package managers).
func DetectDistribution(env map[string]string, binary string) string {
	if value := strings.TrimSpace(env[DistributionEnv]); value != "" {
		return value
	}
	// Resolve symlinks before matching path markers: Homebrew's bin
	// directory is a symlink farm into the Cellar (/usr/local/bin/gc on
	// Intel Macs, /home/linuxbrew/.linuxbrew/bin/gc on Linux), and
	// os.Executable does not guarantee resolution on every platform. Fall
	// back to the unresolved path when resolution fails (missing paths,
	// odd environments) so the heuristics still apply.
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	if manifestDistribution := adjacentManifestDistribution(binary); manifestDistribution != "" {
		return manifestDistribution
	}
	normalized := filepath.ToSlash(strings.ToLower(binary))
	switch {
	case strings.Contains(normalized, "/node_modules/@gitcode-cli/cli/"),
		strings.Contains(normalized, "/node_modules/@atomgit-cli/cli/"),
		strings.Contains(normalized, "/node_modules/atomgit-cli/"):
		return "npm"
	case strings.Contains(normalized, "/site-packages/gc_cli/bin/"):
		return "pypi"
	case strings.Contains(normalized, "/uv/tools/"):
		return "uv"
	case strings.Contains(normalized, "/cellar/gc/"),
		strings.Contains(normalized, "/opt/homebrew/"),
		strings.Contains(normalized, "/.linuxbrew/"):
		return "homebrew"
	case normalized == "/usr/bin/gc" || normalized == "/usr/bin/gitcode":
		return detectSystemPackage(binary)
	default:
		return "archive-or-source"
	}
}

// detectSystemPackage attributes /usr/bin binaries to dpkg or rpm. When
// neither manager claims ownership the binary is a manual root copy, and
// "system-package" guidance (use apt/dnf) would point at repositories that
// do not carry it — fall back to archive-or-source instead.
func detectSystemPackage(binary string) string {
	checks := []struct {
		command      string
		args         []string
		distribution string
	}{
		{command: "dpkg-query", args: []string{"-S", binary}, distribution: "deb"},
		{command: "rpm", args: []string{"-qf", binary}, distribution: "rpm"},
	}
	for _, check := range checks {
		commandPath, err := exec.LookPath(check.command)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = exec.CommandContext(ctx, commandPath, check.args...).Run()
		cancel()
		if err == nil {
			return check.distribution
		}
	}
	return "archive-or-source"
}

func adjacentManifestDistribution(binary string) string {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(binary), ".gitcode-install.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Distribution string `json:"distribution"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	return manifest.Distribution
}
