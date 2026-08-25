package view

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "view artifact", args: []string{"art-1"}, wantErr: false},
		{name: "view with json", args: []string{"--json", "art-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdView(f, func(opts *ViewOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmdViewJSONFlag(t *testing.T) {
	t.Parallel()
	cmd := NewCmdView(cmdutil.TestFactory(), func(opts *ViewOptions) error { return nil })
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("json flag missing")
	}
}

func TestViewRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return viewTestResponse(http.StatusOK, artifactDetailJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
	}
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	want := "/api/v8/repos/owner/repo/actions/artifacts/art-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if strings.Contains(gotPath, "access_token=") {
		t.Fatalf("path unexpectedly contains access_token: %q", gotPath)
	}
}

func TestViewRunJSONIsFaithful(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	body := artifactDetailJSON()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		JSON:       true,
	}
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if out.String() != body+"\n" {
		t.Fatalf("JSON output not faithful verbatim: got len %d, want len %d", len(out.String()), len(body)+1)
	}
}

func TestViewRunHumanRendering(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, artifactDetailJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
	}
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"build-output", "artifact id", "size", "1.0 MiB", "digest", "2026-", "created", "expires"} {
		want := want
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q; output=%s", want, got)
		}
	}
}

func TestViewRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "missing",
	}
	err := viewRun(opts)
	if err == nil {
		t.Fatal("viewRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (404 preserved through %%w)", got, cmdutil.ExitNotFound)
	}
}

func viewTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func artifactDetailJSON() string {
	return `{
		"id":"art-1",
		"name":"build-output",
		"size_bytes":1048576,
		"workflow_id":"wf-1",
		"workflow_run_id":"run-1",
		"digest":"sha256:abc",
		"expires_at":"1783587145000",
		"created_at":"1783500745000",
		"updated_at":"1783500745000"
	}`
}
