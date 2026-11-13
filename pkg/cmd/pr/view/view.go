// Package view implements the pr view command
package view

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/browser"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr/render"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
)

type ViewOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Arguments
	Repository string
	Number     int

	// Flags
	Web        bool
	Comments   bool
	JSON       bool
	TimeFormat string
}

// NewCmdView creates the view command
func NewCmdView(f *cmdutil.Factory, runF func(*ViewOptions) error) *cobra.Command {
	opts := &ViewOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "View a pull request",
		Long: heredoc.Doc(`
			View a pull request in a GitCode repository.

			When enrichment (files/commits stats) or comments retrieval fails,
			the PR details are still output and a non-zero exit code is returned:
			401/403 -> 4 (ExitAuth), 404 -> 3 (ExitNotFound), 409 -> 5 (ExitConflict),
			other -> 1 (ExitError).
		`),
		Example: heredoc.Doc(`
			# View a PR
			$ gc pr view 123 -R owner/repo

			# View PR in browser
			$ gc pr view 123 -R owner/repo --web

			# View PR with relative time display
			$ gc pr view 123 -R owner/repo --time-format relative

			# Output as JSON
			$ gc pr view 123 -R owner/repo --json

			# View comments and output as JSON
			$ gc pr view 123 -R owner/repo --comments --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return cmdutil.NewUsageError(fmt.Sprintf("invalid PR number: %s", args[0]))
			}
			opts.Number = number
			if _, err := parseTimeFormat(opts.TimeFormat); err != nil {
				return err
			}

			if runF != nil {
				return runF(opts)
			}
			return viewRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open in browser")
	cmd.Flags().BoolVarP(&opts.Comments, "comments", "c", false, "View comments")
	cmd.Flags().StringVar(&opts.TimeFormat, "time-format", "absolute", "Time format for dates (absolute/relative)")
	cmdutil.SetFlagEnumOrWarn(cmd, "time-format", "absolute", "relative")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func viewRun(opts *ViewOptions) error {
	cs := opts.IO.ColorScheme()

	if opts.JSON && opts.Web {
		return cmdutil.NewUsageError("cannot use --json with --web")
	}

	timeFormat, err := parseTimeFormat(opts.TimeFormat)
	if err != nil {
		return err
	}
	now := time.Now()

	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
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

	// Get PR
	pr, err := api.GetPullRequest(client, owner, repo, opts.Number)
	if err != nil {
		return cmdutil.WrapNotFound(err, "PR #%d not found in %s/%s", opts.Number, owner, repo)
	}

	// Open in browser if --web flag is set
	if opts.Web {
		fmt.Fprintf(opts.IO.Out, "Opening %s in your browser.\n", pr.HTMLURL)
		if err := browser.Open(pr.HTMLURL); err != nil {
			if opts.IO.IsStdoutTTY() {
				return err
			}
			fmt.Fprintf(opts.IO.ErrOut, "Failed to open browser: %v\n", err)
		}
		return nil
	}
	// Enrichment failure is non-fatal for rendering but signals incomplete data
	// so agents cannot mistake exit code 0 for full PR data.
	enrichErr := enrichPRStats(client, owner, repo, pr)
	if enrichErr != nil {
		fmt.Fprintf(opts.IO.ErrOut, "%s Failed to enrich PR stats: %v\n", cs.Yellow("!"), enrichErr)
	}

	// Review states feed the text-mode summary line; --json deliberately
	// does not fetch them (the dedicated `pr reviews --json` carries the
	// structured states — contract decision recorded in issue 652).
	var reviewStates *api.PRReviewStates
	if !opts.JSON {
		states, serr := api.GetPRReviewStates(client, owner, repo, opts.Number)
		if serr != nil {
			fmt.Fprintf(opts.IO.ErrOut, "%s Failed to get review states: %v\n", cs.Yellow("!"), serr)
			enrichErr = errors.Join(enrichErr, fmt.Errorf("failed to get review states: %w", serr))
		} else {
			reviewStates = states
		}
	}

	if opts.JSON {
		if opts.Comments {
			comments, err := api.ListPRComments(client, owner, repo, opts.Number)
			if err != nil {
				fmt.Fprintf(opts.IO.ErrOut, "%s Failed to get comments: %v\n", cs.Yellow("!"), err)
				commentErr := fmt.Errorf("failed to get comments: %w", err)
				enrichErr = errors.Join(enrichErr, commentErr)
			}
			if err := cmdutil.WriteJSON(opts.IO.Out, map[string]interface{}{
				"pull_request": pr,
				"comments":     comments,
			}); err != nil {
				return err
			}
		} else if err := cmdutil.WriteJSON(opts.IO.Out, pr); err != nil {
			return err
		}
		if enrichErr != nil {
			return fmt.Errorf("PR data incomplete: %w", enrichErr)
		}
		return nil
	}

	if err := renderPRDetails(opts.IO.Out, cs, pr, reviewStates, timeFormat, now); err != nil {
		return err
	}

	if opts.Comments {
		comments, err := api.ListPRComments(client, owner, repo, opts.Number)
		if err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "%s Failed to get comments: %v\n", cs.Yellow("!"), err)
			commentErr := fmt.Errorf("failed to get comments: %w", err)
			enrichErr = errors.Join(enrichErr, commentErr)
		} else if err := renderPRComments(opts.IO.Out, cs, comments, timeFormat, now); err != nil {
			return err
		}
	}

	if enrichErr != nil {
		return fmt.Errorf("PR data incomplete: %w", enrichErr)
	}
	return nil
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}

func parseTimeFormat(value string) (output.TimeFormat, error) {
	format, err := output.ParseTimeFormat(strings.ToLower(strings.TrimSpace(value)))
	if err != nil {
		return "", cmdutil.NewUsageError(err.Error())
	}
	return format, nil
}

func formatViewTime(t time.Time, format output.TimeFormat, now time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	if format != output.TimeFormatRelative {
		return output.FormatTime(t, format)
	}
	return formatRelativeTime(t, now)
}

func formatRelativeTime(t time.Time, now time.Time) string {
	delta := now.Sub(t)
	if delta < 0 {
		delta = -delta
		return "in " + formatDuration(delta)
	}
	return formatDuration(delta)
}

func formatDuration(delta time.Duration) string {
	switch {
	case delta < time.Minute:
		return "just now"
	case delta < time.Hour:
		mins := int(delta.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case delta < 24*time.Hour:
		hours := int(delta.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case delta < 7*24*time.Hour:
		days := int(delta.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case delta < 30*24*time.Hour:
		weeks := int(delta.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case delta < 365*24*time.Hour:
		months := int(delta.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(delta.Hours() / (24 * 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func joinLabels(labels []*api.Label) string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if label != nil && label.Name != "" {
			names = append(names, label.Name)
		}
	}
	return strings.Join(names, ", ")
}

func joinUsers(users []*api.User) string {
	names := make([]string, 0, len(users))
	for _, user := range users {
		if user != nil && user.Login != "" {
			names = append(names, user.Login)
		}
	}
	return strings.Join(names, ", ")
}

func enrichPRStats(client *api.Client, owner, repo string, pr *api.PullRequest) error {
	if pr == nil {
		return nil
	}
	var errs []error
	if pr.Additions == 0 && pr.Deletions == 0 && pr.ChangedFiles == 0 {
		files, err := api.GetPRFiles(client, owner, repo, pr.Number)
		if err != nil {
			errs = append(errs, err)
		} else if files != nil {
			pr.Additions = files.AddedLines
			pr.Deletions = files.RemoveLines
			pr.ChangedFiles = files.Count
			if pr.ChangedFiles == 0 && len(files.Diffs) > 0 {
				pr.ChangedFiles = len(files.Diffs)
			}
		}
	}
	if pr.Commits == 0 {
		commits, err := api.ListPRCommits(client, owner, repo, pr.Number)
		if err != nil {
			errs = append(errs, err)
		} else if len(commits) > 0 {
			pr.Commits = len(commits)
		}
	}
	return errors.Join(errs...)
}

func renderPRView(out io.Writer, cs *iostreams.ColorScheme, pr *api.PullRequest, comments []api.PRComment, states *api.PRReviewStates, timeFormat output.TimeFormat, now time.Time) error {
	if err := renderPRDetails(out, cs, pr, states, timeFormat, now); err != nil {
		return err
	}
	return renderPRComments(out, cs, comments, timeFormat, now)
}

func renderPRDetails(out io.Writer, cs *iostreams.ColorScheme, pr *api.PullRequest, states *api.PRReviewStates, timeFormat output.TimeFormat, now time.Time) error {
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "%s #%d\n", cs.Bold(pr.Title), pr.Number)
	if pr.Draft {
		fmt.Fprintf(out, "  State: %s (draft)\n", pr.State)
	} else if pr.IsMerged() {
		fmt.Fprintf(out, "  State: merged\n")
	} else {
		fmt.Fprintf(out, "  State: %s\n", pr.State)
	}
	if pr.User != nil {
		fmt.Fprintf(out, "  Author: %s\n", pr.User.Login)
	}
	if pr.Head != nil && pr.Base != nil {
		fmt.Fprintf(out, "  Branch: %s -> %s\n", pr.Head.Ref, pr.Base.Ref)
	}
	fmt.Fprintf(out, "  Created: %s\n", formatViewTime(pr.CreatedAt.Time, timeFormat, now))
	fmt.Fprintf(out, "  Updated: %s\n", formatViewTime(pr.UpdatedAt.Time, timeFormat, now))
	fmt.Fprintf(out, "  Additions: +%d  Deletions: -%d  Files: %d\n", pr.Additions, pr.Deletions, pr.ChangedFiles)
	fmt.Fprintf(out, "  Commits: %d  Comments: %d\n", pr.Commits, pr.Comments)
	if len(pr.Assignees) > 0 {
		fmt.Fprintf(out, "  Assignees: %s\n", joinUsers(pr.Assignees))
	}
	if len(pr.Reviewers) > 0 {
		fmt.Fprintf(out, "  Reviewers: %s\n", joinUsers(pr.Reviewers))
	}
	if summary := reviewSummaryLine(states); summary != "" {
		fmt.Fprintf(out, "  %s\n", summary)
	}
	if len(pr.Labels) > 0 {
		fmt.Fprintf(out, "  Labels: %s\n", joinLabels(pr.Labels))
	}
	if pr.Milestone != nil && pr.Milestone.Title != "" {
		fmt.Fprintf(out, "  Milestone: %s\n", pr.Milestone.Title)
	}
	fmt.Fprintf(out, "\n")
	if pr.Body != "" {
		fmt.Fprintf(out, "%s\n", pr.Body)
		fmt.Fprintf(out, "\n")
	}
	fmt.Fprintf(out, "  %s\n", pr.HTMLURL)

	return nil
}

func renderPRComments(out io.Writer, cs *iostreams.ColorScheme, comments []api.PRComment, timeFormat output.TimeFormat, now time.Time) error {
	if len(comments) == 0 {
		fmt.Fprintf(out, "\n--- No comments ---\n\n")
		return nil
	}

	replyTime := func(t api.FlexibleTime) string {
		return formatViewTime(t.Time, timeFormat, now)
	}

	fmt.Fprintf(out, "\n--- Comments (%d) ---\n", len(comments))
	for _, c := range comments {
		author := "unknown"
		if c.User != nil && c.User.Login != "" {
			author = c.User.Login
		}
		fmt.Fprintf(out, "\n%s at %s", author, formatViewTime(c.CreatedAt.Time, timeFormat, now))
		if c.CommentType != "" {
			fmt.Fprintf(out, " [%s]", c.CommentType)
		}
		if c.DiffFile != "" {
			fmt.Fprintf(out, " (%s)", c.DiffFile)
		}
		fmt.Fprintf(out, ":\n%s\n", c.Body)
		// Nested replies use the same rendering as the comments command
		// (issue 652: one dataset, one rendering contract).
		render.ReplyBlock(out, cs, c.Reply, "  ", replyTime)
	}

	fmt.Fprintf(out, "\n")
	return nil
}

// reviewSummaryLine builds the one-line review state summary for the PR
// details header, e.g. "Review: 1 approved, 1 rejected · Testers: 1 accepted".
// The whole line is empty when neither approval reviewers nor testers
// exist. Detailed grouping stays in `pr reviews`.
func reviewSummaryLine(states *api.PRReviewStates) string {
	if states == nil {
		return ""
	}
	var parts []string
	if len(states.ApprovalReviewers) > 0 {
		parts = append(parts, "Review: "+joinReviewerCounts(states.ApprovalReviewers))
	}
	if len(states.Testers) > 0 {
		parts = append(parts, "Testers: "+joinTesterCounts(states.Testers))
	}
	return strings.Join(parts, " · ")
}

// joinReviewerCounts summarizes approval reviewers as "1 approved, 2
// pending"; statuses with a zero count are omitted.
func joinReviewerCounts(reviewers []*api.PRReviewer) string {
	approved, rejected, pending := 0, 0, 0
	for _, reviewer := range reviewers {
		switch render.AcceptStatusText(reviewer.AcceptStatus) {
		case "approved":
			approved++
		case "rejected":
			rejected++
		default:
			pending++
		}
	}
	counts := make([]string, 0, 3)
	for _, c := range []struct {
		label string
		n     int
	}{{"approved", approved}, {"rejected", rejected}, {"pending", pending}} {
		if c.n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", c.n, c.label))
		}
	}
	return strings.Join(counts, ", ")
}

// joinTesterCounts summarizes testers as "1 accepted, 1 pending";
// statuses with a zero count are omitted.
func joinTesterCounts(testers []*api.PRReviewer) string {
	accepted, pending := 0, 0
	for _, tester := range testers {
		if tester.Accept {
			accepted++
		} else {
			pending++
		}
	}
	counts := make([]string, 0, 2)
	if accepted > 0 {
		counts = append(counts, fmt.Sprintf("%d accepted", accepted))
	}
	if pending > 0 {
		counts = append(counts, fmt.Sprintf("%d pending", pending))
	}
	return strings.Join(counts, ", ")
}
