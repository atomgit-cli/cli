// Package update implements the repo pr-settings update command.
package update

import (
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// UpdateOptions configures the pr-settings update command.
type UpdateOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository    string
	Reviewers     int
	ReviewersSet  bool
	PipelineReq   bool
	PipelineSet   bool
	NoSelfMerge   bool
	SelfMergeSet  bool
	ForceMerge    bool
	ForceMergeSet bool
	DeleteSrc     bool
	DeleteSrcSet  bool
	CloseIssue    bool
	CloseIssueSet bool
	MergeMethod   string
	JSON          bool
}

// NewCmdUpdate creates the pr-settings update command.
func NewCmdUpdate(f *cmdutil.Factory, runF func(*UpdateOptions) error) *cobra.Command {
	opts := &UpdateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update pull request settings",
		Long: heredoc.Doc(`
			Update the pull request merge gate settings for a repository.
			Only provided fields are updated.
		`),
		Example: heredoc.Doc(`
			# Require pipeline success before merge
			$ gc repo pr-settings update --pipeline-required -R owner/repo

			# Set minimum reviewers to 2
			$ gc repo pr-settings update --reviewers 2 -R owner/repo

			# Disable self-merge
			$ gc repo pr-settings update --no-self-merge -R owner/repo

			# Set merge method to fast-forward
			$ gc repo pr-settings update --merge-method ff -R owner/repo

			# Close the issues referenced by a merged pull request
			$ gc repo pr-settings update --close-issue-on-merge -R owner/repo
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ReviewersSet = cmd.Flags().Changed("reviewers")
			opts.PipelineSet = cmd.Flags().Changed("pipeline-required")
			opts.SelfMergeSet = cmd.Flags().Changed("no-self-merge")
			opts.ForceMergeSet = cmd.Flags().Changed("force-merge")
			opts.DeleteSrcSet = cmd.Flags().Changed("delete-source-branch")
			opts.CloseIssueSet = cmd.Flags().Changed("close-issue-on-merge")
			if runF != nil {
				return runF(opts)
			}
			return updateRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().IntVar(&opts.Reviewers, "reviewers", 0, "Minimum number of reviewers (0 to disable)")
	cmd.Flags().BoolVar(&opts.PipelineReq, "pipeline-required", false, "Require pipeline success before merge")
	cmd.Flags().BoolVar(&opts.NoSelfMerge, "no-self-merge", false, "Disable merging own pull requests")
	cmd.Flags().BoolVar(&opts.ForceMerge, "force-merge", false, "Allow admin force merge")
	cmd.Flags().BoolVar(&opts.DeleteSrc, "delete-source-branch", false, "Delete source branch on merge")
	cmd.Flags().BoolVar(&opts.CloseIssue, "close-issue-on-merge", false, "Close the issues referenced by a merged pull request")
	cmd.Flags().StringVar(&opts.MergeMethod, "merge-method", "", "Merge method (merge/rebase_merge/ff)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func updateRun(opts *UpdateOptions) error {
	req := &api.UpdatePRSettingsRequest{}

	if opts.ReviewersSet {
		req.ApprovalRequiredReviewers = &opts.Reviewers
	}
	if opts.PipelineSet {
		req.OnlyAllowMergeIfPipelineSucceeds = &opts.PipelineReq
	}
	if opts.SelfMergeSet {
		req.DisableMergeBySelf = &opts.NoSelfMerge
	}
	if opts.ForceMergeSet {
		req.CanForceMerge = &opts.ForceMerge
	}
	if opts.DeleteSrcSet {
		req.DeleteSourceBranchWhenMerged = &opts.DeleteSrc
	}
	if opts.CloseIssueSet {
		req.CloseIssueWhenMrMerged = &opts.CloseIssue
	}
	if opts.MergeMethod != "" {
		req.MergeMethod = opts.MergeMethod
	}

	if req.ApprovalRequiredReviewers == nil &&
		req.OnlyAllowMergeIfPipelineSucceeds == nil &&
		req.DisableMergeBySelf == nil &&
		req.CanForceMerge == nil &&
		req.DeleteSourceBranchWhenMerged == nil &&
		req.CloseIssueWhenMrMerged == nil &&
		req.MergeMethod == "" {
		return cmdutil.NewUsageError("at least one setting must be provided to update")
	}

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

	settings, err := api.UpdatePRSettings(client, owner, repo, req)
	if err != nil {
		return fmt.Errorf("failed to update PR settings: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, settings)
	}

	fmt.Fprintf(opts.IO.ErrOut, "Updated PR settings for %s/%s.\n", owner, repo)
	return nil
}
