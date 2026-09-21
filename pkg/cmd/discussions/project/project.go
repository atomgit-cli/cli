// Package project implements the discussions project subcommand group.
package project

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/comments"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/create"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/delete"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/edit"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/list"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/project/view"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdProject creates the discussions project subcommand group for
// repository (project-level) discussions.
func NewCmdProject(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command>",
		Short: "Manage repository discussions",
		Long: heredoc.Doc(`
			Work with GitCode repository (project-level) discussions (discuss).

			These are discussion threads scoped to a repository, accessed via the
			GitCode v5 API (GET /api/v5/repos/{owner}/{repo}/discuss).
		`),
		Example: heredoc.Doc(`
			# List discussions in a repository
			$ gc discussions project list -R owner/repo

			# Create a discussion
			$ gc discussions project create -R owner/repo --title "New idea" --category "Ideas" --body "Description"

			# View a discussion
			$ gc discussions project view 42 -R owner/repo

			# Edit a discussion
			$ gc discussions project edit 42 -R owner/repo --title "Updated title"

			# Delete a discussion
			$ gc discussions project delete 42 -R owner/repo

			# Output as JSON
			$ gc discussions project list -R owner/repo --json
		`),
	}

	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(view.NewCmdView(f, nil))
	cmd.AddCommand(create.NewCmdCreate(f, nil))
	cmd.AddCommand(edit.NewCmdEdit(f, nil))
	cmd.AddCommand(comments.NewCmdComments(f))
	cmd.AddCommand(delete.NewCmdDelete(f, nil))

	return cmd
}
