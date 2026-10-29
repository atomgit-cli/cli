// Package create implements the discussions project create command.
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

// CreateOptions configures the discussions project create command.
type CreateOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string

	Title        string
	Body         string
	BodyFile     string
	CategoryName string

	JSON bool
}

// NewCmdCreate creates the discussions project create command.
func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a repository discussion",
		Long: heredoc.Doc(`
			Create a discussion in a GitCode repository via the v5 API
			(POST /api/v5/repos/{owner}/{repo}/discuss).

			The title and body are required; the body can be provided with
			--body or --body-file (use - for stdin). The content is scanned
			for secrets before submission.
		`),
		Example: heredoc.Doc(`
			# Create a discussion in a repository
			$ gc discussions project create -R owner/repo --title "New idea" --category "Ideas" --body "Description"

			# Read body from a file
			$ gc discussions project create -R owner/repo --title "New idea" --category "Ideas" --body-file idea.md

			# Read body from stdin
			$ cat idea.md | gc discussions project create -R owner/repo --title "New idea" --category "Ideas" --body-file -

			# Output as JSON
			$ gc discussions project create -R owner/repo --title "New idea" --category "Ideas" --body "Description" --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Discussion title (required)")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Discussion body (required)")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use - for stdin)")
	cmd.Flags().StringVar(&opts.CategoryName, "category", "", "Discussion category name (required)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func createRun(opts *CreateOptions) error {
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

	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := cmdutil.ParseRepo(repository)
	if err != nil {
		return err
	}

	d, err := api.CreateRepoDiscussion(client, owner, repo, &api.CreateRepoDiscussionOptions{
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
