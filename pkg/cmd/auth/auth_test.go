package auth

import (
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdAuthIncludesGitCredentialSubcommand(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdAuth(f)

	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "git-credential" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("auth command does not register the git-credential subcommand")
	}
}

func TestNewCmdAuthHelpDocumentsConfigDir(t *testing.T) {
	// GC_CONFIG_DIR is the multi-account isolation switch; it must be
	// discoverable from gc auth --help.
	f := cmdutil.TestFactory()
	cmd := NewCmdAuth(f)
	if !strings.Contains(cmd.Long, "GC_CONFIG_DIR") {
		t.Errorf("auth Long does not mention GC_CONFIG_DIR: %q", cmd.Long)
	}
}
