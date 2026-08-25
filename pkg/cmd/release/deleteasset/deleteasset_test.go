package deleteasset

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdDeleteAsset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "tag+id", args: []string{"v1.0.0", "12345", "-R", "o/r"}, wantErr: false},
		{name: "with yes", args: []string{"v1.0.0", "12345", "-R", "o/r", "--yes"}, wantErr: false},
		{name: "dry-run", args: []string{"v1.0.0", "12345", "-R", "o/r", "--dry-run"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "one arg", args: []string{"v1.0.0"}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdDeleteAsset(f, func(opts *DeleteAssetOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteAssetRunWithYes(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	var capturedMethod, capturedPath string
	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		AttachFileID: "12345",
		Yes:          true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					capturedMethod = req.Method
					capturedPath = req.URL.Path
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       ioNopCloser(``),
					}, nil
				}),
			}, nil
		},
	}

	if err := deleteAssetRun(opts); err != nil {
		t.Fatalf("deleteAssetRun() error = %v", err)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if !strings.Contains(capturedPath, "/releases/v1.0.0/attach_files/12345") {
		t.Errorf("path = %q, want contains /releases/v1.0.0/attach_files/12345", capturedPath)
	}
	if !strings.Contains(out.String(), "Deleted asset 12345") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDeleteAssetRunDryRun(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		AttachFileID: "12345",
		DryRun:       true,
		HttpClient:   func() (*http.Client, error) { return &http.Client{}, nil },
	}

	if err := deleteAssetRun(opts); err != nil {
		t.Fatalf("deleteAssetRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Dry run: would delete asset 12345") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDeleteAssetRunRequiresConfirmationInNonInteractiveMode(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		AttachFileID: "12345",
		Yes:          false,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request to %s", req.URL.String())
					return nil, nil
				}),
			}, nil
		},
	}

	err := deleteAssetRun(opts)
	if err == nil {
		t.Fatal("deleteAssetRun() error = nil, want confirmation error")
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("error = %q, want confirmation required", err.Error())
	}
}

func TestDeleteAssetRunAPIErrorReturnsError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		AttachFileID: "999",
		Yes:          true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"asset not found"}`),
					}, nil
				}),
			}, nil
		},
	}

	err := deleteAssetRun(opts)
	if err == nil {
		t.Fatal("deleteAssetRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want not found", err.Error())
	}
}

func TestDeleteAssetRunParseRepoError(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "invalid-no-slash",
		TagName:      "v1.0.0",
		AttachFileID: "12345",
		Yes:          true,
		HttpClient:   func() (*http.Client, error) { return &http.Client{}, nil },
	}
	err := deleteAssetRun(opts)
	if err == nil {
		t.Fatal("deleteAssetRun() error = nil, want parse repo error")
	}
}

func TestDeleteAssetRunHttpClientError(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		AttachFileID: "12345",
		Yes:          true,
		HttpClient:   func() (*http.Client, error) { return nil, errors.New("dial fail") },
	}
	err := deleteAssetRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to create HTTP client") {
		t.Fatalf("err = %v, want failed to create HTTP client", err)
	}
}

func TestDeleteAssetRunNoTokenReturnsAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	opts := &DeleteAssetOptions{
		IO:           f.IOStreams,
		Repository:   "owner/repo",
		TagName:      "v1.0.0",
		AttachFileID: "12345",
		Yes:          true,
		HttpClient:   func() (*http.Client, error) { return &http.Client{}, nil },
	}
	err := deleteAssetRun(opts)
	if err == nil {
		t.Fatal("deleteAssetRun() error = nil, want auth error")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("err = %v, want not authenticated", err)
	}
}

func ioNopCloser(body string) *readCloser {
	return &readCloser{Reader: strings.NewReader(body)}
}

type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error { return nil }
