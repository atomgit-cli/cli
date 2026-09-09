// Package retry implements the actions run retry command.
package retry

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// retryableJobStatuses are the job statuses eligible for `run retry --failed`.
var retryableJobStatuses = map[string]bool{
	"FAILED":   true,
	"CANCELED": true,
}

// RetryResult represents the result of a run retry operation.
type RetryResult struct {
	RunID     string   `json:"run_id"`
	Owner     string   `json:"owner"`
	Repo      string   `json:"repo"`
	Action    string   `json:"action"`
	JobRunIDs []string `json:"job_run_ids"`
}

// RetryOptions configures the actions run retry command.
type RetryOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	RunID      string
	JobIDs     []string
	Failed     bool

	JSON bool
}

// NewCmdRetry creates the actions run retry command.
func NewCmdRetry(f *cmdutil.Factory, runF func(*RetryOptions) error) *cobra.Command {
	opts := &RetryOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "retry <run-id>",
		Short: "Retry failed or canceled jobs of a pipeline run",
		Long: heredoc.Doc(`
			Retry selected jobs of a pipeline (workflow) run without rerunning the
			whole run. Only runs in a failed or canceled status can be retried.

			The run id is the workflow_run_id returned by ` + "`gc actions run list`" + `;
			job ids are the job_run_id values returned by ` + "`gc actions job list <run-id>`" + `.

			Exactly one of --job or --failed is required. Job ids are validated
			against the target run before the request is sent, because the server
			silently accepts job ids belonging to other runs.
		`),
		Example: heredoc.Doc(`
			# Retry specific jobs (repeatable)
			$ gc actions run retry <run-id> --job <job-id> --job <job-id-2> -R owner/repo

			# Retry all failed/canceled jobs
			$ gc actions run retry <run-id> --failed -R owner/repo

			# JSON output
			$ gc actions run retry <run-id> --failed --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RunID = strings.TrimSpace(args[0])
			if opts.RunID == "" {
				return cmdutil.NewUsageError("run id is required")
			}
			if opts.Failed == (len(opts.JobIDs) > 0) {
				return cmdutil.NewUsageError("exactly one of --job or --failed is required")
			}
			if runF != nil {
				return runF(opts)
			}
			return retryRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringArrayVar(&opts.JobIDs, "job", nil, "Job id to retry (repeatable)")
	cmd.Flags().BoolVar(&opts.Failed, "failed", false, "Retry all failed/canceled jobs of the run")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func retryRun(opts *RetryOptions) error {
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

	// Always list jobs first: --failed needs the statuses, --job needs the
	// ownership validation (the server silently accepts foreign job ids).
	jobs, err := api.ListActionsRunJobs(client, owner, repo, opts.RunID)
	if err != nil {
		return fmt.Errorf("failed to list jobs of pipeline run: %w", err)
	}

	jobIDs, err := resolveRetryJobIDs(opts, jobs.Jobs)
	if err != nil {
		return err
	}

	if err := api.RetryActionsRun(client, owner, repo, opts.RunID, jobIDs); err != nil {
		return fmt.Errorf("failed to retry pipeline run jobs: %w", err)
	}

	result := RetryResult{
		RunID:     opts.RunID,
		Owner:     owner,
		Repo:      repo,
		Action:    "retried",
		JobRunIDs: jobIDs,
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, result)
	}

	cs := opts.IO.ColorScheme()
	if _, err := fmt.Fprintf(opts.IO.Out, "%s Retried %d job(s) of pipeline run %s in %s/%s (track with: gc actions run watch %s)\n",
		cs.Green("✓"), len(jobIDs), opts.RunID, owner, repo, opts.RunID); err != nil {
		return err
	}
	return nil
}

// resolveRetryJobIDs turns --job/--failed into a validated job id list.
func resolveRetryJobIDs(opts *RetryOptions, jobs []api.WorkflowRunJob) ([]string, error) {
	if opts.Failed {
		var jobIDs []string
		for _, job := range jobs {
			if retryableJobStatuses[job.Status] {
				jobIDs = append(jobIDs, job.ID)
			}
		}
		if len(jobIDs) == 0 {
			return nil, fmt.Errorf("no failed or canceled jobs to retry in run %s", opts.RunID)
		}
		return jobIDs, nil
	}

	known := make(map[string]bool, len(jobs))
	for _, job := range jobs {
		known[job.ID] = true
	}
	jobIDs := make([]string, 0, len(opts.JobIDs))
	for _, id := range opts.JobIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, cmdutil.NewUsageError("job id must not be empty")
		}
		if !known[id] {
			return nil, fmt.Errorf("job %s does not belong to run %s", id, opts.RunID)
		}
		jobIDs = append(jobIDs, id)
	}
	return jobIDs, nil
}
