// Package stop implements the actions run stop command.
package stop

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

// StopResult represents the result of a run stop operation.
type StopResult struct {
	RunID  string `json:"run_id"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Action string `json:"action"`
}

// StopOptions configures the actions run stop command.
type StopOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	RunID      string

	JSON bool
}

// NewCmdStop creates the actions run stop command.
func NewCmdStop(f *cmdutil.Factory, runF func(*StopOptions) error) *cobra.Command {
	opts := &StopOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "stop <run-id>",
		Short: "Stop a running pipeline run",
		Long: heredoc.Doc(`
			Stop a running pipeline (workflow) run.

			The run id is the workflow_run_id returned by ` + "`gc actions run list`" + `.
			The API is idempotent: stopping a run that has already finished still
			returns success. Use --json for a machine-readable result.
		`),
		Example: heredoc.Doc(`
			# Stop a running pipeline run
			$ gc actions run stop <run-id> -R owner/repo

			# JSON output
			$ gc actions run stop <run-id> -R owner/repo --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RunID = strings.TrimSpace(args[0])
			if opts.RunID == "" {
				return cmdutil.NewUsageError("run id is required")
			}
			if runF != nil {
				return runF(opts)
			}
			return stopRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func stopRun(opts *StopOptions) error {
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

	if err := api.StopActionsRun(client, owner, repo, opts.RunID); err != nil {
		return fmt.Errorf("failed to stop pipeline run: %w", err)
	}

	result := StopResult{
		RunID:  opts.RunID,
		Owner:  owner,
		Repo:   repo,
		Action: "stopped",
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, result)
	}

	cs := opts.IO.ColorScheme()
	if _, err := fmt.Fprintf(opts.IO.Out, "%s Stopped pipeline run %s in %s/%s\n", cs.Green("✓"), opts.RunID, owner, repo); err != nil {
		return err
	}
	return nil
}
