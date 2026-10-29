// Package edit implements the discussions project edit command.
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

// EditOptions configures the discussions project edit command.
type EditOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	Number     int

	Title        string
	Body         string
	BodyFile     string
	CategoryName string

	JSON bool
}

// NewCmdEdit creates the discussions project edit command.
func NewCmdEdit(f *cmdutil.Factory, runF func(*EditOptions) error) *cobra.Command {
	opts := &EditOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "edit <number>",
		Short: "Edit a repository discussion",
		Long: heredoc.Doc(`
			Edit a discussion in a GitCode repository via the v5 API
			(PUT /api/v5/repos/{owner}/{repo}/discuss/{number}).

			Only the fields provided via flags are updated. The body can be
			provided with --body or --body-file (use - for stdin) and is
			scanned for secrets before submission.
		`),
		Example: heredoc.Doc(`
			# Edit a discussion title
			$ gc discussions project edit 42 -R owner/repo --title "Updated title"

			# Edit a discussion body from a file
			$ gc discussions project edit 42 -R owner/repo --body-file idea.md

			# Move a discussion to another category
			$ gc discussions project edit 42 -R owner/repo --category "Q&A"

			# Output as JSON
			$ gc discussions project edit 42 -R owner/repo --title "Updated title" --json
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

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "New discussion title")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "New discussion body")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use - for stdin)")
	cmd.Flags().StringVar(&opts.CategoryName, "category", "", "New discussion category name")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func editRun(opts *EditOptions) error {
	if opts.Number < 1 {
		return cmdutil.NewUsageError("discussion number must be a positive integer")
	}
	if opts.Title == "" && opts.Body == "" && opts.BodyFile == "" && opts.CategoryName == "" {
		return cmdutil.NewUsageError("no changes specified. Use flags to specify what to edit")
	}

	updateOpts := &api.UpdateRepoDiscussionOptions{}
	if opts.Title != "" {
		updateOpts.Title = opts.Title
	}
	if opts.Body != "" || opts.BodyFile != "" {
		body, err := cmdutil.ReadBody(opts.Body, opts.BodyFile, opts.IO.In)
		if err != nil {
			return err
		}
		if body == "" {
			return cmdutil.NewUsageError("--body/--body-file must not be empty")
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

	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := cmdutil.ParseRepo(repository)
	if err != nil {
		return err
	}

	d, err := api.UpdateRepoDiscussion(client, owner, repo, opts.Number, updateOpts)
	if err != nil {
		return fmt.Errorf("failed to edit discussion: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, d)
	}

	render.PrintDiscussion(opts.IO, d)
	return nil
}
