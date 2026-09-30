package delete

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdDelete(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "delete with tag",
			args:    []string{"v1.0.0"},
			wantErr: false,
		},
		{
			name:    "delete with yes flag",
			args:    []string{"v1.0.0", "--yes"},
			wantErr: false,
		},
		{
			name:    "no tag specified",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdDelete(f, func(opts *DeleteOptions) error {
				return nil
			})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteBaseRepoInjected(t *testing.T) {
	f := cmdutil.TestFactory()
	var capturedBaseRepo func() (string, error)
	cmd := NewCmdDelete(f, func(opts *DeleteOptions) error {
		capturedBaseRepo = opts.BaseRepo
		return nil
	})
	cmd.SetArgs([]string{"v1.0.0", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if capturedBaseRepo == nil {
		t.Fatal("opts.BaseRepo is nil, want injected from f.BaseRepo")
	}
}

func TestDeleteRunDryRunMakesNoRequest(t *testing.T) {
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &DeleteOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		DryRun:     true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request to %s", req.URL.String())
					return nil, nil
				}),
			}, nil
		},
	}

	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Dry run: would delete release v1.0.0") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDeleteRun405ReturnsWebGuidance(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &DeleteOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == "GET" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body: io.NopCloser(strings.NewReader(
								`{"tag_name":"v1.0.0","name":"rel","html_url":"https://gitcode.com/owner/repo/-/releases/v1.0.0"}`)),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusMethodNotAllowed,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(``)),
					}, nil
				}),
			}, nil
		},
	}

	err := deleteRun(opts)
	if err == nil {
		t.Fatal("deleteRun() error = nil, want 405 error")
	}
	for _, want := range []string{"405", "web UI", "https://gitcode.com/owner/repo/-/releases/v1.0.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want contains %q", err.Error(), want)
		}
	}
	if strings.Contains(out.String(), "Deleted release") {
		t.Errorf("output = %q, want no success line", out.String())
	}
}

func TestDeleteRun405WithoutHTMLURLFallsBack(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &DeleteOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == "GET" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.0.0","name":"rel"}`)),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusMethodNotAllowed,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(``)),
					}, nil
				}),
			}, nil
		},
	}

	err := deleteRun(opts)
	if err == nil {
		t.Fatal("deleteRun() error = nil, want 405 error")
	}
	if !strings.Contains(err.Error(), "Delete the release from the web UI instead.") {
		t.Fatalf("error = %q, want fallback guidance without URL", err.Error())
	}
}

// Success path: the platform does not support deletes today, but if it
// ever enables the endpoint this is the only lock on the success output.
func TestDeleteRunSucceeds(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	var deletePath string
	opts := &DeleteOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == "GET" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body: io.NopCloser(strings.NewReader(
								`{"tag_name":"v1.0.0","name":"rel","html_url":"https://gitcode.com/owner/repo/-/releases/v1.0.0","id":123}`)),
						}, nil
					}
					deletePath = req.URL.Path
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(``)),
					}, nil
				}),
			}, nil
		},
	}

	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	if !strings.Contains(deletePath, "/releases/tags/v1.0.0") && !strings.Contains(deletePath, "/releases/") {
		t.Errorf("delete path = %q, want a release delete endpoint", deletePath)
	}
	if !strings.Contains(out.String(), "Deleted release v1.0.0") {
		t.Fatalf("output = %q, want success line", out.String())
	}
}

func TestDeleteRunReleaseNotFoundIsWrapped(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &DeleteOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(``)),
					}, nil
				}),
			}, nil
		},
	}

	err := deleteRun(opts)
	if err == nil {
		t.Fatal("deleteRun() error = nil, want not-found error")
	}
	if !strings.Contains(err.Error(), "release v1.0.0 not found in owner/repo") {
		t.Fatalf("error = %q, want wrapped not-found message", err.Error())
	}
	if code := cmdutil.ExitCode(err); code != 3 {
		t.Errorf("exit code = %d, want 3 (not found)", code)
	}
}

func TestDeleteRunErrNoReleaseIDMessage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &DeleteOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Yes:        true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == "GET" {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.0.0","name":"rel"}`)),
						}, nil
					}
					// Tag-based deletion 404s and the fallback needs a
					// release ID, which the lookup response omitted.
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(``)),
					}, nil
				}),
			}, nil
		},
	}

	err := deleteRun(opts)
	if err == nil {
		t.Fatal("deleteRun() error = nil, want ErrNoReleaseID error")
	}
	if !strings.Contains(err.Error(), "tag-based endpoint unavailable") {
		t.Fatalf("error = %q, want tag-based endpoint unavailable", err.Error())
	}
}

func TestConfirmTitle(t *testing.T) {
	tests := []struct {
		name    string
		release api.Release
		want    string
	}{
		{name: "name equals tag", release: api.Release{TagName: "v1.0.0", Name: "v1.0.0"}, want: "v1.0.0"},
		{name: "name differs from tag", release: api.Release{TagName: "v1.0.0", Name: "Version 1.0.0"}, want: "Version 1.0.0 (tag: v1.0.0)"},
		{name: "empty name falls back to tag", release: api.Release{TagName: "v1.0.0"}, want: "v1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := confirmTitle(&tt.release); got != tt.want {
				t.Errorf("confirmTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
