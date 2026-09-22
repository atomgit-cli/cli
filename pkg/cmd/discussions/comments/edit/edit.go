// Package edit implements the discussions comments edit command (org scope).
package edit

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/render"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// EditOptions configures the discussions comments edit command (org scope).
type EditOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)

	Org       string
	Number    int
	CommentID string

	Body     string
	BodyFile string

	JSON bool
}

// NewCmdEdit creates the discussions comments edit command (org scope).
func NewCmdEdit(f *cmdutil.Factory, runF func(*EditOptions) error) *cobra.Command {
	opts := &EditOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "edit <number> <comment-id>",
		Short: "Edit a comment on an organization discussion",
		Long: heredoc.Doc(`
			Edit a comment on a GitCode organization discussion via the v5 API
			(PUT /api/v5/orgs/{org}/discuss/{number}/comment/{id}).

			The comment body is required and can be provided with --body or
			--body-file (use - for stdin). The content is scanned for secrets
			before submission.
		`),
		Example: heredoc.Doc(`
			# Edit a comment on discussion #42
			$ gc discussions comments edit 42 <comment-id> --org my-org --body "Updated text"

			# Read body from a file
			$ gc discussions comments edit 42 <comment-id> --org my-org --body-file comment.md

			# Read body from stdin
			$ echo "Updated text" | gc discussions comments edit 42 <comment-id> --org my-org --body-file -

			# Output as JSON
			$ gc discussions comments edit 42 <comment-id> --org my-org --body "Updated text" --json
		`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return cmdutil.NewUsageError(fmt.Sprintf("invalid discussion number: %s", args[0]))
			}
			opts.Number = number
			opts.CommentID = args[1]
			if runF != nil {
				return runF(opts)
			}
			return editRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Org, "org", "", "Organization path (required)")
	cmd.MarkFlagRequired("org")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "New comment body (required)")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use - for stdin)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func editRun(opts *EditOptions) error {
	if opts.Org == "" {
		return cmdutil.NewUsageError("--org is required")
	}
	if opts.Number < 1 {
		return cmdutil.NewUsageError("discussion number must be a positive integer")
	}
	if opts.CommentID == "" {
		return cmdutil.NewUsageError("comment-id is required")
	}

	body, err := cmdutil.ReadBody(opts.Body, opts.BodyFile, opts.IO.In)
	if err != nil {
		return err
	}
	if body == "" {
		return cmdutil.NewUsageError("--body or --body-file is required")
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	c, err := api.UpdateOrgDiscussionComment(client, opts.Org, opts.Number, opts.CommentID, &api.UpdateOrgDiscussionCommentOptions{
		MdContent: body,
	})
	if err != nil {
		return fmt.Errorf("failed to edit discussion comment: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, c)
	}

	render.PrintComments(opts.IO, []*api.DiscussionComment{c})
	return nil
}
