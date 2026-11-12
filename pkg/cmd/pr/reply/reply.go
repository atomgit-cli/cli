// Package reply implements the pr reply command
package reply

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type ReplyOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Arguments
	Repository   string
	Number       int
	DiscussionID string

	// Flags
	Body string
	JSON bool
}

// NewCmdReply creates the reply command
func NewCmdReply(f *cmdutil.Factory, runF func(*ReplyOptions) error) *cobra.Command {
	opts := &ReplyOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "reply <pr-number> --discussion <id> --body <text>",
		Short: "Reply to a PR comment discussion",
		Long: heredoc.Doc(`
			Reply to a comment discussion on a pull request.

			Use 'gc pr comments <number>' to find the DiscussionID for the comment you want to reply to.

			The output includes the Comment ID (note_id) of the created reply:
			read it back individually via
			'gc api repos/<owner>/<repo>/pulls/comments/<note_id>', or find it in
			the parent comment's reply array in 'gc pr comments'.
		`),
		Example: heredoc.Doc(`
			# Reply to a comment discussion
			$ gc pr reply 123 --discussion abc123 --body "Thanks for the feedback!" -R owner/repo

			# Reply with multi-line body
			$ gc pr reply 123 --discussion abc123 --body "Line 1
			Line 2" -R owner/repo

			# Machine-readable output with the reply's comment id
			$ gc pr reply 123 --discussion abc123 --body "ack" --json -R owner/repo
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return cmdutil.NewUsageError(fmt.Sprintf("invalid PR number: %s", args[0]))
			}
			opts.Number = number

			if opts.DiscussionID == "" {
				return cmdutil.NewUsageError("discussion ID is required. Use --discussion flag")
			}

			if opts.Body == "" {
				return cmdutil.NewUsageError("body is required. Use --body flag")
			}

			if runF != nil {
				return runF(opts)
			}
			return replyRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.DiscussionID, "discussion", "d", "", "Discussion ID to reply to")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Reply body text")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	_ = cmd.MarkFlagRequired("discussion")
	_ = cmd.MarkFlagRequired("body")

	return cmd
}

func replyRun(opts *ReplyOptions) error {
	if opts.Body != "" {
		if err := cmdutil.ScanContentForSecrets(opts.Body); err != nil {
			return err
		}
	}
	cs := opts.IO.ColorScheme()

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	// Get repository
	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := parseRepo(repository)
	if err != nil {
		return err
	}

	// Reply to comment
	replyOpts := &api.ReplyPRCommentOptions{
		Body: opts.Body,
	}

	result, err := api.ReplyPRComment(client, owner, repo, opts.Number, opts.DiscussionID, replyOpts)
	if err != nil {
		return fmt.Errorf("failed to reply to comment: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, map[string]interface{}{
			"discussion_id": opts.DiscussionID,
			"note_id":       result.NoteID,
			"body":          result.Body,
		})
	}

	// Output
	fmt.Fprintf(opts.IO.Out, "%s Replied to discussion %s\n", cs.Green("✓"), cs.Yellow(opts.DiscussionID))
	if noteID := cmdutil.FormatAPIID(result.NoteID); noteID != "" && noteID != "0" {
		fmt.Fprintf(opts.IO.Out, "  Comment ID: %s\n", cs.Cyan(noteID))
	}
	if result.ID != "" {
		fmt.Fprintf(opts.IO.Out, "  Discussion ID: %s\n", cs.Yellow(result.ID))
	}

	return nil
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}
