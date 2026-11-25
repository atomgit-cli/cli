// Package watch implements the actions run watch command.
package watch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// terminalStatuses are the run states that indicate the run has stopped.
var terminalStatuses = map[string]bool{
	"COMPLETED": true,
	"FAILED":    true,
	"CANCELED":  true,
	"IGNORED":   true,
	"PAUSED":    true,
	"SUSPEND":   true,
}

// failedStatuses are terminal states considered failures for --exit-status.
var failedStatuses = map[string]bool{
	"FAILED":   true,
	"CANCELED": true,
}

// WatchOptions configures the actions run watch command.
type WatchOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	RunID      string

	Interval    int
	Compact     bool
	ExitStatus  bool
	Wait        bool
	JSON        bool
	NonInterval bool
}

// NewCmdWatch creates the actions run watch command.
func NewCmdWatch(f *cmdutil.Factory, runF func(*WatchOptions) error) *cobra.Command {
	opts := &WatchOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "watch <run-id>",
		Short: "Watch a pipeline run until completion",
		Long: heredoc.Doc(`
			Watch a pipeline run, refreshing its status at regular intervals until
			the run reaches a terminal state (COMPLETED, FAILED, CANCELED,
			IGNORED, PAUSED, or SUSPEND).

			In a TTY, the screen is refreshed on each poll. In a non-TTY
			environment (e.g. piped output or --no-interactive), a single
			snapshot is printed and the command exits immediately; pass
			--wait to keep polling in non-TTY mode until the run reaches a
			terminal state (--exit-status then works as in a TTY).

			Use --exit-status to exit with a non-zero code when the run fails.
		`),
		Example: heredoc.Doc(`
			# Watch a run until it finishes
			$ gc actions run watch <run-id> -R owner/repo

			# Watch with a 10-second interval
			$ gc actions run watch <run-id> -R owner/repo --interval 10

			# Compact mode (only failed steps)
			$ gc actions run watch <run-id> -R owner/repo --compact

			# Exit non-zero on failure (for CI scripts)
			$ gc actions run watch <run-id> -R owner/repo --exit-status

			# Wait for completion from a script (non-interactive)
			$ gc actions run watch <run-id> -R owner/repo --wait --exit-status
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RunID = strings.TrimSpace(args[0])
			if opts.RunID == "" {
				return cmdutil.NewUsageError("run id is required")
			}
			if opts.Interval < 1 {
				return cmdutil.NewUsageError("--interval must be at least 1 second")
			}
			if runF != nil {
				return runF(opts)
			}
			return watchRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().IntVarP(&opts.Interval, "interval", "i", 3, "Refresh interval in seconds")
	cmd.Flags().BoolVar(&opts.Compact, "compact", false, "Show only failed steps")
	cmd.Flags().BoolVar(&opts.ExitStatus, "exit-status", false, "Exit with non-zero status if run failed")
	cmd.Flags().BoolVar(&opts.Wait, "wait", false, "In non-interactive mode, keep polling until the run reaches a terminal state")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func watchRun(opts *WatchOptions) error {
	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
	if err != nil {
		return err
	}

	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := cmdutil.ParseRepo(repository)
	if err != nil {
		return err
	}

	return pollLoop(opts, client, owner, repo)
}

func pollLoop(opts *WatchOptions, client *api.Client, owner, repo string) error {
	isTTY := opts.IO.IsStdoutTTY()
	interval := time.Duration(opts.Interval) * time.Second

	for {
		detail, raw, err := api.GetActionsRun(client, owner, repo, opts.RunID)
		if err != nil {
			return fmt.Errorf("failed to get pipeline run: %w", err)
		}

		if opts.JSON {
			return outputJSON(opts, raw, detail)
		}

		if isTTY {
			renderTTY(opts, detail)
		} else {
			renderSnapshot(opts, detail)
			if !terminalStatuses[detail.Status] && !opts.Wait {
				return nil
			}
		}

		if terminalStatuses[detail.Status] {
			return handleExitStatus(opts, detail.Status)
		}

		if isTTY || opts.Wait {
			time.Sleep(interval)
		}
	}
}

func outputJSON(opts *WatchOptions, raw json.RawMessage, detail *api.WorkflowRunDetail) error {
	if _, err := opts.IO.Out.Write(raw); err != nil {
		return fmt.Errorf("failed to write JSON output: %w", err)
	}
	_, err := fmt.Fprintln(opts.IO.Out)
	return err
}

func renderTTY(opts *WatchOptions, d *api.WorkflowRunDetail) {
	out := opts.IO.Out
	cs := opts.IO.ColorScheme()

	fmt.Fprintf(out, "\033[2J\033[H")
	fmt.Fprintf(out, "%s #%d  %s\n", d.WorkflowName, d.RunNumber, statusLabel(cs, d.Status))
	fmt.Fprintf(out, "  run id:    %s\n", orDash(d.WorkflowRunID))
	fmt.Fprintf(out, "  status:    %s\n", statusLabel(cs, d.Status))
	fmt.Fprintf(out, "  branch:    %s\n", orDash(d.HeadBranch))
	fmt.Fprintf(out, "  head sha:  %s\n", orDash(d.HeadSHA))
	fmt.Fprintf(out, "  started:   %s\n", formatTime(d.StartTime))

	if len(d.Stages) == 0 {
		return
	}
	fmt.Fprintf(out, "\nStages:\n")
	for _, stage := range d.Stages {
		if opts.Compact && !hasFailedJobs(stage) {
			continue
		}
		renderStage(out, cs, stage)
	}
}

func renderStage(out io.Writer, cs *iostreams.ColorScheme, stage api.WorkflowRunStage) {
	fmt.Fprintf(out, "  - %s  %s  jobs: %d\n",
		orDash(stage.Name), statusLabel(cs, stage.Status), len(stage.Jobs))
	for _, job := range stage.Jobs {
		fmt.Fprintf(out, "      - %s  %s  steps: %d\n",
			orDash(job.Name), statusLabel(cs, job.Status), len(job.Steps))
		if hasFailedSteps(job) {
			for _, step := range job.Steps {
				if step.Status == "FAILED" {
					fmt.Fprintf(out, "          ! %s  %s\n", orDash(step.Name), statusLabel(cs, step.Status))
				}
			}
		}
	}
}

func renderSnapshot(opts *WatchOptions, d *api.WorkflowRunDetail) {
	out := opts.IO.Out
	cs := opts.IO.ColorScheme()

	fmt.Fprintf(out, "%s #%d  %s\n", d.WorkflowName, d.RunNumber, statusLabel(cs, d.Status))
	fmt.Fprintf(out, "  run id:    %s\n", orDash(d.WorkflowRunID))
	fmt.Fprintf(out, "  status:    %s\n", statusLabel(cs, d.Status))
	fmt.Fprintf(out, "  started:   %s\n", formatTime(d.StartTime))
	fmt.Fprintf(out, "  ended:     %s\n", formatTime(d.EndTime))

	if len(d.Stages) == 0 {
		return
	}
	for _, stage := range d.Stages {
		if opts.Compact && !hasFailedJobs(stage) {
			continue
		}
		renderStage(opts.IO.Out, cs, stage)
	}
}

func handleExitStatus(opts *WatchOptions, status string) error {
	if opts.ExitStatus && failedStatuses[status] {
		return cmdutil.NewCLIError(cmdutil.ExitError, fmt.Sprintf("run ended with status %s", status), nil)
	}
	return nil
}

func hasFailedJobs(stage api.WorkflowRunStage) bool {
	for _, job := range stage.Jobs {
		if job.Status == "FAILED" || job.Status == "CANCELED" {
			return true
		}
	}
	return false
}

func hasFailedSteps(job api.WorkflowRunJob) bool {
	for _, step := range job.Steps {
		if step.Status == "FAILED" {
			return true
		}
	}
	return false
}

func statusLabel(cs *iostreams.ColorScheme, status string) string {
	if cs == nil || status == "" {
		return status
	}
	switch status {
	case "COMPLETED":
		return cs.Green(status)
	case "FAILED":
		return cs.Red(status)
	case "RUNNING":
		return cs.Yellow(status)
	case "CANCELED", "IGNORED", "PAUSED", "SUSPEND":
		return cs.Gray(status)
	default:
		return status
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

const msTimestampThreshold = 100_000_000_000

func formatTime(t int64) string {
	if t <= 0 {
		return "-"
	}
	secs := t
	if t >= msTimestampThreshold {
		secs = t / 1000
	}
	return time.Unix(secs, 0).UTC().Format(time.RFC3339)
}
