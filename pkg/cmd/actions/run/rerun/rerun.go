// Package rerun implements the actions run rerun command.
package rerun

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

// RerunResult represents the result of a run rerun operation.
type RerunResult struct {
	RunID  string `json:"run_id"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Action string `json:"action"`
}

// RerunOptions configures the actions run rerun command.
type RerunOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	RunID      string

	Yes  bool
	JSON bool
}

// NewCmdRerun creates the actions run rerun command.
func NewCmdRerun(f *cmdutil.Factory, runF func(*RerunOptions) error) *cobra.Command {
	opts := &RerunOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "rerun <run-id>",
		Short: "Rerun all jobs of a pipeline run",
		Long: heredoc.Doc(`
			Rerun all jobs of a pipeline (workflow) run.

			The run id is the workflow_run_id returned by ` + "`gc actions run list`" + `.
			Only runs in a complete status can be rerun; rerunning a run that is
			still RUNNING fails with a conflict error. Use --json for a
			machine-readable result.

			Non-interactive mode: Requires --yes to skip confirmation.
		`),
		Example: heredoc.Doc(`
			# Rerun a pipeline run
			$ gc actions run rerun <run-id> -R owner/repo

			# Skip confirmation (for scripts)
			$ gc actions run rerun <run-id> -R owner/repo --yes

			# JSON output
			$ gc actions run rerun <run-id> -R owner/repo --yes --json
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
			return rerunRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "Skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func rerunRun(opts *RerunOptions) error {
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

	expected := fmt.Sprintf("rerun pipeline run %s", opts.RunID)
	if err := cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: expected,
		Prompt: fmt.Sprintf(
			"! This will rerun pipeline run %s in %s/%s\nType %q to confirm: ",
			opts.RunID, owner, repo, expected,
		),
	}); err != nil {
		return err
	}

	if err := api.RerunActionsRun(client, owner, repo, opts.RunID); err != nil {
		return fmt.Errorf("failed to rerun pipeline run: %w", err)
	}

	result := RerunResult{
		RunID:  opts.RunID,
		Owner:  owner,
		Repo:   repo,
		Action: "rerun",
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, result)
	}

	cs := opts.IO.ColorScheme()
	if _, err := fmt.Fprintf(opts.IO.Out, "%s Rerunning pipeline run %s in %s/%s (track with: gc actions run watch %s)\n",
		cs.Red("✗"), opts.RunID, owner, repo, opts.RunID); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
