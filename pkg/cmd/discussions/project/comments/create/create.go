// Package create implements the discussions project comments create command.
package create

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

// CreateOptions configures the discussions project comments create command.
type CreateOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	Number     int

	Body     string
	BodyFile string

	JSON bool
}

// NewCmdCreate creates the discussions project comments create command.
func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "create <number>",
		Short: "Create a comment on a repository discussion",
		Long: heredoc.Doc(`
			Create a comment on a GitCode repository discussion via the v5 API
			(POST /api/v5/repos/{owner}/{repo}/discuss/{number}/comment).

			The comment body is required and can be provided with --body or
			--body-file (use - for stdin). The content is scanned for secrets
			before submission.
		`),
		Example: heredoc.Doc(`
			# Create a comment on discussion #42
			$ gc discussions project comments create 42 -R owner/repo --body "Great idea!"

			# Read body from a file
			$ gc discussions project comments create 42 -R owner/repo --body-file comment.md

			# Read body from stdin
			$ echo "Comment text" | gc discussions project comments create 42 -R owner/repo --body-file -

			# Output as JSON
			$ gc discussions project comments create 42 -R owner/repo --body "Comment" --json
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
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Comment body (required)")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body from file (use - for stdin)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func createRun(opts *CreateOptions) error {
	cs := opts.IO.ColorScheme()

	if opts.Number < 1 {
		return cmdutil.NewUsageError("discussion number must be a positive integer")
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

	c, err := api.CreateRepoDiscussionComment(client, owner, repo, opts.Number, &api.CreateRepoDiscussionCommentOptions{
		MdContent: body,
	})
	if err != nil {
		return fmt.Errorf("failed to create discussion comment: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, c)
	}

	fmt.Fprintf(opts.IO.Out, "%s Created comment on discussion #%d in %s\n", cs.Green("✓"), opts.Number, repository)
	return nil
}
