// Package comments implements the discussions comments subcommand group
// (organization scope).
package comments

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/comments/delete"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/comments/list"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/comments/replies"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdComments creates the discussions comments subcommand group for
// organization-scoped discussion comments and replies.
func NewCmdComments(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments <command>",
		Short: "Manage organization discussion comments and replies",
		Long: heredoc.Doc(`
			Manage comments and comment replies on a GitCode organization
			discussion via the v5 API.
		`),
		Example: heredoc.Doc(`
			# List comments on an org discussion
			$ gc discussions comments list 42 --org my-org

			# Delete a comment
			$ gc discussions comments delete 42 <comment-id> --org my-org

			# List replies to a comment
			$ gc discussions comments replies 42 <comment-id> --org my-org
		`),
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(replies.NewCmdReplies(f, nil))
	cmd.AddCommand(delete.NewCmdDelete(f, nil))

	return cmd
}
