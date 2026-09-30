// Package download implements the release download command
package download

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type DownloadOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// DownloadHttpClient uses longer timeout for file downloads
	// If nil, defaults to HttpClient
	DownloadHttpClient func() (*http.Client, error)

	// Arguments
	TagName string
	Assets  []string

	// Flags
	Repository   string
	Output       string
	All          bool
	Clobber      bool
	SkipExisting bool
}

// NewCmdDownload creates the download command
func NewCmdDownload(f *cmdutil.Factory, runF func(*DownloadOptions) error) *cobra.Command {
	opts := &DownloadOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
		DownloadHttpClient: func() (*http.Client, error) {
			return api.NewDownloadHTTPClientWithEnvTimeout(), nil
		},
	}

	cmd := &cobra.Command{
		Use:   "download [<tag>] [asset]",
		Short: "Download release assets",
		Long: heredoc.Doc(`
			Download assets from a release.

			Without an asset name, downloads all assets.
			With --all, downloads source archives as well.
		`),
		Example: heredoc.Doc(`
			# Download all assets from latest release
			$ gc release download -R owner/repo

			# Download all assets from a specific release
			$ gc release download v1.0.0 -R owner/repo

			# Download a specific asset
			$ gc release download v1.0.0 app.zip -R owner/repo

			# Download to a specific directory
			$ gc release download v1.0.0 -R owner/repo --output ./downloads/
		`),
		Args: cobra.MaximumNArgs(64),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.TagName = args[0]
			}
			if len(args) > 1 {
				opts.Assets = args[1:]
			}

			if runF != nil {
				return runF(opts)
			}
			return downloadRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", ".", "Output directory")
	cmd.Flags().BoolVarP(&opts.All, "all", "A", false, "Download source archives as well")
	cmd.Flags().BoolVar(&opts.Clobber, "clobber", false, "Overwrite existing files")
	cmd.Flags().BoolVar(&opts.SkipExisting, "skip-existing", false, "Skip downloading when a file of the same name exists")
	cmd.MarkFlagsMutuallyExclusive("clobber", "skip-existing")

	return cmd
}

func downloadRun(opts *DownloadOptions) error {
	cs := opts.IO.ColorScheme()

	// API client for release metadata (uses default timeout)
	apiClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(apiClient)
	if err != nil {
		return err
	}

	// Download client for asset downloads (uses 10m timeout or GC_TIMEOUT)
	var downloadClient *http.Client
	if opts.DownloadHttpClient != nil {
		downloadClient, err = opts.DownloadHttpClient()
		if err != nil {
			return fmt.Errorf("failed to create download HTTP client: %w", err)
		}
	} else {
		// Fallback to regular HttpClient if DownloadHttpClient not set
		downloadClient = apiClient
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
	var release *api.Release
	if opts.TagName == "" {
		release, err = api.GetLatestRelease(client, owner, repo)
		if err != nil {
			return cmdutil.WrapNotFound(err, "no releases found in %s/%s", owner, repo)
		}
	} else {
		release, err = api.GetRelease(client, owner, repo, opts.TagName)
		if err != nil {
			return cmdutil.WrapNotFound(err, "release %s not found in %s/%s", opts.TagName, owner, repo)
		}
	}

	// Create output directory
	if err := os.MkdirAll(opts.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Filter assets
	assets := release.Assets
	if len(opts.Assets) > 0 {
		// Explicit names are exact intent: any miss is an error (the message
		// lists the available names so recovery needs no extra round trip),
		// and the named set bypasses the source-archive filter — naming an
		// archive explicitly must download it even without --all.
		matched := filterAssets(assets, opts.Assets)
		requested := map[string]bool{}
		for _, name := range opts.Assets {
			requested[name] = true
		}
		matchedNames := map[string]bool{}
		for _, asset := range matched {
			matchedNames[asset.Name] = true
		}
		var missing []string
		for name := range requested {
			if !matchedNames[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			available := make([]string, 0, len(release.Assets))
			for _, asset := range release.Assets {
				available = append(available, asset.Name)
			}
			sort.Strings(available)
			return fmt.Errorf("asset(s) not found in release %s: %s\navailable assets: %s",
				release.TagName, strings.Join(missing, ", "), strings.Join(available, ", "))
		}
		assets = matched
	} else if !opts.All {
		// Filter out source archives unless --all is set (unnamed path only).
		assets = filterSourceArchives(assets)
	}

	if len(assets) == 0 {
		fmt.Fprintf(opts.IO.Out, "No assets to download\n")
		return nil
	}

	// Download each asset
	for _, asset := range assets {
		err := downloadAsset(asset, opts.Output, downloadClient, cs, opts.IO.Out, client, owner, repo, release.TagName, opts.Clobber, opts.SkipExisting)
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadAsset(asset api.ReleaseAsset, outputDir string, httpClient *http.Client, cs *iostreams.ColorScheme, out io.Writer, client *api.Client, owner, repo, tag string, clobber, skipExisting bool) error {
	// Validate and construct safe output path
	outputPath, err := safeOutputPath(outputDir, asset.Name)
	if err != nil {
		return fmt.Errorf("invalid asset name '%s': %w", asset.Name, err)
	}
	// An existing file is the user's data: refuse by default (gh release
	// download semantics) instead of silently clobbering it.
	if _, err := os.Stat(outputPath); err == nil {
		if skipExisting {
			fmt.Fprintf(out, "%s Skipped %s (exists)\n", cs.Yellow("!"), asset.Name)
			return nil
		}
		if !clobber {
			return fmt.Errorf("file already exists: %s (use --clobber to overwrite or --skip-existing to skip)", outputPath)
		}
	}
	fmt.Fprintf(out, "%s Downloading %s...\n", cs.Blue("⬇"), asset.Name)

	downloadURL := assetDownloadURL(asset, client.Host(), owner, repo, tag)

	// Create request
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if token := client.Token(); token != "" && req.URL.Host == client.Host() {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Write to a temp file first, then atomically rename to target.
	// This avoids the TOCTOU race between Lstat and Create on the output path.
	tmpFile, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy content
	written, err := io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Atomic rename to final destination
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("failed to finalize file: %w", err)
	}

	fmt.Fprintf(out, "%s Downloaded %s (%s)\n", cs.Green("✓"), asset.Name, formatSize(int(written)))
	return nil
}

func assetDownloadURL(asset api.ReleaseAsset, apiHost, owner, repo, tag string) string {
	if isSourceArchiveAsset(asset) {
		return asset.BrowserDownloadURL
	}

	return fmt.Sprintf("https://%s/api/v5/repos/%s/%s/releases/%s/attach_files/%s/download",
		apiHost, owner, repo, url.PathEscape(tag), url.PathEscape(asset.Name))
}

// isSourceArchiveAsset reports whether an asset is an auto-generated source
// archive. Detection is based solely on the browser download URL containing
// the /archive/refs/heads/ path that GitCode uses for generated archives; an
// empty URL is treated as "not a source archive" (kept on default download).
func isSourceArchiveAsset(asset api.ReleaseAsset) bool {
	if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return false
	}

	return strings.Contains(asset.BrowserDownloadURL, "/archive/refs/heads/")
}

func filterAssets(assets []api.ReleaseAsset, names []string) []api.ReleaseAsset {
	var result []api.ReleaseAsset
	for _, asset := range assets {
		for _, name := range names {
			if asset.Name == name {
				result = append(result, asset)
				break
			}
		}
	}
	return result
}

func filterSourceArchives(assets []api.ReleaseAsset) []api.ReleaseAsset {
	var result []api.ReleaseAsset
	for _, asset := range assets {
		// Skip auto-generated source archives, identified reliably by their
		// browser download URL (see isSourceArchiveAsset). Normal release
		// assets are never filtered, even when their names contain "v" and ".".
		if isSourceArchiveAsset(asset) {
			continue
		}
		result = append(result, asset)
	}
	return result
}

func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}

// canonicalDir resolves a path to its absolute canonical form,
// resolving any symbolic links.
func canonicalDir(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}
	return absPath, nil
}

// sanitizeAssetName validates an asset name for safe filesystem operations.
// It rejects empty names, absolute paths, path separators, and path traversal sequences.
func sanitizeAssetName(name string) (string, error) {
	// 1. Reject empty or whitespace-only names
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("asset name cannot be empty")
	}

	// 2. Reject absolute paths (Unix / or Windows drive letter)
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("asset name cannot be an absolute path")
	}

	// 3. Reject names containing path separators
	// This prevents creation of nested directories and path traversal attacks
	if strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("asset name cannot contain path separators")
	}

	// 4. Clean the name and check for invalid values
	cleaned := filepath.Clean(name)

	// 5. Reject if cleaned to "." or ".."
	if cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("invalid asset name")
	}

	return cleaned, nil
}

// safeOutputPath validates and constructs a safe output path within outputDir.
// It ensures the final path cannot escape the output directory.
func safeOutputPath(outputDir, assetName string) (string, error) {
	// 1. Resolve outputDir to canonical absolute path
	outputDirAbs, err := canonicalDir(outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve output directory: %w", err)
	}

	// 2. Sanitize the asset name
	safeName, err := sanitizeAssetName(assetName)
	if err != nil {
		return "", err
	}

	// 3. Construct the target path
	targetPath := filepath.Join(outputDirAbs, safeName)

	// 4. Clean and validate the final path
	targetAbs := filepath.Clean(targetPath)

	// 5. Verify the target is inside outputDir using filepath.Rel
	rel, err := filepath.Rel(outputDirAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("failed to validate output path: %w", err)
	}

	// 6. Reject if relative path starts with ".." (traversal detected)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("asset name would write outside output directory")
	}

	return targetAbs, nil
}
