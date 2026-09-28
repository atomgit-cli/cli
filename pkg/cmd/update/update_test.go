package update

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/installupdate"
)

func TestNonManagedInstallReturnsManualGuidance(t *testing.T) {
	t.Setenv("GITCODE_CLI_BINARY", t.TempDir()+"/missing/gc")
	t.Setenv("GITCODE_CLI_DISTRIBUTION", "pypi")
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "manual" || got.Distribution != "pypi" || !strings.Contains(got.Message, "pipx") {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestManagerMessagesNeverClaimToUninstallOtherChannels(t *testing.T) {
	for _, distribution := range []string{"pypi", "homebrew", "system-package", "npm", "archive"} {
		message := strings.ToLower(managerMessage(distribution))
		if strings.Contains(message, "uninstall") || strings.Contains(message, "remove") {
			t.Fatalf("%s guidance mutates another channel: %s", distribution, message)
		}
	}
}

func TestManagerMessageCoversDetectedChannels(t *testing.T) {
	cases := map[string]string{
		"homebrew":          "brew upgrade gc",
		"deb":               "package manager",
		"rpm":               "package manager",
		"pypi":              "pipx or pip",
		"uv":                "uv tool upgrade gc",
		"npm":               "npm",
		"pnpm":              "npm gitcode wrapper",
		"npm-local":         "npm gitcode wrapper",
		"npm-bootstrap":     "gitcode install",
		"archive-or-source": "release archive",
	}
	for distribution, want := range cases {
		message := managerMessage(distribution)
		if !strings.Contains(message, want) {
			t.Fatalf("managerMessage(%q) = %q, want it to mention %q", distribution, message, want)
		}
	}
}

func TestUpdateUsesPathDetectionWhenEnvUnset(t *testing.T) {
	t.Setenv("GITCODE_CLI_DISTRIBUTION", "")
	binary := t.TempDir() + "/Cellar/gc/0.14.0/bin/gc"
	t.Setenv("GITCODE_CLI_BINARY", binary)
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "manual" || got.Distribution != "homebrew" || !strings.Contains(got.Message, "brew upgrade gc") {
		t.Fatalf("expected homebrew guidance via path detection, got %#v", got)
	}
}

func TestUpdateBootstrapCurrentDoesNotSchedule(t *testing.T) {
	bootstrapManifestDir(t)
	scheduled := restoreUpdateSeams(t, &installupdate.CheckResult{
		Status: "current", Distribution: "npm-bootstrap", Current: "1.2.3", Latest: "1.2.3",
		Message: "GitCode CLI 1.2.3 is current.",
	}, nil)
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "current" || got.Latest != "1.2.3" {
		t.Fatalf("expected current passthrough, got %#v", got)
	}
	if *scheduled != 0 {
		t.Fatalf("startDetached must not run when already current, ran %d times", *scheduled)
	}
}

func TestUpdateBootstrapAvailableSchedules(t *testing.T) {
	bootstrapManifestDir(t)
	scheduled := restoreUpdateSeams(t, &installupdate.CheckResult{
		Status: "available", Distribution: "npm-bootstrap", Current: "1.2.3", Latest: "1.3.0",
		Message: "GitCode CLI 1.3.0 is available.",
	}, nil)
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "scheduled" || got.Latest != "1.3.0" {
		t.Fatalf("expected scheduled result, got %#v", got)
	}
	if *scheduled != 1 {
		t.Fatalf("startDetached must run exactly once, ran %d times", *scheduled)
	}
}

func TestUpdateBootstrapCheckFailureSurfacesError(t *testing.T) {
	bootstrapManifestDir(t)
	scheduled := restoreUpdateSeams(t, nil, errors.New("update check failed: network ECONNRESET"))
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	cmd.SilenceUsage = true
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("a failed foreground check must exit non-zero")
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "error" || !strings.Contains(got.Message, "ECONNRESET") {
		t.Fatalf("error result must carry the real cause, got %#v", got)
	}
	if *scheduled != 0 {
		t.Fatalf("startDetached must not run after a failed check, ran %d times", *scheduled)
	}
}

func TestUpdateBootstrapBusyPassthroughDoesNotSchedule(t *testing.T) {
	bootstrapManifestDir(t)
	scheduled := restoreUpdateSeams(t, &installupdate.CheckResult{
		Status: "busy", Distribution: "npm-bootstrap", Current: "1.2.3", Latest: "",
		Message: "Another update is running.",
	}, nil)
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "busy" || !strings.Contains(got.Message, "Another update") {
		t.Fatalf("expected busy passthrough, got %#v", got)
	}
	if *scheduled != 0 {
		t.Fatalf("startDetached must not run while another update holds the lock, ran %d times", *scheduled)
	}
}

func TestUpdateBootstrapScheduleFailureKeepsJSONContract(t *testing.T) {
	bootstrapManifestDir(t)
	scheduled := restoreUpdateSeams(t, &installupdate.CheckResult{
		Status: "available", Distribution: "npm-bootstrap", Current: "1.2.3", Latest: "1.3.0",
		Message: "GitCode CLI 1.3.0 is available.",
	}, nil)
	startDetached = func(*installupdate.Manifest, bool) error {
		*scheduled += 1
		return errors.New("exec: node: executable file not found")
	}
	cmd := NewCmdUpdate(cmdutil.TestFactory())
	cmd.SilenceUsage = true
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("a failed schedule must exit non-zero")
	}
	var got result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "error" || !strings.Contains(got.Message, "executable file not found") {
		t.Fatalf("schedule failure must still emit a JSON error object, got %#v", got)
	}
}

func restoreUpdateSeams(t *testing.T, check *installupdate.CheckResult, checkErr error) *int {
	t.Cleanup(func() {
		checkNow = installupdate.CheckNow
		startDetached = installupdate.StartDetached
	})
	scheduled := new(int)
	checkNow = func(*installupdate.Manifest) (*installupdate.CheckResult, error) {
		return check, checkErr
	}
	startDetached = func(*installupdate.Manifest, bool) error {
		*scheduled += 1
		return nil
	}
	return scheduled
}

func bootstrapManifestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GITCODE_CLI_BINARY", filepath.Join(dir, "gc"))
	manifest := `{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":"` + filepath.ToSlash(dir) + `","helper":"` + filepath.ToSlash(filepath.Join(dir, "helper.js")) + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}
