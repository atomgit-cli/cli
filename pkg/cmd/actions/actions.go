// Package actions implements the actions command.
package actions

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	artifactcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/artifact"
	jobcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/job"
	plugincmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/plugin"
	runcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/run"
	runnercmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/runner"
	runnergroupcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/runner-group"
	runnersetcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/runner-set"
	settingcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/setting"
	workflowcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/workflow"
	yamlcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions/yaml"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdActions creates the actions command.
func NewCmdActions(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions <command>",
		Short: "Manage GitCode Actions",
		Long: heredoc.Doc(`
			Work with GitCode Actions: inspect pipeline runs and workflow jobs,
			or enable and disable repository Actions.

			Changing repository Actions permissions is a dangerous operation and
			requires confirmation unless --yes is provided.
		`),
		Example: heredoc.Doc(`
			# List recent pipeline runs
			$ gc actions run list -R owner/repo

			# Filter runs by status
			$ gc actions run list -R owner/repo --status FAILED

			# Output runs as JSON
			$ gc actions run list -R owner/repo --json

			# List jobs of a pipeline run
			$ gc actions job list <run-id> -R owner/repo
		`),
		Annotations: map[string]string{
			"IsCore": "true",
		},
	}

	cmd.AddCommand(runcmd.NewCmdRun(f))
	cmd.AddCommand(jobcmd.NewCmdJob(f))
	cmd.AddCommand(artifactcmd.NewCmdArtifact(f))
	cmd.AddCommand(plugincmd.NewCmdPlugin(f))
	cmd.AddCommand(runnergroupcmd.NewCmdRunnerGroup(f))
	cmd.AddCommand(runnercmd.NewCmdRunner(f))
	cmd.AddCommand(runnersetcmd.NewCmdRunnerSet(f))
	cmd.AddCommand(settingcmd.NewCmdSetting(f, nil))
	cmd.AddCommand(yamlcmd.NewCmdYaml(f))
	cmd.AddCommand(workflowcmd.NewCmdWorkflow(f))

	return cmd
}
