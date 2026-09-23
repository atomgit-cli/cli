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
		distributionEnv + "=pypi",
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
	}
	for _, leftover := range leftovers {
		if err := os.WriteFile(filepath.Join(dir, leftover), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.backup-x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := Inspect([]string{"PATH=" + dir}, runtime.GOOS, "", "", "")
	if len(report.Leftovers) != len(leftovers) {
		t.Fatalf("Leftovers = %#v, want %d entries", report.Leftovers, len(leftovers))
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
}
