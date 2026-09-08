// Package comments implements the discussions project comments subcommand
// group (repository scope).
package comments

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/comments/create"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/comments/list"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/comments/replies"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdComments creates the discussions project comments subcommand group
// for repository (project-level) discussion comments and replies.
func NewCmdComments(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments <command>",
		Short: "Manage repository discussion comments and replies",
		Long: heredoc.Doc(`
			Manage comments and comment replies on a GitCode repository
			(project-level) discussion via the v5 API.
		`),
		Example: heredoc.Doc(`
			# List comments on a repo discussion
			$ gc discussions project comments list 42 -R owner/repo

			# Create a comment
			$ gc discussions project comments create 42 -R owner/repo --body "Great idea!"

			# List replies to a comment
			$ gc discussions project comments replies 42 <comment-id> -R owner/repo
		`),
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(replies.NewCmdReplies(f, nil))
	cmd.AddCommand(create.NewCmdCreate(f, nil))

	return cmd
}
