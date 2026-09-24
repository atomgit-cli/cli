// Package install diagnoses GitCode CLI installation sources and PATH conflicts.
package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

const (
	distributionEnv = "GITCODE_CLI_DISTRIBUTION"
	entrypointEnv   = "GITCODE_CLI_ENTRYPOINT"
	binaryEnv       = "GITCODE_CLI_BINARY"
	packageRootEnv  = "GITCODE_CLI_PACKAGE_ROOT"
)

// CommandResolution describes how a command name resolves through PATH.
type CommandResolution struct {
	Selected   string   `json:"selected,omitempty"`
	Candidates []string `json:"candidates"`
}

// Report is the stable JSON contract for doctor install.
type Report struct {
	Version           string                       `json:"version"`
	Commit            string                       `json:"commit,omitempty"`
	Built             string                       `json:"built,omitempty"`
	Distribution      string                       `json:"distribution"`
	Entrypoint        string                       `json:"entrypoint"`
	Binary            string                       `json:"binary"`
	Commands          map[string]CommandResolution `json:"commands"`
	PowerShellGCAlias bool                         `json:"powershell_gc_alias"`
	Conflicts         []string                     `json:"conflicts"`
	Recommendations   []string                     `json:"recommendations"`
	Leftovers         []string                     `json:"leftovers,omitempty"`
}

// NewCmdInstall creates the doctor install command.
func NewCmdInstall(_ *cmdutil.Factory, version, commit, built string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Inspect installation source and command conflicts",
		Long: `Inspect the current executable, its distribution channel, and every gc or
gitcode candidate visible on PATH. This command is offline and does not read
authentication configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := Inspect(os.Environ(), runtime.GOOS, version, commit, built)
			if jsonOutput {
				return cmdutil.WriteJSON(cmd.OutOrStdout(), report)
			}
			writeHuman(cmd, report)
			return nil
		},
	}
	cmdutil.AddJSONFlag(cmd, &jsonOutput)
	return cmd
}

// Inspect builds an installation report. Empty version fields are filled by callers in production.
func Inspect(environ []string, goos, version, commit, built string) Report {
	env := environmentMap(environ)
	binary := env[binaryEnv]
	if binary == "" {
		binary, _ = os.Executable()
	}
	entrypoint := env[entrypointEnv]
	if entrypoint == "" {
		entrypoint = binary
	}
	distribution := detectDistribution(env, binary)
	report := Report{
		Version:           version,
		Commit:            commit,
		Built:             built,
		Distribution:      distribution,
		Entrypoint:        entrypoint,
		Binary:            binary,
		Commands:          map[string]CommandResolution{},
		PowerShellGCAlias: goos == "windows",
		Conflicts:         []string{},
		Recommendations:   []string{},
	}
	for _, name := range []string{"gc", "gitcode"} {
		candidates := commandCandidates(name, env, goos)
		selected := ""
		if len(candidates) > 0 {
			selected = candidates[0]
		}
		report.Commands[name] = CommandResolution{Selected: selected, Candidates: candidates}
	}
	addDiagnostics(&report, env, goos)
	return report
}

func environmentMap(environ []string) map[string]string {
	env := make(map[string]string, len(environ))
	for _, item := range environ {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[strings.ToUpper(key)] = value
		}
	}
	return env
}

func detectDistribution(env map[string]string, binary string) string {
	if value := strings.TrimSpace(env[distributionEnv]); value != "" {
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

func commandCandidates(name string, env map[string]string, goos string) []string {
	pathValue := env["PATH"]
	extensions := []string{""}
	if goos == "windows" {
		extensions = []string{".exe", ".com", ".bat", ".cmd", ".ps1", ""}
	}
	seen := map[string]struct{}{}
	var candidates []string
	for _, dir := range filepath.SplitList(pathValue) {
		dir = strings.Trim(strings.TrimSpace(dir), `"`)
		if dir == "" {
			continue
		}
		for _, extension := range extensions {
			candidate := filepath.Join(dir, name+extension)
			key := normalizedPath(candidate, goos)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}

// transactionLeftoverPrefixes matches the temp/backup file names an
// interrupted bootstrap install can leave behind (see npm/lib/install.js).
var transactionLeftoverPrefixes = []string{
	"gc.backup-", "gc.tmp-", "gc.exe.backup-", "gc.exe.tmp-",
	"gitcode.backup-", "gitcode.tmp-", "gitcode.exe.backup-", "gitcode.exe.tmp-",
	"gitcode-update-helper.js.backup-", "gitcode-update-helper.js.tmp-",
	".gc-install-probe-", ".gc-write-probe",
}

// transactionLeftovers lists interrupted-install leftover files in dir.
func transactionLeftovers(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, prefix := range transactionLeftoverPrefixes {
			if strings.HasPrefix(entry.Name(), prefix) {
				found = append(found, filepath.Join(dir, entry.Name()))
				break
			}
		}
	}
	sort.Strings(found)
	return found
}

func addDiagnostics(report *Report, env map[string]string, goos string) {
	directories := map[string]struct{}{}
	for _, command := range []string{"gitcode", "gc"} {
		for _, candidate := range report.Commands[command].Candidates {
			directories[normalizedPath(filepath.Dir(candidate), goos)] = struct{}{}
		}
	}
	for dir := range directories {
		report.Leftovers = append(report.Leftovers, transactionLeftovers(dir)...)
	}
	sort.Strings(report.Leftovers)
	if len(report.Leftovers) > 0 {
		report.Conflicts = append(report.Conflicts,
			fmt.Sprintf("interrupted-install leftovers detected (%d file(s))", len(report.Leftovers)))
		report.Recommendations = append(report.Recommendations,
			"rerun the npm bootstrap install to sweep stale leftovers (regular files older than 24h), or delete the listed files manually")
	}
	if report.PowerShellGCAlias {
		report.Conflicts = append(report.Conflicts, `Windows PowerShell may resolve "gc" as the Get-Content alias`)
		report.Recommendations = append(report.Recommendations, `use "gitcode" in PowerShell; do not remove the built-in gc alias globally`)
	}
	for _, command := range []string{"gitcode", "gc"} {
		resolution := report.Commands[command]
		directories := map[string]struct{}{}
		for _, candidate := range resolution.Candidates {
			directories[normalizedPath(filepath.Dir(candidate), goos)] = struct{}{}
		}
		if len(directories) > 1 {
			report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s is provided by multiple PATH directories", command))
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("keep the intended %s provider first on PATH and explicitly upgrade or remove the others", command))
		}
	}
	if report.Distribution == "npm" {
		metadataPrefix := npmPrefix(env[packageRootEnv])
		selected := report.Commands["gitcode"].Selected
		if metadataPrefix != "" && selected != "" {
			expected := metadataPrefix
			if goos != "windows" {
				expected = filepath.Join(expected, "bin")
			}
			if normalizedPath(filepath.Dir(selected), goos) != normalizedPath(expected, goos) {
				report.Conflicts = append(report.Conflicts, "another gitcode command appears before the npm global bin directory")
				report.Recommendations = append(report.Recommendations, fmt.Sprintf("move %s before %s on PATH, or uninstall the older global channel explicitly", expected, filepath.Dir(selected)))
			}
		}
	}
	if len(report.Conflicts) == 0 {
		report.Recommendations = append(report.Recommendations, "no command conflict detected")
	}
	sort.Strings(report.Conflicts)
}

func npmPrefix(packageRoot string) string {
	if packageRoot == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(packageRoot, ".gitcode-install.json"))
	if err != nil {
		return ""
	}
	var metadata struct {
		Prefix string `json:"prefix"`
	}
	if json.Unmarshal(data, &metadata) != nil {
		return ""
	}
	return metadata.Prefix
}

func normalizedPath(value, goos string) string {
	clean := filepath.Clean(value)
	if goos == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}

func writeHuman(cmd *cobra.Command, report Report) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Distribution: %s\n", report.Distribution)
	fmt.Fprintf(out, "Entrypoint:   %s\n", report.Entrypoint)
	fmt.Fprintf(out, "Binary:       %s\n", report.Binary)
	for _, name := range []string{"gc", "gitcode"} {
		resolution := report.Commands[name]
		fmt.Fprintf(out, "%s selected: %s\n", name, emptyValue(resolution.Selected))
		for _, candidate := range resolution.Candidates {
			fmt.Fprintf(out, "  - %s\n", candidate)
		}
	}
	for _, conflict := range report.Conflicts {
		fmt.Fprintf(out, "Conflict: %s\n", conflict)
	}
	for _, leftover := range report.Leftovers {
		fmt.Fprintf(out, "Leftover: %s\n", leftover)
	}
	for _, recommendation := range report.Recommendations {
		fmt.Fprintf(out, "Recommendation: %s\n", recommendation)
	}
}

func emptyValue(value string) string {
	if value == "" {
		return "(not found)"
	}
	return value
}
