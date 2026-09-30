// Package delete implements the release delete command
package delete

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type DeleteOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Arguments
	TagName string

	// Flags
	Repository string
	Yes        bool
	DryRun     bool
}

// NewCmdDelete creates the delete command
func NewCmdDelete(f *cmdutil.Factory, runF func(*DeleteOptions) error) *cobra.Command {
	opts := &DeleteOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "delete <tag>",
		Short: "Delete a release",
		Long: heredoc.Doc(`
			Delete a release from a repository.

			GitCode does not currently provide a release deletion API: the
			deletion request fails with HTTP 405 Method Not Allowed. Use
			--dry-run to preview the target, and delete the release from
			the repository's Releases page in the web UI.

			This would delete the release but not the associated git tag.

				Non-interactive mode: Requires --yes to skip confirmation.
		`),
		Example: heredoc.Doc(`
			# Preview a deletion (the platform has no delete API)
			$ gc release delete v1.0.0 -R owner/repo --dry-run

			# Attempt a deletion without confirmation
			$ gc release delete v1.0.0 -R owner/repo --yes
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TagName = args[0]

			if runF != nil {
				return runF(opts)
			}
			return deleteRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview the deletion without deleting the release")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	cs := opts.IO.ColorScheme()

	// Get repository
	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := parseRepo(repository)
	if err != nil {
		return err
	}

	if opts.DryRun {
		fmt.Fprintf(opts.IO.Out, "Dry run: would delete release %s from %s/%s\n", opts.TagName, owner, repo)
		return nil
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	// Get release for confirmation
	release, err := api.GetRelease(client, owner, repo, opts.TagName)
	if err != nil {
		return cmdutil.WrapNotFound(err, "release %s not found in %s/%s", opts.TagName, owner, repo)
	}

	if err := cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: opts.TagName,
		Prompt:   fmt.Sprintf("! This will delete release %s\nType the tag name to confirm: ", cs.Bold(confirmTitle(release))),
	}); err != nil {
		return err
	}

	// Delete release, reusing the pre-fetched release to avoid an extra
	// GetRelease call if the tag-based endpoint falls back to ID deletion.
	err = api.DeleteReleaseByTagKnown(client, owner, repo, opts.TagName, release)
	if err != nil {
		if errors.Is(err, api.ErrNoReleaseID) {
			return fmt.Errorf("failed to delete release: %w; tag-based endpoint unavailable and GitCode omits release IDs in lookup responses", err)
		}
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusMethodNotAllowed {
			// GitCode has no release-deletion API (docs/COMMANDS.md,
			// "release delete"); point the user at the web UI instead.
			guide := "Delete the release from the web UI instead."
			if release.HTMLURL != "" {
				guide = fmt.Sprintf("Delete the release from the web UI instead:\n  %s", release.HTMLURL)
			}
			return fmt.Errorf("failed to delete release: %w\n\nGitCode does not support deleting releases through the API (HTTP 405). %s", err, guide)
		}
		return fmt.Errorf("failed to delete release: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Deleted release %s\n", cs.Red("✓"), opts.TagName)
	return nil
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}

// confirmTitle shows the release identity in the confirmation prompt. The
// expected input is the tag, so when Name and TagName differ both are shown
// — displaying only the name invited typing the name and being rejected
// (mirrors delete-asset, which shows exactly what it expects).
func confirmTitle(release *api.Release) string {
	if release.Name != "" && release.Name != release.TagName {
		return fmt.Sprintf("%s (tag: %s)", release.Name, release.TagName)
	}
	return release.TagName
}
