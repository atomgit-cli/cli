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
	t.Parallel()
	for _, distribution := range []string{"pypi", "homebrew", "system-package", "npm", "archive"} {
		distribution := distribution
		message := strings.ToLower(managerMessage(distribution))
		if strings.Contains(message, "uninstall") || strings.Contains(message, "remove") {
			t.Fatalf("%s guidance mutates another channel: %s", distribution, message)
		}
	}
}
