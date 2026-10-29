// Package delete implements the discussions project delete command.
package delete

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

// DeleteOptions configures the discussions project delete command.
type DeleteOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	Number     int

	Yes bool
}

// NewCmdDelete creates the discussions project delete command.
func NewCmdDelete(f *cmdutil.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := &DeleteOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "delete <number>",
		Short: "Delete a repository discussion",
		Long: heredoc.Doc(`
			Delete a discussion in a GitCode repository via the v5 API
			(DELETE /api/v5/repos/{owner}/{repo}/discuss/{number}).

			This is a destructive operation. Confirmation is required unless
			--yes is provided.
		`),
		Example: heredoc.Doc(`
			# Delete a discussion (prompts for confirmation)
			$ gc discussions project delete 42 -R owner/repo

			# Skip confirmation
			$ gc discussions project delete 42 -R owner/repo --yes
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return cmdutil.NewUsageError(fmt.Sprintf("invalid discussion number: %s", args[0]))
			}
			opts.Number = number
			if runF != nil {
				return runF(opts)
			}
			return deleteRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	cs := opts.IO.ColorScheme()

	if opts.Number < 1 {
		return cmdutil.NewUsageError("discussion number must be a positive integer")
	}

	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}

	if err := cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: fmt.Sprintf("%d", opts.Number),
		Prompt:   fmt.Sprintf("! This will delete discussion #%d in %s\nType the discussion number to confirm: ", opts.Number, cs.Bold(repository)),
	}); err != nil {
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

	owner, repo, err := cmdutil.ParseRepo(repository)
	if err != nil {
		return err
	}

	if err := api.DeleteRepoDiscussion(client, owner, repo, opts.Number); err != nil {
		return fmt.Errorf("failed to delete discussion: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Deleted discussion #%d from %s\n", cs.Red("✗"), opts.Number, repository)
	return nil
}
