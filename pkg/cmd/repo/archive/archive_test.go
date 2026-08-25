package archive

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdArchive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "archive with yes", args: []string{"owner/repo", "--yes"}, wantErr: false},
		{name: "archive dry-run", args: []string{"owner/repo", "--dry-run"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdArchive(f, func(opts *Options) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunArchiveWithYes(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	testStatusChange(t, "owner/repo", statusArchived, "archive", "archived", 2)
}

func TestRunUnarchiveWithYes(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	testStatusChange(t, "owner/repo", statusActive, "unarchive", "unarchived", 0)
}

func testStatusChange(t *testing.T, repo string, status int, verb, pastTense string, wantStatus int) {
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	var capturedMethod, capturedPath, capturedBody string
	opts := &Options{
		IO:         f.IOStreams,
		Repository: repo,
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					capturedMethod = req.Method
					capturedPath = req.URL.Path
					if req.Body != nil {
						b, _ := io.ReadAll(req.Body)
						capturedBody = string(b)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"code":1,"msg":"success"}`)),
					}, nil
				}),
			}, nil
		},
	}

	if err := run(opts, status, verb, pastTense); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if capturedMethod != "PUT" {
		t.Errorf("method = %q, want PUT", capturedMethod)
	}
	if !strings.Contains(capturedPath, "/org/owner/repo/repo/status") {
		t.Errorf("path = %q, want contains /org/owner/repo/repo/status", capturedPath)
	}
	var body struct {
		Status int `json:"status"`
	}
	if err := json.Unmarshal([]byte(capturedBody), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", capturedBody, err)
	}
	if body.Status != wantStatus {
		t.Errorf("body status = %d, want %d", body.Status, wantStatus)
	}
}

func TestRunArchiveDryRun(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		DryRun:     true,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
	}
	if err := run(opts, statusArchived, "archive", "archived"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), "Dry run: would archive repository owner/repo") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunArchiveRequiresConfirmation(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Yes:        false,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request to %s", req.URL.String())
					return nil, nil
				}),
			}, nil
		},
	}
	err := run(opts, statusArchived, "archive", "archived")
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("err = %v, want confirmation required", err)
	}
}

func TestRunArchiveNoTokenReturnsAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Yes:        true,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
	}
	err := run(opts, statusArchived, "archive", "archived")
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("err = %v, want not authenticated", err)
	}
}

func TestRunArchiveAPIErrorReturnsError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"message":"repo not found"}`)),
					}, nil
				}),
			}, nil
		},
	}
	err := run(opts, statusArchived, "archive", "archived")
	if err == nil || !strings.Contains(err.Error(), "failed to archive repository") {
		t.Fatalf("err = %v, want failed to archive repository", err)
	}
}

func TestRunArchiveParseRepoError(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &Options{
		IO:         f.IOStreams,
		Repository: "invalid-no-slash",
		Yes:        true,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
	}
	err := run(opts, statusArchived, "archive", "archived")
	if err == nil {
		t.Fatal("err = nil, want parse repo error")
	}
}

func TestNewCmdUnarchive(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdUnarchive(f, func(opts *Options) error { return nil })
	cmd.SetArgs([]string{"owner/repo", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestRunArchiveJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Yes:        true,
		JSON:       true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":1,"msg":"success"}`))}, nil
				}),
			}, nil
		},
	}
	if err := run(opts, statusArchived, "archive", "archived"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), `"action": "archived"`) || !strings.Contains(out.String(), `"repository": "owner/repo"`) {
		t.Fatalf("json output = %q", out.String())
	}
}

func TestRunArchiveDryRunJSON(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		DryRun:     true,
		JSON:       true,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
	}
	if err := run(opts, statusArchived, "archive", "archived"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), `"action": "dry_run"`) {
		t.Fatalf("json output = %q", out.String())
	}
}

func TestRunArchiveHttpClientError(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &Options{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Yes:        true,
		HttpClient: func() (*http.Client, error) { return nil, errors.New("dial fail") },
	}
	err := run(opts, statusArchived, "archive", "archived")
	if err == nil || !strings.Contains(err.Error(), "failed to create HTTP client") {
		t.Fatalf("err = %v, want failed to create HTTP client", err)
	}
}
