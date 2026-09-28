package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestInspectUsesWrapperMetadataAndFindsCandidates(t *testing.T) {
	dir := t.TempDir()
	command := "gitcode"
	if runtime.GOOS == "windows" {
		command += ".exe"
	}
	commandPath := filepath.Join(dir, command)
	if err := os.WriteFile(commandPath, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	environ := []string{
		"PATH=" + dir,
		"GITCODE_CLI_DISTRIBUTION=pypi",
		entrypointEnv + "=" + commandPath,
		binaryEnv + "=" + filepath.Join(dir, "gc-binary"),
	}

	report := Inspect(environ, runtime.GOOS, "1.2.3", "abc", "today")
	if report.Distribution != "pypi" {
		t.Fatalf("Distribution = %q, want pypi", report.Distribution)
	}
	if report.Version != "1.2.3" || report.Commit != "abc" || report.Built != "today" {
		t.Fatalf("unexpected version metadata: %#v", report)
	}
	if report.Commands["gitcode"].Selected != commandPath {
		t.Fatalf("selected = %q, want %q", report.Commands["gitcode"].Selected, commandPath)
	}
}

func TestInspectDetectsBootstrapManifest(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "gitcode")
	if err := os.WriteFile(binary, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"distribution":"npm-bootstrap"}`)
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}

	report := Inspect([]string{"PATH=" + dir, binaryEnv + "=" + binary}, runtime.GOOS, "", "", "")
	if report.Distribution != "npm-bootstrap" {
		t.Fatalf("Distribution = %q, want npm-bootstrap", report.Distribution)
	}
}

func TestInspectReportsMultipleProviderDirectories(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	report := Inspect([]string{"PATH=" + first + string(os.PathListSeparator) + second}, runtime.GOOS, "", "", "")
	found := false
	for _, conflict := range report.Conflicts {
		if strings.Contains(conflict, "gitcode is provided by multiple PATH directories") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected multiple-provider conflict, got %#v", report.Conflicts)
	}
}

func TestInspectScansBinaryDirectoryForLeftovers(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "gitcode-cli", "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "gc.backup-123-abc"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The install directory is not on PATH: only the binary env points there,
	// so its leftovers are invisible to the candidate scan alone.
	environ := []string{
		"PATH=" + t.TempDir(),
		binaryEnv + "=" + filepath.Join(installDir, "gc"),
	}
	report := Inspect(environ, runtime.GOOS, "", "", "")
	found := false
	for _, leftover := range report.Leftovers {
		if strings.HasSuffix(leftover, "gc.backup-123-abc") {
			found = true
		}
	}
	if !found {
		t.Fatalf("leftover in the non-PATH install dir must be reported, got %#v", report.Leftovers)
	}
}

func TestInspectNpmLocalSkipsGlobalPrefixConflict(t *testing.T) {
	dir := t.TempDir()
	packageRoot := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A stale global prefix in the metadata must not produce the npm-global
	// conflict for a project-local install.
	if err := os.WriteFile(filepath.Join(packageRoot, ".gitcode-install.json"),
		[]byte(`{"distribution":"npm-local","prefix":"/nowhere"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	environ := []string{
		"PATH=" + dir,
		"GITCODE_CLI_DISTRIBUTION=npm-local",
		packageRootEnv + "=" + packageRoot,
	}
	report := Inspect(environ, runtime.GOOS, "", "", "")
	if report.Distribution != "npm-local" {
		t.Fatalf("Distribution = %q, want npm-local", report.Distribution)
	}
	for _, conflict := range report.Conflicts {
		if strings.Contains(conflict, "npm global bin") {
			t.Fatalf("npm-local must not produce the npm-global conflict: %q", conflict)
		}
	}
	found := false
	for _, rec := range report.Recommendations {
		if strings.Contains(rec, "project-local npm dependency") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a project-local recommendation, got %#v", report.Recommendations)
	}
}

func TestInspectReportsBootstrapInstallNotOnPath(t *testing.T) {
	installDir := t.TempDir()
	otherDir := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(installDir, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"distribution":"npm-bootstrap","targetDir":"` + strings.ReplaceAll(installDir, `\`, `\\`) + `"}`
	if err := os.WriteFile(filepath.Join(installDir, ".gitcode-install.json"), []byte(manifestJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	environ := []string{
		"PATH=" + otherDir,
		binaryEnv + "=" + filepath.Join(installDir, name),
	}
	report := Inspect(environ, runtime.GOOS, "", "", "")
	found := false
	for _, conflict := range report.Conflicts {
		if strings.Contains(conflict, "not on PATH") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an off-PATH bootstrap install must be reported, got %#v", report.Conflicts)
	}

	// With the install directory on PATH the shadowing is covered by the
	// multiple-provider check instead; no "not on PATH" conflict.
	onPath := Inspect([]string{
		"PATH=" + otherDir + string(os.PathListSeparator) + installDir,
		binaryEnv + "=" + filepath.Join(installDir, name),
	}, runtime.GOOS, "", "", "")
	for _, conflict := range onPath.Conflicts {
		if strings.Contains(conflict, "not on PATH") {
			t.Fatalf("an on-PATH bootstrap install must not report off-PATH: %q", conflict)
		}
	}
}

func TestInspectPnpmProjectDependencyIsNotAChannelConflict(t *testing.T) {
	projBin := t.TempDir()
	home := t.TempDir()
	packageRoot := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(projBin, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A project-level pnpm dependency: the recorded metadata says
	// global:false, and PATH resolving through the project's
	// node_modules/.bin is pnpm's designed behavior.
	if err := os.WriteFile(filepath.Join(packageRoot, ".gitcode-install.json"),
		[]byte(`{"distribution":"pnpm","global":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	report := Inspect([]string{
		"PATH=" + projBin,
		"GITCODE_CLI_DISTRIBUTION=pnpm",
		"GITCODE_CLI_PACKAGE_ROOT=" + packageRoot,
		"PNPM_HOME=" + home,
	}, runtime.GOOS, "", "", "")
	for _, conflict := range report.Conflicts {
		if strings.Contains(conflict, "PNPM_HOME") {
			t.Fatalf("a project-level pnpm dependency must not report a channel conflict: %q", conflict)
		}
	}
}

func TestInspectPnpmComparesAgainstPnpmHome(t *testing.T) {
	oldDir := t.TempDir()
	home := t.TempDir()
	packageRoot := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(oldDir, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A global pnpm install records {distribution: "pnpm", global: true}.
	if err := os.WriteFile(filepath.Join(packageRoot, ".gitcode-install.json"),
		[]byte(`{"distribution":"pnpm","global":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	environ := []string{
		"PATH=" + oldDir,
		"GITCODE_CLI_DISTRIBUTION=pnpm",
		"GITCODE_CLI_PACKAGE_ROOT=" + packageRoot,
		"PNPM_HOME=" + home,
	}
	report := Inspect(environ, runtime.GOOS, "", "", "")
	found := false
	for _, conflict := range report.Conflicts {
		if strings.Contains(conflict, "PNPM_HOME") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a PNPM_HOME conflict, got %#v", report.Conflicts)
	}

	// The selected entry inside PNPM_HOME must not conflict.
	if err := os.WriteFile(filepath.Join(home, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	clean := Inspect([]string{
		"PATH=" + home + string(os.PathListSeparator) + oldDir,
		"GITCODE_CLI_DISTRIBUTION=pnpm",
		"GITCODE_CLI_PACKAGE_ROOT=" + packageRoot,
		"PNPM_HOME=" + home,
	}, runtime.GOOS, "", "", "")
	for _, conflict := range clean.Conflicts {
		if strings.Contains(conflict, "PNPM_HOME") {
			t.Fatalf("selected inside PNPM_HOME must not conflict: %q", conflict)
		}
	}
}

func TestDoctorInstallJSON(t *testing.T) {
	cmd := NewCmdInstall(cmdutil.TestFactory(), "1.2.3", "abc", "today")
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if report.Version != "1.2.3" {
		t.Fatalf("Version = %q, want 1.2.3", report.Version)
	}
}

func TestInspectReportsInterruptedInstallLeftovers(t *testing.T) {
	dir := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	leftovers := []string{
		"gc.backup-123-abc",
		"gitcode.tmp-456-def",
		"gitcode-update-helper.js.backup-123-abc",
		".gc-install-probe-789",
		".gc-write-probe",
		"gc.exe.backup-123-abc",
		"gitcode.exe.tmp-456-def",
		".gitcode-install.json.tmp-789-xyz",
	}
	for _, leftover := range leftovers {
		if err := os.WriteFile(filepath.Join(dir, leftover), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A suffixed ".gc-write-probe-x" is not an installer artifact (JS matches
	// the exact name only) and must not be reported.
	if err := os.WriteFile(filepath.Join(dir, ".gc-write-probe-x9"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.backup-x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A symlink backup (an interrupted alias migration renames the link
	// itself) must be reported with manual-deletion guidance: the sweep
	// never removes symlinks, so the rerun-install advice would loop.
	createdSymlink := true
	if err := os.Symlink(filepath.Join(dir, "gc"), filepath.Join(dir, "gitcode.backup-sym-789")); err != nil {
		createdSymlink = false
	}
	wantLeftovers := len(leftovers)
	if createdSymlink {
		wantLeftovers++
	}
	report := Inspect([]string{"PATH=" + dir}, runtime.GOOS, "", "", "")
	if len(report.Leftovers) != wantLeftovers {
		t.Fatalf("Leftovers = %#v, want %d entries", report.Leftovers, wantLeftovers)
	}
	for _, leftover := range leftovers {
		found := false
		for _, entry := range report.Leftovers {
			if strings.HasSuffix(entry, leftover) {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected leftover %q in %#v", leftover, report.Leftovers)
		}
	}
	conflict := false
	recommendation := false
	for _, line := range report.Conflicts {
		if strings.Contains(line, "interrupted-install leftovers detected") {
			conflict = true
		}
	}
	for _, line := range report.Recommendations {
		if strings.Contains(line, "rerun the npm bootstrap install") {
			recommendation = true
		}
	}
	if !conflict || !recommendation {
		t.Fatalf("expected leftover conflict and recommendation, got conflicts=%#v recommendations=%#v",
			report.Conflicts, report.Recommendations)
	}
	if createdSymlink {
		symlinkRecommendation := false
		for _, line := range report.Recommendations {
			if strings.Contains(line, "symlink leftovers") && strings.Contains(line, "manually") {
				symlinkRecommendation = true
			}
		}
		if !symlinkRecommendation {
			t.Fatalf("expected manual-deletion recommendation for symlink leftovers, got %#v", report.Recommendations)
		}
	}
}
