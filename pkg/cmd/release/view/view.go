// Package view implements the release view command
package view

import (
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/browser"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type ViewOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// OpenBrowser opens the given URL; injected for tests (default
	// browser.Open). Mirrors the pr create / auth login seams.
	OpenBrowser func(string) error

	// Arguments
	TagName string

	// Flags
	Repository string
	Web        bool
	JSON       bool
}

// NewCmdView creates the view command
func NewCmdView(f *cmdutil.Factory, runF func(*ViewOptions) error) *cobra.Command {
	opts := &ViewOptions{
		IO:          f.IOStreams,
		HttpClient:  f.HttpClient,
		BaseRepo:    f.BaseRepo,
		OpenBrowser: browser.Open,
	}

	cmd := &cobra.Command{
		Use:   "view <tag>",
		Short: "View release details",
		Long: heredoc.Doc(`
			View details of a release.
		`),
		Example: heredoc.Doc(`
			# View a release
			$ gc release view v1.0.0 -R owner/repo

			# View in browser
			$ gc release view v1.0.0 -R owner/repo --web

			# Output as JSON
			$ gc release view v1.0.0 -R owner/repo --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TagName = args[0]

			if runF != nil {
				return runF(opts)
			}
			return viewRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open in browser")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func viewRun(opts *ViewOptions) error {
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

	// Get release
	release, err := api.GetRelease(client, owner, repo, opts.TagName)
	if err != nil {
		return cmdutil.WrapNotFound(err, "release %s not found in %s/%s", opts.TagName, owner, repo)
	}

	// Open in browser if requested
	if opts.Web {
		fmt.Fprintf(opts.IO.Out, "Opening %s in your browser.\n", release.HTMLURL)
		openBrowser := opts.OpenBrowser
		if openBrowser == nil {
			openBrowser = browser.Open
		}
		if err := openBrowser(release.HTMLURL); err != nil {
			if opts.IO.IsStdoutTTY() {
				return err
			}
			fmt.Fprintf(opts.IO.ErrOut, "Failed to open browser: %v\n", err)
		}
		return nil
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, release)
	}

	// Output
	title := release.TagName
	if release.Name != "" {
		title = release.Name
	}

	fmt.Fprintf(opts.IO.Out, "\n")
	fmt.Fprintf(opts.IO.Out, "%s\n", cs.Bold(title))
	fmt.Fprintf(opts.IO.Out, "  Tag: %s\n", release.TagName)

	// Status
	if release.Draft {
		fmt.Fprintf(opts.IO.Out, "  Status: %s\n", cs.Gray("draft"))
	} else if release.Prerelease {
		fmt.Fprintf(opts.IO.Out, "  Status: %s\n", cs.Yellow("pre-release"))
	} else {
		fmt.Fprintf(opts.IO.Out, "  Status: %s\n", cs.Green("published"))
	}

	// Dates
	if release.PublishedAt != nil {
		fmt.Fprintf(opts.IO.Out, "  Published: %s\n", release.PublishedAt.Format("2006-01-02 15:04:05"))
	}
	if !release.CreatedAt.IsZero() {
		fmt.Fprintf(opts.IO.Out, "  Created: %s\n", release.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	// URL
	fmt.Fprintf(opts.IO.Out, "  URL: %s\n", release.HTMLURL)

	// Body
	if release.Body != "" {
		fmt.Fprintf(opts.IO.Out, "\n")
		fmt.Fprintf(opts.IO.Out, "%s\n", release.Body)
	}

	// Assets
	if len(release.Assets) > 0 {
		fmt.Fprintf(opts.IO.Out, "\n%s\n", cs.Bold("Assets:"))
		for _, asset := range release.Assets {
			fmt.Fprintf(opts.IO.Out, "  %s (%s, %d downloads)\n", asset.Name, assetSizeLabel(asset), asset.Downloads)
		}
	}

	fmt.Fprintf(opts.IO.Out, "\n")
	return nil
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}

func assetSizeLabel(asset api.ReleaseAsset) string {
	if asset.Size <= 0 {
		return "unknown size"
	}
	return fmt.Sprintf("%d bytes", asset.Size)
}
