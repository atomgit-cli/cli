// Package reviews implements the pr reviews command.
package reviews

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

type ReviewsOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Arguments
	Repository string
	Number     int

	// Flags
	JSON bool
}

// NewCmdReviews creates the pr reviews command.
func NewCmdReviews(f *cmdutil.Factory, runF func(*ReviewsOptions) error) *cobra.Command {
	opts := &ReviewsOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "reviews <number>",
		Short: "View review states on a pull request",
		Long: heredoc.Doc(`
			View the review states on a pull request: approval reviewers
			(with their accept_status), testers, and assignees.

			The v5 API has no dedicated reviews list endpoint; these states
			are embedded in the PR detail response. accept_status values:
			true (approved), reject (rejected), optional (pending).
			Per-reviewer timestamps are not provided by the endpoint.
		`),
		Example: heredoc.Doc(`
			# View review states on a PR
			$ gc pr reviews 123 -R owner/repo

			# Machine-readable output
			$ gc pr reviews 123 -R owner/repo --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil || number < 1 {
				return cmdutil.NewUsageError(fmt.Sprintf("invalid PR number: %s", args[0]))
			}
			opts.Number = number

			if runF != nil {
				return runF(opts)
			}
			return reviewsRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func reviewsRun(opts *ReviewsOptions) error {
	cs := opts.IO.ColorScheme()

	// Validate the PR number before repository resolution and any network
	// activity so usage errors are never masked by auth or API failures.
	if opts.Number < 1 {
		return cmdutil.NewUsageError(fmt.Sprintf("invalid PR number: %d", opts.Number))
	}

	// Validate repository input before auth so usage errors are not masked
	// by missing local credentials.
	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := cmdutil.ParseRepo(repository)
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

	states, err := api.GetPRReviewStates(client, owner, repo, opts.Number)
	if err != nil {
		return fmt.Errorf("failed to get review states: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, states)
	}

	if len(states.ApprovalReviewers) == 0 && len(states.Testers) == 0 && len(states.Assignees) == 0 {
		fmt.Fprintf(opts.IO.Out, "No review states on PR #%d\n", opts.Number)
		return nil
	}

	fmt.Fprintf(opts.IO.Out, "\nReview states on PR #%d:\n\n", opts.Number)
	fmt.Fprintf(opts.IO.Out, "Approval reviewers (%d):\n", len(states.ApprovalReviewers))
	if len(states.ApprovalReviewers) == 0 {
		fmt.Fprintln(opts.IO.Out, "  (none)")
	}
	for _, reviewer := range states.ApprovalReviewers {
		line := fmt.Sprintf("  %s %s (%s", cs.Green("✓"), reviewer.Login, acceptStatusText(reviewer.AcceptStatus))
		if reviewer.CodeOwner {
			line += ", code owner"
		}
		line += ")\n"
		fmt.Fprint(opts.IO.Out, line)
	}

	fmt.Fprintf(opts.IO.Out, "\nTesters (%d):\n", len(states.Testers))
	if len(states.Testers) == 0 {
		fmt.Fprintln(opts.IO.Out, "  (none)")
	}
	for _, tester := range states.Testers {
		state := "pending"
		if tester.Accept {
			state = "accepted"
		}
		fmt.Fprintf(opts.IO.Out, "  %s %s (%s)\n", cs.Green("✓"), tester.Login, state)
	}

	fmt.Fprintf(opts.IO.Out, "\nAssignees (%d):\n", len(states.Assignees))
	if len(states.Assignees) == 0 {
		fmt.Fprintln(opts.IO.Out, "  (none)")
	}
	for _, assignee := range states.Assignees {
		fmt.Fprintf(opts.IO.Out, "  %s %s\n", cs.Green("✓"), assignee.Login)
	}

	return nil
}

// acceptStatusText maps the API accept_status enum to display text.
func acceptStatusText(status string) string {
	switch status {
	case "true":
		return "approved"
	case "reject":
		return "rejected"
	default:
		return "pending"
	}
}
