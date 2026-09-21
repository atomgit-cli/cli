// Package edit implements the discussions edit command (org scope).
package edit

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/render"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// EditOptions configures the discussions edit command (org scope).
type EditOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)

	Org    string
	Number int

	Title        string
	Body         string
	BodyFile     string
	CategoryName string

	JSON bool
}

// NewCmdEdit creates the discussions edit command (org scope).
func NewCmdEdit(f *cmdutil.Factory, runF func(*EditOptions) error) *cobra.Command {
	opts := &EditOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "edit <number>",
		Short: "Edit an organization discussion",
		Long: heredoc.Doc(`
			Edit a discussion in a GitCode organization via the v5 API
			(PUT /api/v5/orgs/{org}/discuss/{number}).

			Only the fields provided via flags are updated. The body can be
			provided with --body or --body-file (use - for stdin) and is
			scanned for secrets before submission.
		`),
		Example: heredoc.Doc(`
			# Edit a discussion title
			$ gc discussions edit 42 --org my-org --title "Updated title"

			# Edit a discussion body from a file
			$ gc discussions edit 42 --org my-org --body-file idea.md

			# Move a discussion to another category
			$ gc discussions edit 42 --org my-org --category "Q&A"

			# Output as JSON
			$ gc discussions edit 42 --org my-org --title "Updated title" --json
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
			return editRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Org, "org", "", "Organization path (required)")
	cmd.MarkFlagRequired("org")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "New discussion title")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "New discussion body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use - for stdin)")
	cmd.Flags().StringVar(&opts.CategoryName, "category", "", "New discussion category name")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func editRun(opts *EditOptions) error {
	if opts.Org == "" {
		return cmdutil.NewUsageError("--org is required")
	}
	if opts.Number < 1 {
		return cmdutil.NewUsageError("discussion number must be a positive integer")
	}
	if opts.Title == "" && opts.Body == "" && opts.BodyFile == "" && opts.CategoryName == "" {
		return cmdutil.NewUsageError("no changes specified. Use flags to specify what to edit")
	}

	updateOpts := &api.UpdateOrgDiscussionOptions{}
	if opts.Title != "" {
		updateOpts.Title = opts.Title
	}
	if opts.Body != "" || opts.BodyFile != "" {
		body, err := cmdutil.ReadBody(opts.Body, opts.BodyFile, opts.IO.In)
		if err != nil {
			return err
		}
		updateOpts.MdContent = body
	}
	if opts.CategoryName != "" {
		updateOpts.CategoryName = opts.CategoryName
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	d, err := api.UpdateOrgDiscussion(client, opts.Org, opts.Number, updateOpts)
	if err != nil {
		return fmt.Errorf("failed to edit discussion: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, d)
	}

	render.PrintDiscussion(opts.IO, d)
	return nil
}
