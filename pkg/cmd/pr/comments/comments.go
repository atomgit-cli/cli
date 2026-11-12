// Package comments implements the pr comments command
package comments

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type CommentsOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Arguments
	Repository string
	Number     int

	// Flags
	Limit int
	JSON  bool
}

// NewCmdComments creates the comments command
func NewCmdComments(f *cmdutil.Factory, runF func(*CommentsOptions) error) *cobra.Command {
	opts := &CommentsOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "comments <number>",
		Short: "List comments on a pull request",
		Long: heredoc.Doc(`
			List comments on a pull request in a GitCode repository.

			Displays comment ID, discussion ID, author, creation time, and content.
			Use the discussion ID with 'gc pr reply --discussion'.
		`),
		Example: heredoc.Doc(`
			# List all comments on a PR
			$ gc pr comments 123 -R owner/repo

			# List latest 5 comments
			$ gc pr comments 123 --limit 5 -R owner/repo

			# List comments as JSON
			$ gc pr comments 123 -R owner/repo --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return cmdutil.NewUsageError(fmt.Sprintf("invalid PR number: %s", args[0]))
			}
			opts.Number = number

			if runF != nil {
				return runF(opts)
			}
			return commentsRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 0, "Maximum number of comments to list (0 = all)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func commentsRun(opts *CommentsOptions) error {
	cs := opts.IO.ColorScheme()

	// Validate repository input before auth so usage errors are not masked by
	// missing local credentials.
	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := parseRepo(repository)
	if err != nil {
		return err
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	// List PR comments
	comments, err := api.ListPRComments(client, owner, repo, opts.Number)
	if err != nil {
		return fmt.Errorf("failed to list comments: %w", err)
	}

	// Apply limit
	if opts.Limit > 0 && len(comments) > opts.Limit {
		// Show the latest comments (from the end)
		comments = comments[len(comments)-opts.Limit:]
	}

	// Output
	if opts.JSON {
		if comments == nil {
			comments = []api.PRComment{}
		}
		return cmdutil.WriteJSON(opts.IO.Out, comments)
	}

	if len(comments) == 0 {
		fmt.Fprintf(opts.IO.Out, "No comments on PR #%d\n", opts.Number)
		return nil
	}

	fmt.Fprintf(opts.IO.Out, "\nComments on PR #%d (%d total):\n\n", opts.Number, len(comments))

	for i, comment := range comments {
		// Format ID
		commentID := cmdutil.FormatAPIID(comment.ID)

		// Format author
		author := "unknown"
		if comment.User != nil {
			author = comment.User.Login
		}

		// Format time
		timeStr := formatTime(comment.CreatedAt)

		// Print comment header
		fmt.Fprintf(opts.IO.Out, "%s) ID: %s\n", cs.Gray(fmt.Sprintf("#%d", i+1)), cs.Cyan(commentID))

		// Print discussion ID if available (for reply)
		if comment.DiscussionID != "" {
			fmt.Fprintf(opts.IO.Out, "   Discussion ID: %s\n", cs.Yellow(comment.DiscussionID))
		}

		fmt.Fprintf(opts.IO.Out, "   Author: %s at %s\n", cs.Bold(author), timeStr)

		// Print comment type if available
		if comment.CommentType != "" {
			fmt.Fprintf(opts.IO.Out, "   Type: %s", comment.CommentType)
			if comment.Resolved {
				fmt.Fprintf(opts.IO.Out, " %s", cs.Green("[resolved]"))
			}
			fmt.Fprintln(opts.IO.Out)
		}

		// Print file location if available (inline comment)
		if comment.DiffFile != "" {
			positionStr := ""
			if comment.DiffPosition != nil {
				positionStr = fmt.Sprintf(" (position %v)", comment.DiffPosition)
			}
			fmt.Fprintf(opts.IO.Out, "   File: %s%s\n", cs.Magenta(comment.DiffFile), positionStr)
		}

		// Print body
		if comment.Body != "" {
			fmt.Fprintf(opts.IO.Out, "\n")
			// Indent body lines
			bodyLines := strings.Split(comment.Body, "\n")
			for _, line := range bodyLines {
				fmt.Fprintf(opts.IO.Out, "   %s\n", line)
			}
		}

		// Print nested replies (the API embeds each discussion's replies in
		// the parent comment's reply array; replies carry no discussion_id
		// of their own — the parent owns it).
		if len(comment.Reply) > 0 {
			fmt.Fprintf(opts.IO.Out, "   Replies (%d):\n", len(comment.Reply))
			for _, reply := range comment.Reply {
				replyAuthor := "unknown"
				if reply.User != nil {
					replyAuthor = reply.User.Login
				}
				replyID := cmdutil.FormatAPIID(reply.ID)
				fmt.Fprintf(opts.IO.Out, "     ↳ ID: %s, %s at %s\n", cs.Cyan(replyID), cs.Bold(replyAuthor), formatTime(reply.CreatedAt))
				for _, line := range strings.Split(reply.Body, "\n") {
					fmt.Fprintf(opts.IO.Out, "       %s\n", line)
				}
			}
		}

		fmt.Fprintf(opts.IO.Out, "\n")
	}

	return nil
}

func formatTime(t api.FlexibleTime) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("2006-01-02 15:04")
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}
