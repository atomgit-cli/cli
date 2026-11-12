// Package pr implements the pr command
package pr

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/checkout"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/close"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/comment"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/comments"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/create"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/diff"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/edit"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/issues"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/label"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/list"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/merge"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/ready"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/reopen"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/reply"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/review"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/reviews"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/sync"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/test"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/view"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// NewCmdPR creates the pr command
func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <command>",
		Short: "Manage pull requests",
		Long: heredoc.Doc(`
			Work with GitCode pull requests.

			A pull request is a proposal to merge changes from one branch into another.
		`),
		Example: heredoc.Doc(`
			# Create a new pull request
			$ gc pr create -R owner/repo --title "Feature" --body "Description"

			# List pull requests
			$ gc pr list -R owner/repo

			# View a pull request
			$ gc pr view 123 -R owner/repo

			# Review a pull request
			$ gc pr review 123 -R owner/repo --approve
		`),
		Annotations: map[string]string{
			"IsCore":                "true",
			cmdutil.TopicAnnotation: "pull-requests",
		},
	}

	cmd.AddCommand(create.NewCmdCreate(f, nil))
	cmd.AddCommand(list.NewCmdList(f, nil))
	cmd.AddCommand(view.NewCmdView(f, nil))
	cmd.AddCommand(comments.NewCmdComments(f, nil))
	cmd.AddCommand(comment.NewCmdComment(f, nil))
	cmd.AddCommand(reply.NewCmdReply(f, nil))
	cmd.AddCommand(reviews.NewCmdReviews(f, nil))
	cmd.AddCommand(checkout.NewCmdCheckout(f, nil))
	cmd.AddCommand(merge.NewCmdMerge(f, nil))
	cmd.AddCommand(close.NewCmdClose(f, nil))
	cmd.AddCommand(reopen.NewCmdReopen(f, nil))
	cmd.AddCommand(review.NewCmdReview(f, nil))
	cmd.AddCommand(diff.NewCmdDiff(f, nil))
	cmd.AddCommand(label.NewCmdLabel(f, nil))
	cmd.AddCommand(ready.NewCmdReady(f, nil))
	cmd.AddCommand(edit.NewCmdEdit(f, nil))
	cmd.AddCommand(issues.NewCmdIssues(f, nil))
	cmd.AddCommand(sync.NewCmdSync(f, nil))
	cmd.AddCommand(test.NewCmdTest(f, nil))

	return cmd
}
