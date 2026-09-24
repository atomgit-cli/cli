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
	if manifestDistribution := adjacentManifestDistribution(binary); manifestDistribution != "" {
		return manifestDistribution
	}
	normalized := filepath.ToSlash(strings.ToLower(binary))
	switch {
	case strings.Contains(normalized, "/node_modules/@gitcode-cli/cli/"),
		strings.Contains(normalized, "/node_modules/@atomgit-cli/cli/"),
		strings.Contains(normalized, "/node_modules/atomgit-cli/"):
		return "npm"
	case strings.Contains(normalized, "/gc_cli/bin/"):
		return "pypi"
	case strings.Contains(normalized, "/cellar/gc/") || strings.Contains(normalized, "/homebrew/"):
		return "homebrew"
	case normalized == "/usr/bin/gc" || normalized == "/usr/bin/gitcode":
		return detectSystemPackage(binary)
	default:
		return "archive-or-source"
	}
}

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
	return "system-package"
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
