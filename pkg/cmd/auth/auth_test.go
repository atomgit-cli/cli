package auth

import (
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
