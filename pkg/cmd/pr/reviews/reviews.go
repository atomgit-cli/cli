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
	renderReviewStates(opts.IO, states, opts.Number)
	return nil
}

// renderReviewStates writes the text output grouped by review role.
func renderReviewStates(io *iostreams.IOStreams, states *api.PRReviewStates, number int) {
	cs := io.ColorScheme()
	if len(states.ApprovalReviewers) == 0 && len(states.Testers) == 0 && len(states.Assignees) == 0 {
		fmt.Fprintf(io.Out, "No review states on PR #%d\n", number)
		return
	}

	fmt.Fprintf(io.Out, "\nReview states on PR #%d:\n\n", number)
	renderApprovalReviewers(io, cs, states.ApprovalReviewers)
	fmt.Fprintln(io.Out)
	renderTesters(io, cs, states.Testers)
	fmt.Fprintln(io.Out)
	renderAssignees(io, cs, states.Assignees)
}

// renderApprovalReviewers writes the approval reviewer group with the
// accept_status enum mapped to display text and state-colored icons.
func renderApprovalReviewers(io *iostreams.IOStreams, cs *iostreams.ColorScheme, reviewers []*api.PRReviewer) {
	fmt.Fprintf(io.Out, "Approval reviewers (%d):\n", len(reviewers))
	if len(reviewers) == 0 {
		fmt.Fprintln(io.Out, "  (none)")
		return
	}
	for _, reviewer := range reviewers {
		line := fmt.Sprintf("  %s %s (%s", reviewIcon(cs, reviewer.AcceptStatus),
			reviewer.Login, acceptStatusText(reviewer.AcceptStatus))
		if reviewer.CodeOwner {
			line += ", code owner"
		}
		line += ")\n"
		fmt.Fprint(io.Out, line)
	}
}

// renderTesters writes the tester group; testers carry an accept boolean
// rather than the accept_status enum.
func renderTesters(io *iostreams.IOStreams, cs *iostreams.ColorScheme, testers []*api.PRReviewer) {
	fmt.Fprintf(io.Out, "Testers (%d):\n", len(testers))
	if len(testers) == 0 {
		fmt.Fprintln(io.Out, "  (none)")
		return
	}
	for _, tester := range testers {
		state := "pending"
		icon := cs.Gray("·")
		if tester.Accept {
			state = "accepted"
			icon = cs.Green("✓")
		}
		fmt.Fprintf(io.Out, "  %s %s (%s)\n", icon, tester.Login, state)
	}
}

// renderAssignees writes the assignee group; assignees show the login only
// in text mode (their accept state stays available in --json), so no
// state icon is drawn for this group.
func renderAssignees(io *iostreams.IOStreams, _ *iostreams.ColorScheme, assignees []*api.PRReviewer) {
	fmt.Fprintf(io.Out, "Assignees (%d):\n", len(assignees))
	if len(assignees) == 0 {
		fmt.Fprintln(io.Out, "  (none)")
		return
	}
	for _, assignee := range assignees {
		fmt.Fprintf(io.Out, "  %s\n", assignee.Login)
	}
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

// reviewIcon returns a state-colored icon for an accept_status value.
func reviewIcon(cs *iostreams.ColorScheme, status string) string {
	switch status {
	case "true":
		return cs.Green("✓")
	case "reject":
		return cs.FailureIcon()
	default:
		return cs.Gray("·")
	}
}
