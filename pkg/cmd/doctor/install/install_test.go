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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	first := t.TempDir()
	second := t.TempDir()
	name := "gitcode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	for _, dir := range []string{first, second} {
		dir := dir
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	report := Inspect([]string{"PATH=" + first + string(os.PathListSeparator) + second}, runtime.GOOS, "", "", "")
	found := false
	for _, conflict := range report.Conflicts {
		conflict := conflict
		if strings.Contains(conflict, "gitcode is provided by multiple PATH directories") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected multiple-provider conflict, got %#v", report.Conflicts)
	}
}

func TestDoctorInstallJSON(t *testing.T) {
	t.Parallel()
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
