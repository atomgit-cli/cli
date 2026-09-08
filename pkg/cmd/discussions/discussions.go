// Package discussions implements the discussions command.
package discussions

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/comments"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/delete"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/list"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/view"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdDiscussions creates the discussions command.
func NewCmdDiscussions(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discussions <command>",
		Short: "Manage organization discussions",
		Long: heredoc.Doc(`
			Work with GitCode organization discussions (discuss).

			Discussions are organization-level conversation threads for ideas,
			Q&A, and announcements. The CLI exposes list, view, and delete
			operations via the GitCode v5 API.
		`),
		Example: heredoc.Doc(`
			# List discussions in an organization
			$ gc discussions list --org my-org

			# View a discussion
			$ gc discussions view 42 --org my-org

			# Delete a discussion
			$ gc discussions delete 42 --org my-org

			# Output as JSON
			$ gc discussions list --org my-org --json
		`),
		Annotations: map[string]string{
			"IsCore":                "true",
			cmdutil.TopicAnnotation: "discussions",
		},
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(view.NewCmdView(f, nil))
	cmd.AddCommand(delete.NewCmdDelete(f, nil))
	cmd.AddCommand(comments.NewCmdComments(f))
	cmd.AddCommand(project.NewCmdProject(f))

	return cmd
}
