// Package list implements the release list command
package list

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
)

type ListOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Flags
	Repository string
	Limit      int
	Page       int
	JSON       bool
}

// NewCmdList creates the list command
func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases",
		Long: heredoc.Doc(`
			List releases in a GitCode repository.
		`),
		Example: heredoc.Doc(`
			# List releases
			$ gc release list -R owner/repo

			# Limit the number of results
			$ gc release list -R owner/repo --limit 10

			# List 150 releases (fetched across API pages)
			$ gc release list -R owner/repo --limit 150

			# Fetch a single specific page
			$ gc release list -R owner/repo --page 2

			# Output as JSON
			$ gc release list -R owner/repo --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of releases to list (fetched across API pages when greater than 100)")
	cmd.Flags().IntVar(&opts.Page, "page", 0, "Page number to fetch (single request; the API returns at most 100 per page)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func listRun(opts *ListOptions) error {
	cs := opts.IO.ColorScheme()

	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
	if err != nil {
		return err
	}

	// Get repository
	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := parseRepo(repository)
	if err != nil {
		return err
	}

	// Validate pagination parameters
	if opts.Limit <= 0 {
		return cmdutil.NewUsageError("--limit must be greater than 0")
	}
	if opts.Page < 0 {
		return cmdutil.NewUsageError("--page must be greater than or equal to 0")
	}
	if opts.Page > 0 && opts.Limit > 100 {
		return cmdutil.NewUsageError("--page cannot be combined with --limit greater than 100 (the API returns at most 100 releases per page)")
	}

	// List releases
	releases, err := fetchReleases(client, owner, repo, opts)
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	// Sort releases by published date descending (newest first).
	// Falls back to created_at when published_at is nil. Sort before the
	// limit trim: pages accumulate in created_at order, but the display
	// order (and the (latest) marker) is published_at order, so trimming
	// first could drop newer-published releases at a page boundary.
	sortReleasesByDate(releases)
	if len(releases) > opts.Limit {
		releases = releases[:opts.Limit]
	}

	if len(releases) == 0 {
		if opts.JSON {
			return cmdutil.WriteJSON(opts.IO.Out, releases)
		}
		fmt.Fprintf(opts.IO.Out, "No releases found in %s/%s\n", owner, repo)
		return nil
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, releases)
	}

	// Output
	fmt.Fprintf(opts.IO.Out, "\n")
	latestPublishedMarked := false
	for _, r := range releases {
		tag := r.TagName
		if r.Name != "" {
			tag = r.Name
		}

		// Status indicators
		var status string
		if r.Draft {
			status = cs.Gray("(draft)")
		} else if r.Prerelease {
			status = cs.Yellow("(pre-release)")
		} else if !latestPublishedMarked {
			status = cs.Green("(latest)")
			latestPublishedMarked = true
		} else {
			status = cs.Green("(published)")
		}

		fmt.Fprintf(opts.IO.Out, "%s %s\n", cs.Bold(tag), status)
		if r.Body != "" {
			// Show first line of body
			lines := strings.Split(r.Body, "\n")
			if len(lines) > 0 && lines[0] != "" {
				fmt.Fprintf(opts.IO.Out, "  %s\n", output.Truncate(lines[0], 60))
			}
		}
		fmt.Fprintf(opts.IO.Out, "  %s\n", r.HTMLURL)
		fmt.Fprintf(opts.IO.Out, "\n")
	}

	return nil
}

// fetchReleases returns the releases for the requested window. The API
// caps a single page at 100 releases, so without an explicit --page the
// list accumulates consecutive pages until the limit is satisfied or a
// short page marks the end — a --limit above 100 must not silently
// truncate. An explicit --page fetches exactly that page instead.
func fetchReleases(client *api.Client, owner, repo string, opts *ListOptions) ([]api.Release, error) {
	perPage := opts.Limit
	if perPage > 100 {
		perPage = 100
	}
	if opts.Page > 0 {
		return api.ListReleases(client, owner, repo, &api.ReleaseListOptions{
			PerPage:   perPage,
			Page:      opts.Page,
			Direction: "desc",
		})
	}
	// Non-nil empty slice: an empty result must stay "[]" in JSON output.
	all := []api.Release{}
	for page := 1; ; page++ {
		releases, err := api.ListReleases(client, owner, repo, &api.ReleaseListOptions{
			PerPage:   perPage,
			Page:      page,
			Direction: "desc",
		})
		if err != nil {
			return nil, err
		}
		all = append(all, releases...)
		if len(all) >= opts.Limit || len(releases) < perPage {
			return all, nil
		}
	}
}

// sortReleasesByDate sorts releases by published date in descending order (newest first).
//
// When PublishedAt is nil (e.g., draft releases or API omissions), the function
// falls back to CreatedAt. If CreatedAt is the zero value (time.Time{}), the
// comparison via time.Time.After places that release at the oldest position —
// time.Time{} corresponds to year 1, which is earlier than any real timestamp.
// In practice this is acceptable because releases always have a valid CreatedAt
// populated by GitCode at creation time.
func sortReleasesByDate(releases []api.Release) {
	sort.Slice(releases, func(i, j int) bool {
		a, b := releases[i], releases[j]

		// Prefer PublishedAt; fall back to CreatedAt when PublishedAt is nil.
		var aTime, bTime *api.FlexibleTime
		if a.PublishedAt != nil {
			aTime = a.PublishedAt
		} else {
			aTime = &a.CreatedAt
		}
		if b.PublishedAt != nil {
			bTime = b.PublishedAt
		} else {
			bTime = &b.CreatedAt
		}

		// Newest first (descending order)
		return aTime.Time.After(bTime.Time)
	})
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}
