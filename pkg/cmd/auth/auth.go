// Package auth implements authentication commands
package auth

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/gitcredential"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/login"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/logout"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/setupgit"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/status"
	switchcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/auth/switch"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/token"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdAuth creates the auth command
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <command>",
		Short: "Authenticate with GitCode",
		Long: heredoc.Doc(`
			Authenticate with a GitCode account.

			Environment variables:
			  GC_CONFIG_DIR  Read and write configuration (including the
			      login state) from this directory instead of the default
			      ~/.config/gc. Point it at a separate directory to keep an
			      isolated login, e.g. for a second account on one machine:
			      an empty directory is "not logged in" and never falls
			      back to the default directory.
		`),
		Annotations: map[string]string{
			cmdutil.TopicAnnotation: "auth",
		},
	}

	cmd.AddCommand(login.NewCmdLogin(f, nil))
	cmd.AddCommand(logout.NewCmdLogout(f, nil))
	cmd.AddCommand(setupgit.NewCmdSetupGit(f, nil))
	cmd.AddCommand(gitcredential.NewCmdGitCredential(f, nil))
	cmd.AddCommand(status.NewCmdStatus(f, nil))
	cmd.AddCommand(switchcmd.NewCmdSwitch(f, nil))
	cmd.AddCommand(token.NewCmdToken(f, nil))

	return cmd
}
