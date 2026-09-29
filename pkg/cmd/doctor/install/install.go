// Package install diagnoses GitCode CLI installation sources and PATH conflicts.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/installupdate"
)

const (
	entrypointEnv  = "GITCODE_CLI_ENTRYPOINT"
	binaryEnv      = "GITCODE_CLI_BINARY"
	packageRootEnv = "GITCODE_CLI_PACKAGE_ROOT"
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
	return installupdate.DetectDistribution(env, binary)
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
// Keep in sync with LEFTOVER_PREFIXES there: a prefix known to only one
// side creates a report-without-cleanup loop.
var transactionLeftoverPrefixes = []string{
	"gc.backup-", "gc.tmp-", "gc.exe.backup-", "gc.exe.tmp-",
	"gitcode.backup-", "gitcode.tmp-", "gitcode.exe.backup-", "gitcode.exe.tmp-",
	"gitcode-update-helper.js.backup-", "gitcode-update-helper.js.tmp-",
	".gitcode-install.json.backup-", ".gitcode-install.json.tmp-",
	".gc-install-probe-",
}

// transactionLeftoverNames are matched by full name, mirroring the JS
// isTransactionLeftoverName equality check: ".gc-write-probe" with a suffix
// is not an installer artifact, and a prefix match would report files the
// npm sweep never cleans.
var transactionLeftoverNames = []string{".gc-write-probe"}

func isTransactionLeftoverName(name string) bool {
	for _, prefix := range transactionLeftoverPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, exact := range transactionLeftoverNames {
		if name == exact {
			return true
		}
	}
	return false
}

// transactionLeftovers lists interrupted-install leftovers in dir, split
// into regular files (sweepable by age after 24h) and symlinks (never
// swept: a renamed symlink keeps its original mtime, so age cannot prove
// it is not owned by an active transaction).
func transactionLeftovers(dir string) (files, symlinks []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isTransactionLeftoverName(entry.Name()) {
			continue
		}
		target := filepath.Join(dir, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			symlinks = append(symlinks, target)
		} else {
			files = append(files, target)
		}
	}
	sort.Strings(files)
	sort.Strings(symlinks)
	return files, symlinks
}

func addDiagnostics(report *Report, env map[string]string, goos string) {
	// PATH-candidate directories only: membership answers "is this
	// directory on PATH" (the leftover scan below uses a wider set).
	pathDirs := map[string]struct{}{}
	for _, command := range []string{"gitcode", "gc"} {
		for _, candidate := range report.Commands[command].Candidates {
			pathDirs[normalizedPath(filepath.Dir(candidate), goos)] = struct{}{}
		}
	}
	directories := map[string]struct{}{}
	for dir := range pathDirs {
		directories[dir] = struct{}{}
	}
	// The install directory is not necessarily on PATH, so leftovers there
	// are invisible to the candidate scan; include the binary and entrypoint
	// directories as well.
	for _, target := range []string{report.Binary, report.Entrypoint} {
		if target != "" && filepath.IsAbs(target) {
			directories[normalizedPath(filepath.Dir(target), goos)] = struct{}{}
		}
	}
	// An interrupted install leaves its temp/backup files in the target
	// directory even before any gc/gitcode entry exists there (interrupted
	// between the probe and the first replacePath), so scan every PATH
	// directory — the candidate set alone would miss those.
	for _, dir := range filepath.SplitList(env["PATH"]) {
		dir = strings.Trim(strings.TrimSpace(dir), `"`)
		if dir == "" {
			continue
		}
		directories[normalizedPath(dir, goos)] = struct{}{}
	}
	var leftoverFiles, leftoverSymlinks []string
	for dir := range directories {
		files, symlinks := transactionLeftovers(dir)
		leftoverFiles = append(leftoverFiles, files...)
		leftoverSymlinks = append(leftoverSymlinks, symlinks...)
	}
	report.Leftovers = append(report.Leftovers, leftoverFiles...)
	report.Leftovers = append(report.Leftovers, leftoverSymlinks...)
	sort.Strings(report.Leftovers)
	if len(report.Leftovers) > 0 {
		report.Conflicts = append(report.Conflicts,
			fmt.Sprintf("interrupted-install leftovers detected (%d file(s))", len(report.Leftovers)))
	}
	if len(leftoverFiles) > 0 {
		report.Recommendations = append(report.Recommendations,
			"rerun the npm bootstrap install to sweep stale leftovers (regular files older than 24h), or delete the listed files manually")
	}
	if len(leftoverSymlinks) > 0 {
		// Recommending the sweep here would be a no-op loop: the installer
		// deliberately never removes symlinks, so point at manual deletion.
		report.Recommendations = append(report.Recommendations,
			"delete the listed symlink leftovers manually: the installer never removes symlinks automatically (concurrency safety), so rerunning the install will not clear them")
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
	switch report.Distribution {
	case "npm":
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
	case "npm-bootstrap":
		// The bootstrap layout records its bin directory in the adjacent
		// manifest. An install directory that is not on PATH (direct
		// invocation by full path) is invisible to the candidate-based
		// checks; a shadowed-but-on-PATH install is covered by the
		// multiple-provider check above.
		if target := manifestTargetDir(report.Binary); target != "" {
			if _, ok := pathDirs[normalizedPath(target, goos)]; !ok {
				report.Conflicts = append(report.Conflicts,
					fmt.Sprintf("the bootstrap install at %s is not on PATH (invoked directly)", target))
				report.Recommendations = append(report.Recommendations,
					fmt.Sprintf("add %s to PATH or remove the other providers so the bootstrap install resolves", target))
			}
		}
	case "pnpm":
		// pnpm records no npm prefix; compare against PNPM_HOME when it is
		// set so a shadowed pnpm global install is reported instead of
		// silently skipped. Without PNPM_HOME there is no expected
		// directory to compare against, so stay quiet rather than guess.
		// Project-level pnpm dependencies (global:false) legitimately
		// resolve through the owning project's node_modules/.bin — only a
		// global install compares against PNPM_HOME.
		if home := env["PNPM_HOME"]; home != "" && pnpmGlobal(env[packageRootEnv]) {
			if selected := report.Commands["gitcode"].Selected; selected != "" &&
				normalizedPath(filepath.Dir(selected), goos) != normalizedPath(home, goos) {
				report.Conflicts = append(report.Conflicts, "another gitcode command appears before the pnpm global bin directory (PNPM_HOME)")
				report.Recommendations = append(report.Recommendations, fmt.Sprintf("move %s before %s on PATH, or uninstall the older channel explicitly", home, filepath.Dir(selected)))
			}
		}
	case "npm-local":
		// A project-local dependency is not a global install: the npm
		// prefix comparison does not apply and would only produce noise.
		report.Recommendations = append(report.Recommendations,
			"this is a project-local npm dependency, not a global install; run \"npm install -g <package>\" to switch to the global channel, or manage updates in the owning project")
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

// pnpmGlobal reports whether the recorded pnpm install is a global one.
// Project-level pnpm dependencies are recorded as {distribution: "pnpm",
// global: false}: their PATH resolution through the owning project's
// node_modules/.bin is pnpm's designed behavior, not a channel conflict.
// A missing or unreadable manifest reports false (no comparison).
func pnpmGlobal(packageRoot string) bool {
	if packageRoot == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(packageRoot, ".gitcode-install.json"))
	if err != nil {
		return false
	}
	var metadata struct {
		Global bool `json:"global"`
	}
	if json.Unmarshal(data, &metadata) != nil {
		return false
	}
	return metadata.Global
}

// manifestTargetDir reads the targetDir recorded in the bootstrap manifest
// adjacent to the binary (empty when absent or unreadable).
func manifestTargetDir(binary string) string {
	if binary == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(binary), ".gitcode-install.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		TargetDir string `json:"targetDir"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	return manifest.TargetDir
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
