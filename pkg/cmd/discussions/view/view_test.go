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

func httpFactory(client *http.Client) func() (*http.Client, error) {
	return func() (*http.Client, error) { return client, nil }
}

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "valid", args: []string{"42", "--org", "my-org"}, wantErr: false},
		{name: "missing org", args: []string{"42"}, wantErr: true},
		{name: "non-numeric", args: []string{"abc", "--org", "my-org"}, wantErr: true},
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

func TestViewRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `{"id":"a","number":42,"title":"release plan","md_content":"body text","comment_total":3,"is_closed":0,"author":{"login":"alice"},"created_at":"2026-07-13T10:00:00Z","updated_at":"2026-07-14T11:00:00Z"}`
	var gotPath string
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := viewRun(&ViewOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Number:     42,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/42" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/42", gotPath)
	}
	if !strings.Contains(out.String(), "release plan") {
		t.Fatalf("output = %q, want title", out.String())
	}
	if !strings.Contains(out.String(), "body text") {
		t.Fatalf("output = %q, want md_content body", out.String())
	}
	if !strings.Contains(out.String(), "alice") {
		t.Fatalf("output = %q, want author login", out.String())
	}
}

func TestViewRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `{"id":"a","number":42,"title":"release plan","comment_total":3}`
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := viewRun(&ViewOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Number:     42,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if !strings.Contains(out.String(), `"release plan"`) || !strings.Contains(out.String(), `"number": 42`) {
		t.Fatalf("output = %q, want JSON with title and number", out.String())
	}
}

func TestViewRunNumberMustBePositive(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ios, _, _, _ := iostreams.Test()
	err := viewRun(&ViewOptions{
		IO:         ios,
		HttpClient: httpFactory(&http.Client{}),
		Org:        "my-org",
		Number:     0, // itoa(0) would otherwise build /discuss/30
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want usage error for number < 1")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode = %d, want ExitUsage(%d)", got, cmdutil.ExitUsage)
	}
}

func TestViewRunNilAuthorText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `{"id":"a","number":42,"title":"no author","md_content":"body","comment_total":0,"is_closed":0}`
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := viewRun(&ViewOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Number:     42,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "no author") {
		t.Fatalf("output = %q, want title even with nil author", out.String())
	}
}

func TestViewRunNotFoundExit3(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     http.StatusText(http.StatusNotFound),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
			}, nil
		}),
	}
	ios, _, _, _ := iostreams.Test()
	err := viewRun(&ViewOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Number:     999,
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want not-found error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want ExitNotFound(%d)", got, cmdutil.ExitNotFound)
	}
}
