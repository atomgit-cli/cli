// Package run implements the actions run command.
package run

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run/list"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run/rerun"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run/retry"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run/stop"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run/view"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run/watch"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdRun creates the actions run command.
func NewCmdRun(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <command>",
		Short: "Manage pipeline runs",
		Long: heredoc.Doc(`
			Inspect pipeline run records for a repository.
		`),
		Example: heredoc.Doc(`
			# List recent pipeline runs
			$ gc actions run list -R owner/repo

			# View a pipeline run detail
			$ gc actions run view <run-id> -R owner/repo

			# Watch a run until completion
			$ gc actions run watch <run-id> -R owner/repo
		`),
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(rerun.NewCmdRerun(f, nil))
	cmd.AddCommand(retry.NewCmdRetry(f, nil))
	cmd.AddCommand(stop.NewCmdStop(f, nil))
	cmd.AddCommand(view.NewCmdView(f, nil))
	cmd.AddCommand(watch.NewCmdWatch(f, nil))

	return cmd
}
