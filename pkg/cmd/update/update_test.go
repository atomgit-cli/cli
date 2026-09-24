package update

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
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
		"npm":               "npm",
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
