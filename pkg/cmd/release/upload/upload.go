// Package upload implements the release upload command
package upload

import (
	"fmt"
	"io"
	"mime"
	"net/http"
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

type UploadOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	// Arguments
	TagName string
	Files   []string

	// Flags
	Repository string
	Label      string
	JSON       bool
}

type uploadResult struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int    `json:"size"`
	ContentType string `json:"content_type"`
}

// NewCmdUpload creates the upload command
func NewCmdUpload(f *cmdutil.Factory, runF func(*UploadOptions) error) *cobra.Command {
	opts := &UploadOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "upload <tag> <file>...",
		Short: "Upload assets to a release",
		Long: heredoc.Doc(`
			Upload files as release assets.

			You can upload multiple files at once.
		`),
		Example: heredoc.Doc(`
			# Upload a single file
			$ gc release upload v1.0.0 app.zip -R owner/repo

			# Upload multiple files
			$ gc release upload v1.0.0 app.zip checksum.txt -R owner/repo

			# Upload to a specific repository
			$ gc release upload v1.0.0 app.zip -R owner/repo

			# Output as JSON
			$ gc release upload v1.0.0 app.zip -R owner/repo --json
		`),
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.TagName = args[0]
			opts.Files = args[1:]

			if runF != nil {
				return runF(opts)
			}
			return uploadRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.Label, "label", "l", "", "Asset label")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func uploadRun(opts *UploadOptions) error {
	cs := opts.IO.ColorScheme()

	if opts.Label != "" {
		return cmdutil.NewUsageError("--label is not supported by the current GitCode release upload API")
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
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

	// Same-name precheck: the platform's overwrite behavior for duplicate
	// asset names is undefined; refuse the whole batch before any upload so
	// a rerun cannot half-clobber an existing release.
	release, err := api.GetRelease(client, owner, repo, opts.TagName)
	if err != nil {
		return cmdutil.WrapNotFound(err, "release %s not found in %s/%s", opts.TagName, owner, repo)
	}
	existing := map[string]bool{}
	for _, asset := range release.Assets {
		existing[asset.Name] = true
	}
	var conflicts []string
	for _, file := range opts.Files {
		if name := filepath.Base(file); existing[name] {
			conflicts = append(conflicts, name)
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return fmt.Errorf("release %s already has asset(s): %s\nremove them with \"gc release delete-asset\" first, or upload files with different names",
			opts.TagName, strings.Join(conflicts, ", "))
	}

	results := make([]uploadResult, 0, len(opts.Files))

	// Upload each file using two-step process
	for _, file := range opts.Files {
		result, err := uploadFile(client, owner, repo, opts.TagName, file, opts.Label, cs, opts.IO.Out, !opts.JSON)
		if err != nil {
			// A partial batch must not look like a clean failure: name what
			// already landed so the user can clean up or resume knowingly.
			if len(results) > 0 {
				uploaded := make([]string, 0, len(results))
				for _, r := range results {
					uploaded = append(uploaded, r.Name)
				}
				err = fmt.Errorf("%w\nalready uploaded in this run: %s", err, strings.Join(uploaded, ", "))
			}
			return err
		}
		results = append(results, result)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, results)
	}

	return nil
}

func uploadFile(client *api.Client, owner, repo, tag, filePath, label string, cs *iostreams.ColorScheme, out io.Writer, writeText bool) (uploadResult, error) {
	// Open file; the upload streams from the handle instead of buffering
	// the whole asset in memory.
	file, err := os.Open(filePath)
	if err != nil {
		return uploadResult{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return uploadResult{}, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return uploadResult{}, fmt.Errorf("cannot upload a directory: %s", filePath)
	}

	// Get filename
	filename := filepath.Base(filePath)

	// Detect content type (sniffs the head and rewinds the handle)
	contentType, err := detectContentType(filename, file)
	if err != nil {
		return uploadResult{}, fmt.Errorf("failed to detect content type: %w", err)
	}

	// Upload using two-step process
	err = api.UploadReleaseAssetByTag(client, owner, repo, tag, filename, file, info.Size(), contentType)
	if err != nil {
		return uploadResult{}, fmt.Errorf("failed to upload %s: %w", filename, err)
	}

	result := uploadResult{
		Name:        filename,
		Path:        filePath,
		Size:        int(info.Size()),
		ContentType: contentType,
	}

	if writeText {
		fmt.Fprintf(out, "%s Uploaded %s (%s)\n", cs.Green("✓"), filename, formatSize(int(info.Size())))
	}

	return result, nil
}

// detectContentType resolves the asset content type: the extension table
// first, then a 512-byte sniff via http.DetectContentType. The reader is a
// ReadSeeker and is rewound after the sniff so the upload streams from the
// start.
func detectContentType(filename string, content io.ReadSeeker) (string, error) {
	// Try to detect from file extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".zip":
		return "application/zip", nil
	case ".tar":
		return "application/x-tar", nil
	case ".gz", ".tgz":
		return "application/gzip", nil
	case ".bz2":
		return "application/x-bzip2", nil
	case ".xz":
		return "application/x-xz", nil
	case ".deb":
		return "application/vnd.debian.binary-package", nil
	case ".rpm":
		return "application/x-rpm", nil
	case ".dmg":
		return "application/x-apple-diskimage", nil
	case ".exe":
		return "application/vnd.microsoft.portable-executable", nil
	case ".msi":
		return "application/x-msi", nil
	case ".apk":
		return "application/vnd.android.package-archive", nil
	case ".pdf":
		return "application/pdf", nil
	case ".txt", ".md":
		return "text/plain", nil
	case ".json":
		return "application/json", nil
	case ".yaml", ".yml":
		return "application/x-yaml", nil
	case ".xml":
		return "application/xml", nil
	}

	// Try to detect from content: read the head, sniff, rewind.
	head := make([]byte, 512)
	n, err := content.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := content.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if ct := http.DetectContentType(head[:n]); ct != "application/octet-stream" {
		return ct, nil
	}

	// Use mime.TypeByExtension
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct, nil
	}

	return "application/octet-stream", nil
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
