// Package create implements the discussions create command (org scope).
package create

import (
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions/render"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// CreateOptions configures the discussions create command (org scope).
type CreateOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)

	Org string

	Title        string
	Body         string
	BodyFile     string
	CategoryName string

	JSON bool
}

// NewCmdCreate creates the discussions create command (org scope).
func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an organization discussion",
		Long: heredoc.Doc(`
			Create a discussion in a GitCode organization via the v5 API
			(POST /api/v5/orgs/{org}/discuss).

			The title and body are required; the body can be provided with
			--body or --body-file (use - for stdin). The content is scanned
			for secrets before submission.
		`),
		Example: heredoc.Doc(`
			# Create a discussion in an organization
			$ gc discussions create --org my-org --title "New idea" --category "Ideas" --body "Description"

			# Read body from a file
			$ gc discussions create --org my-org --title "New idea" --category "Ideas" --body-file idea.md

			# Read body from stdin
			$ cat idea.md | gc discussions create --org my-org --title "New idea" --category "Ideas" --body-file -

			# Output as JSON
			$ gc discussions create --org my-org --title "New idea" --category "Ideas" --body "Description" --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Org, "org", "", "Organization path (required)")
	cmd.MarkFlagRequired("org")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Discussion title (required)")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Discussion body (required)")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use - for stdin)")
	cmd.Flags().StringVar(&opts.CategoryName, "category", "", "Discussion category name (required)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if opts.Org == "" {
		return cmdutil.NewUsageError("--org is required")
	}
	if opts.Title == "" {
		return cmdutil.NewUsageError("--title is required")
	}
	if opts.CategoryName == "" {
		return cmdutil.NewUsageError("--category is required")
	}

	body, err := cmdutil.ReadBody(opts.Body, opts.BodyFile, opts.IO.In)
	if err != nil {
		return err
	}
	if body == "" {
		return cmdutil.NewUsageError("--body or --body-file is required")
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	d, err := api.CreateOrgDiscussion(client, opts.Org, &api.CreateOrgDiscussionOptions{
		Title:        opts.Title,
		MdContent:    body,
		CategoryName: opts.CategoryName,
	})
	if err != nil {
		return fmt.Errorf("failed to create discussion: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, d)
	}

	render.PrintDiscussion(opts.IO, d)
	return nil
}
