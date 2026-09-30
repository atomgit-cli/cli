package download

import (
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestNewCmdDownloadAllowsLatestReleaseWithoutTag(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdDownload(f, func(opts *DownloadOptions) error {
		return nil
	})
	cmd.SetArgs([]string{"-R", "owner/repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestDownloadRunUsesLatestReleaseWhenTagOmitted(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	var gotPaths []string
	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			if req.URL.RawQuery != "" {
				path += "?" + req.URL.RawQuery
			}
			gotPaths = append(gotPaths, path)

			switch {
			case req.Method == http.MethodGet && path == "/api/v5/repos/owner/repo/releases/latest":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"tag_name":"v1.0.0",
						"assets":[{"name":"app.tar.gz"}]
					}`)),
				}, nil
			case req.Method == http.MethodGet && path == "/api/v5/repos/owner/repo/releases/v1.0.0/attach_files/app.tar.gz/download":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("asset-body")),
				}, nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, path)
				return nil, nil
			}
		}),
	}

	err := downloadRun(&DownloadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return httpClient, nil },
		Repository: "owner/repo",
		Output:     tempDir,
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	if len(gotPaths) < 2 || gotPaths[0] != "/api/v5/repos/owner/repo/releases/latest" {
		t.Fatalf("requests = %#v, want latest release lookup first", gotPaths)
	}
}

func TestDownloadRunLatestWithAllUsesBrowserDownloadURLForSourceArchives(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	var gotURLs []string
	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURLs = append(gotURLs, req.URL.String())

			switch req.URL.String() {
			case "https://api.gitcode.com/api/v5/repos/owner/repo/releases/latest":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"tag_name":"v1.0.0",
						"assets":[
							{
								"name":"v1.0.0.zip",
								"browser_download_url":"https://raw.gitcode.com/owner/repo/archive/refs/heads/v1.0.0.zip"
							},
							{
								"name":"asset.txt",
								"browser_download_url":"https://api.gitcode.com/owner/repo/releases/download/v1.0.0/asset.txt"
							}
						]
					}`)),
				}, nil
			case "https://raw.gitcode.com/owner/repo/archive/refs/heads/v1.0.0.zip":
				if got := req.Header.Get("Authorization"); got != "" {
					t.Fatalf("Authorization header for source archive = %q, want empty", got)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("zip-body")),
				}, nil
			case "https://api.gitcode.com/api/v5/repos/owner/repo/releases/v1.0.0/attach_files/asset.txt/download":
				if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Fatalf("Authorization header for uploaded asset = %q, want %q", got, "Bearer test-token")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("asset-body")),
				}, nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}
		}),
	}

	err := downloadRun(&DownloadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return httpClient, nil },
		Repository: "owner/repo",
		Output:     tempDir,
		All:        true,
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}

	if len(gotURLs) != 3 {
		t.Fatalf("requests = %#v, want latest lookup plus 2 asset downloads", gotURLs)
	}

	zipContent, err := os.ReadFile(filepath.Join(tempDir, "v1.0.0.zip"))
	if err != nil {
		t.Fatalf("ReadFile(v1.0.0.zip) error = %v", err)
	}
	if string(zipContent) != "zip-body" {
		t.Fatalf("zip content = %q, want %q", string(zipContent), "zip-body")
	}

	assetContent, err := os.ReadFile(filepath.Join(tempDir, "asset.txt"))
	if err != nil {
		t.Fatalf("ReadFile(asset.txt) error = %v", err)
	}
	if string(assetContent) != "asset-body" {
		t.Fatalf("asset content = %q, want %q", string(assetContent), "asset-body")
	}
}

func TestDownloadRunUsesConfiguredHostForReleaseAndAssetRequests(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "enterprise.example.com")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("enterprise.example.com", "tester", "stored-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	var gotURLs []string
	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURLs = append(gotURLs, req.URL.String())
			if got := req.Header.Get("Authorization"); got != "Bearer stored-token" {
				t.Fatalf("Authorization = %q, want stored token", got)
			}

			switch req.URL.String() {
			case "https://api.enterprise.example.com/api/v5/repos/owner/repo/releases/latest":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"tag_name":"v1.0.0",
						"assets":[{"name":"asset.txt"}]
					}`)),
				}, nil
			case "https://api.enterprise.example.com/api/v5/repos/owner/repo/releases/v1.0.0/attach_files/asset.txt/download":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("asset-body")),
				}, nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}
		}),
	}

	err := downloadRun(&DownloadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return httpClient, nil },
		Repository: "owner/repo",
		Output:     tempDir,
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	if len(gotURLs) != 2 {
		t.Fatalf("requests = %#v, want release lookup and asset download", gotURLs)
	}
}

func TestDownloadAssetUsesAuthorizationHeader(t *testing.T) {
	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	cs := ioStreams.ColorScheme()

	var gotPath string
	var gotAuth string
	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			if req.URL.RawQuery != "" {
				gotPath += "?" + req.URL.RawQuery
			}
			gotAuth = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("asset-body")),
			}, nil
		}),
	}

	client := api.NewClientFromHTTP(httpClient)
	client.SetToken("test-token", "test")

	err := downloadAsset(api.ReleaseAsset{Name: "app.tar.gz"}, tempDir, httpClient, cs, io.Discard, client, "owner", "repo", "v1.0.0", false, false)
	if err != nil {
		t.Fatalf("downloadAsset() error = %v", err)
	}

	if strings.Contains(gotPath, "access_token=") {
		t.Fatalf("request path unexpectedly contains access_token query: %q", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}

	outputPath := filepath.Join(tempDir, "app.tar.gz")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "asset-body" {
		t.Fatalf("downloaded content = %q, want %q", string(content), "asset-body")
	}
}

func TestDownloadAssetFallsBackToAttachFilesEndpointWithoutBrowserDownloadURL(t *testing.T) {
	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	cs := ioStreams.ColorScheme()

	var gotURL string
	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("asset-body")),
			}, nil
		}),
	}

	client := api.NewClientFromHTTP(httpClient)
	client.SetToken("test-token", "test")

	err := downloadAsset(api.ReleaseAsset{Name: "app.tar.gz"}, tempDir, httpClient, cs, io.Discard, client, "owner", "repo", "v1.0.0", false, false)
	if err != nil {
		t.Fatalf("downloadAsset() error = %v", err)
	}

	wantURL := "https://api.gitcode.com/api/v5/repos/owner/repo/releases/v1.0.0/attach_files/app.tar.gz/download"
	if gotURL != wantURL {
		t.Fatalf("download URL = %q, want %q", gotURL, wantURL)
	}
}

func TestDownloadAssetIgnoresNonArchiveBrowserDownloadURL(t *testing.T) {
	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	cs := ioStreams.ColorScheme()

	var gotURL string
	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("asset-body")),
			}, nil
		}),
	}

	client := api.NewClientFromHTTP(httpClient)
	client.SetToken("test-token", "test")

	err := downloadAsset(api.ReleaseAsset{
		Name:               "asset.txt",
		BrowserDownloadURL: "https://api.gitcode.com/owner/repo/releases/download/v1.0.0/asset.txt",
	}, tempDir, httpClient, cs, io.Discard, client, "owner", "repo", "v1.0.0", false, false)
	if err != nil {
		t.Fatalf("downloadAsset() error = %v", err)
	}

	wantURL := "https://api.gitcode.com/api/v5/repos/owner/repo/releases/v1.0.0/attach_files/asset.txt/download"
	if gotURL != wantURL {
		t.Fatalf("download URL = %q, want %q", gotURL, wantURL)
	}
}

// Security tests for path traversal prevention

func TestSanitizeAssetName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		{"valid simple name", "app.tar.gz", "app.tar.gz", false, ""},
		{"valid with spaces trimmed", "  app.zip  ", "app.zip", false, ""},
		{"valid with dots in filename", "file.name.txt", "file.name.txt", false, ""},
		{"empty name rejected", "", "", true, "asset name cannot be empty"},
		{"whitespace-only rejected", "   ", "", true, "asset name cannot be empty"},
		{"absolute path Unix rejected", "/tmp/outside.txt", "", true, "asset name cannot be an absolute path"},
		{"absolute path Windows rejected", "C:\\Windows\\outside.txt", "", true, ""},
		{"path traversal parent rejected", "../outside.txt", "", true, "asset name cannot contain path separators"},
		{"nested traversal rejected", "nested/../outside.txt", "", true, "asset name cannot contain path separators"},
		{"path separator slash rejected", "nested/file.txt", "", true, "asset name cannot contain path separators"},
		{"path separator backslash rejected", "nested\\file.txt", "", true, "asset name cannot contain path separators"},
		{"single dot rejected", ".", "", true, "invalid asset name"},
		{"double dot rejected", "..", "", true, "invalid asset name"},
		{"deep traversal rejected", "../../outside.txt", "", true, "asset name cannot contain path separators"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeAssetName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeAssetName(%q) expected error, got nil", tt.input)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("sanitizeAssetName(%q) error = %v, want containing %q", tt.input, err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("sanitizeAssetName(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("sanitizeAssetName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSafeOutputPath(t *testing.T) {
	tempDir := t.TempDir()
	// Resolve symlinks for canonical comparison (macOS /var → /private/var)
	if resolved, err := filepath.EvalSymlinks(tempDir); err == nil {
		tempDir = resolved
	}

	tests := []struct {
		name        string
		outputDir   string
		assetName   string
		wantErr     bool
		errContains string
	}{
		{"valid simple name", tempDir, "app.tar.gz", false, ""},
		{"valid with dots", tempDir, "file.name.txt", false, ""},
		{"traversal outside dir", tempDir, "../outside.txt", true, "path separators"},
		{"absolute path", tempDir, "/tmp/outside.txt", true, "absolute path"},
		{"nested traversal", tempDir, "nested/../outside.txt", true, "path separators"},
		{"deep traversal", tempDir, "../../outside.txt", true, "path separators"},
		{"path separator", tempDir, "nested/file.txt", true, "path separators"},
		{"empty name", tempDir, "", true, "cannot be empty"},
		{"single dot", tempDir, ".", true, "invalid asset name"},
		{"double dot", tempDir, "..", true, "invalid asset name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeOutputPath(tt.outputDir, tt.assetName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("safeOutputPath() expected error, got path %q", got)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("safeOutputPath() error = %v, want containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("safeOutputPath() unexpected error: %v", err)
				return
			}
			// Verify path is inside outputDir
			rel, err := filepath.Rel(tempDir, got)
			if err != nil {
				t.Errorf("filepath.Rel() error: %v", err)
				return
			}
			if rel == ".." || strings.HasPrefix(rel, "../") {
				t.Errorf("safeOutputPath() returned path outside output dir: %q", got)
			}
		})
	}
}

func TestDownloadAssetRejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	cs := ioStreams.ColorScheme()

	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("malicious-content")),
			}, nil
		}),
	}

	client := api.NewClientFromHTTP(httpClient)

	err := downloadAsset(api.ReleaseAsset{Name: "../outside.txt"}, tempDir, httpClient, cs, io.Discard, client, "owner", "repo", "v1.0.0", false, false)
	if err == nil {
		t.Fatal("downloadAsset() expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "invalid asset name") {
		t.Errorf("downloadAsset() error = %v, want containing 'invalid asset name'", err)
	}

	// Verify no file was created outside tempDir
	outsidePath := filepath.Join(filepath.Dir(tempDir), "outside.txt")
	if _, err := os.Stat(outsidePath); !os.IsNotExist(err) {
		t.Fatalf("file was created outside output directory: %s", outsidePath)
	}
}

func TestDownloadAssetRejectsAbsolutePath(t *testing.T) {
	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	cs := ioStreams.ColorScheme()

	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("malicious-content")),
			}, nil
		}),
	}

	client := api.NewClientFromHTTP(httpClient)

	err := downloadAsset(api.ReleaseAsset{Name: "/tmp/malicious.txt"}, tempDir, httpClient, cs, io.Discard, client, "owner", "repo", "v1.0.0", false, false)
	if err == nil {
		t.Fatal("downloadAsset() expected error for absolute path, got nil")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("downloadAsset() error = %v, want containing 'absolute path'", err)
	}
}

func TestDownloadAssetRejectsPathSeparator(t *testing.T) {
	tempDir := t.TempDir()
	ioStreams, _, _, _ := iostreams.Test()
	cs := ioStreams.ColorScheme()

	httpClient := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("malicious-content")),
			}, nil
		}),
	}

	client := api.NewClientFromHTTP(httpClient)

	err := downloadAsset(api.ReleaseAsset{Name: "nested/file.txt"}, tempDir, httpClient, cs, io.Discard, client, "owner", "repo", "v1.0.0", false, false)
	if err == nil {
		t.Fatal("downloadAsset() expected error for path separator, got nil")
	}
	if !strings.Contains(err.Error(), "path separators") {
		t.Errorf("downloadAsset() error = %v, want containing 'path separators'", err)
	}

	// Verify no nested directory was created
	nestedPath := filepath.Join(tempDir, "nested")
	if _, err := os.Stat(nestedPath); !os.IsNotExist(err) {
		t.Fatalf("nested directory was created inside output directory: %s", nestedPath)
	}
}

func TestFilterSourceArchives(t *testing.T) {
	// Regression for #285: the previous heuristic skipped any .zip/.tar* asset
	// whose name contained both "v" and ".", wrongly dropping normal assets.
	assets := []api.ReleaseAsset{
		// Real source archives: identified by the /archive/refs/heads/ URL.
		{
			Name:               "v1.0.0.zip",
			BrowserDownloadURL: "https://raw.gitcode.com/owner/repo/archive/refs/heads/v1.0.0.zip",
		},
		{
			Name:               "v1.0.0.tar.gz",
			BrowserDownloadURL: "https://raw.gitcode.com/owner/repo/archive/refs/heads/v1.0.0.tar.gz",
		},
		// Normal release assets whose names contain both "v" and "." — must be kept.
		{
			Name:               "server.tar.gz",
			BrowserDownloadURL: "https://api.gitcode.com/owner/repo/releases/download/v1.0.0/server.tar.gz",
		},
		{
			Name:               "driver.zip",
			BrowserDownloadURL: "https://api.gitcode.com/owner/repo/releases/download/v1.0.0/driver.zip",
		},
		{
			Name:               "service.zip",
			BrowserDownloadURL: "https://api.gitcode.com/owner/repo/releases/download/v1.0.0/service.zip",
		},
		{
			// Control sample (not a regression guard): plain .txt never triggered
			// the old heuristic; included to confirm ordinary assets pass through.
			Name:               "checksums.txt",
			BrowserDownloadURL: "https://api.gitcode.com/owner/repo/releases/download/v1.0.0/checksums.txt",
		},
		{
			// Edge case: a .zip asset with no browser_download_url. isSourceArchiveAsset
			// treats an empty URL as "not a source archive", so it must be kept.
			Name:               "noturl.zip",
			BrowserDownloadURL: "",
		},
		{
			// Edge case: detection is URL-only and ignores the file extension — an
			// archive URL with an extension-less name must still be filtered.
			Name:               "source",
			BrowserDownloadURL: "https://raw.gitcode.com/owner/repo/archive/refs/heads/main",
		},
	}

	got := filterSourceArchives(assets)

	gotNames := make(map[string]bool, len(got))
	for _, a := range got {
		gotNames[a.Name] = true
	}

	wantKept := []string{"server.tar.gz", "driver.zip", "service.zip", "checksums.txt", "noturl.zip"}
	for _, name := range wantKept {
		if !gotNames[name] {
			t.Errorf("filterSourceArchives() dropped normal asset %q, want it kept", name)
		}
	}

	wantFiltered := []string{"v1.0.0.zip", "v1.0.0.tar.gz", "source"}
	for _, name := range wantFiltered {
		if gotNames[name] {
			t.Errorf("filterSourceArchives() kept source archive %q, want it filtered", name)
		}
	}

	if len(got) != len(wantKept) {
		t.Errorf("filterSourceArchives() returned %d assets, want %d", len(got), len(wantKept))
	}
}

// run-level fixture: one release with a normal asset and a source archive.
func downloadFixtureClient(t *testing.T, requests *[]string) *http.Client {
	t.Helper()
	return &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			if req.URL.RawQuery != "" {
				path += "?" + req.URL.RawQuery
			}
			if requests != nil {
				*requests = append(*requests, req.Method+" "+path)
			}
			switch {
			case req.Method == http.MethodGet && path == "/api/v5/repos/owner/repo/releases/tags/v1.0.0":
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"tag_name":"v1.0.0",
						"assets":[
							{"name":"app.tar.gz"},
							{"name":"v1.0.0.zip","browser_download_url":"https://raw.gitcode.com/owner/repo/archive/refs/heads/v1.0.0.zip"}
						]
					}`)),
				}, nil
			case req.Method == http.MethodGet && (strings.HasSuffix(path, "/download") || strings.Contains(path, "/archive/refs/heads/")):
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("asset-body")),
				}, nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, path)
				return nil, nil
			}
		}),
	}
}

func TestDownloadRunRefusesExistingFileByDefault(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "app.tar.gz"), []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	var requests []string
	client := downloadFixtureClient(t, &requests)

	err := downloadRun(&DownloadOptions{
		IO:         viewTestIO(),
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Output:     tempDir,
		Assets:     []string{"app.tar.gz"},
	})
	if err == nil || !strings.Contains(err.Error(), "file already exists") || !strings.Contains(err.Error(), "--clobber") {
		t.Fatalf("error = %v, want already-exists error mentioning --clobber", err)
	}
	// The refusal happens before any download request.
	for _, r := range requests {
		if strings.HasSuffix(r, "/download") {
			t.Errorf("unexpected download request: %s", r)
		}
	}
	// The pre-existing content is untouched.
	data, _ := os.ReadFile(filepath.Join(tempDir, "app.tar.gz"))
	if string(data) != "previous" {
		t.Fatalf("existing file was modified: %q", data)
	}
}

func TestDownloadRunSkipExistingSkipsOnlyPresentFiles(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "app.tar.gz"), []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	var requests []string
	client := downloadFixtureClient(t, &requests)

	ioStreams, _, out, _ := iostreams.Test()
	err := downloadRun(&DownloadOptions{
		IO:           ioStreams,
		HttpClient:   func() (*http.Client, error) { return client, nil },
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		Output:       tempDir,
		All:          true,
		SkipExisting: true,
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Skipped app.tar.gz (exists)") {
		t.Errorf("output = %q, want skip notice", out.String())
	}
	if !strings.Contains(out.String(), "Downloaded v1.0.0.zip") {
		t.Errorf("output = %q, want the other asset downloaded", out.String())
	}
	data, _ := os.ReadFile(filepath.Join(tempDir, "app.tar.gz"))
	if string(data) != "previous" {
		t.Fatalf("skipped file was modified: %q", data)
	}
}

func TestDownloadRunClobberOverwrites(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "app.tar.gz"), []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := downloadFixtureClient(t, nil)

	err := downloadRun(&DownloadOptions{
		IO:         viewTestIO(),
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Output:     tempDir,
		Assets:     []string{"app.tar.gz"},
		Clobber:    true,
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(tempDir, "app.tar.gz"))
	if string(data) != "asset-body" {
		t.Fatalf("file content = %q, want overwritten with download", data)
	}
}

func TestDownloadClobberAndSkipExistingAreMutuallyExclusive(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdDownload(f, nil)
	cmd.SetArgs([]string{"v1.0.0", "app.tar.gz", "-R", "owner/repo", "--clobber", "--skip-existing"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want mutual-exclusion error")
	}
	// Pin the cobra flag-group error: a bare err != nil also passes when
	// the flags are merely incompatible for unrelated reasons (e.g. auth).
	if !strings.Contains(err.Error(), "flags in the group") {
		t.Fatalf("error = %v, want cobra mutually-exclusive flags error", err)
	}
}

func TestDownloadRunNamedAssetMissingIsAnError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	client := downloadFixtureClient(t, nil)

	err := downloadRun(&DownloadOptions{
		IO:         viewTestIO(),
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Output:     tempDir,
		Assets:     []string{"missing.txt"},
	})
	if err == nil || !strings.Contains(err.Error(), "missing.txt") || !strings.Contains(err.Error(), "available assets") {
		t.Fatalf("error = %v, want missing-name error with available list", err)
	}
	if !strings.Contains(err.Error(), "app.tar.gz") {
		t.Errorf("error = %v, want available list to name app.tar.gz", err)
	}
}

func TestDownloadRunPartiallyNamedAssetsStillError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	client := downloadFixtureClient(t, nil)

	err := downloadRun(&DownloadOptions{
		IO:         viewTestIO(),
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Output:     tempDir,
		Assets:     []string{"app.tar.gz", "missing.txt"},
	})
	if err == nil || !strings.Contains(err.Error(), "missing.txt") {
		t.Fatalf("error = %v, want partial miss to fail the batch", err)
	}
	if _, statErr := os.Stat(filepath.Join(tempDir, "app.tar.gz")); statErr == nil {
		t.Error("matched asset should not be downloaded when a sibling name misses")
	}
}

func TestDownloadRunNamedSourceArchiveBypassesFilter(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	client := downloadFixtureClient(t, nil)

	err := downloadRun(&DownloadOptions{
		IO:         viewTestIO(),
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Output:     tempDir,
		Assets:     []string{"v1.0.0.zip"},
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tempDir, "v1.0.0.zip")); statErr != nil {
		t.Errorf("named source archive not downloaded: %v", statErr)
	}
}

func TestDownloadRunUnnamedSourceArchiveStaysFiltered(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	tempDir := t.TempDir()
	client := downloadFixtureClient(t, nil)

	ioStreams, _, out, _ := iostreams.Test()
	err := downloadRun(&DownloadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Output:     tempDir,
	})
	if err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	// Unnamed + no --all: the archive is filtered, the normal asset stays.
	if !strings.Contains(out.String(), "Downloaded app.tar.gz") {
		t.Errorf("output = %q, want the normal asset downloaded", out.String())
	}
	if strings.Contains(out.String(), "v1.0.0.zip") {
		t.Errorf("output = %q, want the source archive filtered out", out.String())
	}
}

func viewTestIO() *iostreams.IOStreams {
	ioStreams, _, _, _ := iostreams.Test()
	return ioStreams
}
