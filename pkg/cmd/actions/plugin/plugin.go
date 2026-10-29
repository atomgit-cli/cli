// Package plugin implements the actions plugin command.
package plugin

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	listcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/plugin/list"
	viewcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/plugin/view"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdPlugin creates the actions plugin command.
func NewCmdPlugin(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin <command>",
		Short: "Query official Actions plugins",
		Long: heredoc.Doc(`
			Query the official GitCode Actions plugin directory and usage docs.

			Plugins are published by GitCode and provide reusable workflow steps.
			Use the list subcommand to discover available plugins and the view
			subcommand to inspect a plugin's versions and README.
		`),
		Example: heredoc.Doc(`
			# List available plugins for a repository
			$ gc actions plugin list -R owner/repo

			# View a plugin's README and versions
			$ gc actions plugin view checkout -R owner/repo

			# Output as JSON
			$ gc actions plugin list -R owner/repo --json
		`),
	}

	cmd.AddCommand(listcmd.NewCmdList(f, nil))
	cmd.AddCommand(viewcmd.NewCmdView(f, nil))

	return cmd
}
