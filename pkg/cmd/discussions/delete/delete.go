// Package delete implements the discussions delete command.
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

// DeleteOptions configures the discussions delete command.
type DeleteOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)

	Org    string
	Number int

	Yes bool
}

// NewCmdDelete creates the discussions delete command.
func NewCmdDelete(f *cmdutil.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := &DeleteOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "delete <number>",
		Short: "Delete an organization discussion",
		Long: heredoc.Doc(`
			Delete a discussion in a GitCode organization via the v5 API
			(DELETE /api/v5/orgs/{org}/discuss/{number}).

			This is a destructive operation. Confirmation is required unless
			--yes is provided.
		`),
		Example: heredoc.Doc(`
			# Delete a discussion (prompts for confirmation)
			$ gc discussions delete 42 --org my-org

			# Skip confirmation
			$ gc discussions delete 42 --org my-org --yes
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

	cmd.Flags().StringVar(&opts.Org, "org", "", "Organization path (required)")
	cmd.MarkFlagRequired("org")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	cs := opts.IO.ColorScheme()

	if opts.Org == "" {
		return cmdutil.NewUsageError("--org is required")
	}
	if opts.Number < 1 {
		return cmdutil.NewUsageError("discussion number must be a positive integer")
	}

	if err := cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: fmt.Sprintf("%d", opts.Number),
		Prompt:   fmt.Sprintf("! This will delete discussion #%d in %s\nType the discussion number to confirm: ", opts.Number, cs.Bold(opts.Org)),
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

	if err := api.DeleteOrgDiscussion(client, opts.Org, opts.Number); err != nil {
		return fmt.Errorf("failed to delete discussion: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Deleted discussion #%d from %s\n", cs.Red("✗"), opts.Number, opts.Org)
	return nil
}
