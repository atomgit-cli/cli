package download

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdDownload(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "download with id", args: []string{"art-1"}, wantErr: false},
		{name: "download with output", args: []string{"-o", "out.zip", "art-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdDownload(f, func(opts *DownloadOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmdDownloadOutputFlag(t *testing.T) {
	t.Parallel()
	cmd := NewCmdDownload(cmdutil.TestFactory(), func(opts *DownloadOptions) error { return nil })
	if cmd.Flags().Lookup("output") == nil {
		t.Fatal("output flag missing")
	}
}

func TestDownloadRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotPath string
	outFile := filepath.Join(t.TempDir(), "test.zip")
	opts := &DownloadOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return downloadTestResponse(http.StatusOK, zipContent()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		Output:     outFile,
	}
	if err := downloadRun(opts); err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	want := "/api/v8/repos/owner/repo/actions/artifacts/art-1/zip"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if strings.Contains(gotPath, "access_token=") {
		t.Fatalf("path unexpectedly contains access_token: %q", gotPath)
	}
}

func TestDownloadRunOutputFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	body := zipContent()
	outFile := filepath.Join(t.TempDir(), "artifact.zip")
	opts := &DownloadOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return downloadTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		Output:     outFile,
	}
	if err := downloadRun(opts); err != nil {
		t.Fatalf("downloadRun() error = %v", err)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(got) != body {
		t.Fatalf("file content mismatch: got len %d, want len %d", len(got), len(body))
	}
}

func TestDownloadRunRefusesTTYWithoutOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.TestTTY()
	apiCalled := false
	opts := &DownloadOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					apiCalled = true
					return downloadTestResponse(http.StatusOK, zipContent()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
	}
	err := downloadRun(opts)
	if err == nil {
		t.Fatal("downloadRun() error = nil, want usage error on TTY without --output")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitUsage)
	}
	if apiCalled {
		t.Fatal("API was called; the TTY guard should refuse before downloading")
	}
}

func TestDownloadRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &DownloadOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return downloadTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "missing",
		Output:     filepath.Join(t.TempDir(), "test.zip"),
	}
	err := downloadRun(opts)
	if err == nil {
		t.Fatal("downloadRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitNotFound)
	}
}

func downloadTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func zipContent() string {
	return "PK\x03\x04" + strings.Repeat("\x00", 20) + "fake-zip-content"
}
