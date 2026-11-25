package list

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

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "valid", args: []string{"--org", "my-org"}, wantErr: false},
		{name: "missing org", args: []string{}, wantErr: true},
		{name: "json", args: []string{"--org", "my-org", "--json"}, wantErr: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdList(f, func(opts *ListOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"a","number":1,"title":"release plan","comment_total":3,"is_closed":0,"author":{"login":"alice"}},{"id":"b","number":2,"title":"Q&A","comment_total":0,"is_closed":1,"author":{"login":"bob"}}]`
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
	err := listRun(&ListOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss", gotPath)
	}
	if !strings.Contains(out.String(), "release plan") || !strings.Contains(out.String(), "Q&A") {
		t.Fatalf("output = %q, want both titles", out.String())
	}
	if !strings.Contains(out.String(), "#1") || !strings.Contains(out.String(), "#2") {
		t.Fatalf("output = %q, want discussion numbers", out.String())
	}
}

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"a","number":1,"title":"release plan","comment_total":3}]`
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
	err := listRun(&ListOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Limit:      30,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(out.String(), `"release plan"`) || !strings.Contains(out.String(), `"number": 1`) {
		t.Fatalf("output = %q, want JSON with title and number", out.String())
	}
}

func TestListRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[]`)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := listRun(&ListOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Org:        "my-org",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No discussions found") {
		t.Fatalf("output = %q, want 'No discussions found'", out.String())
	}
}

func TestListRunValidationErrors(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	for _, opts := range []ListOptions{
		{Org: "", Page: 1, Limit: 30},                // missing org
		{Org: "o", Page: -1, Limit: 30},              // page < 0
		{Org: "o", Limit: 0, Page: 1},                // limit <= 0
		{Org: "o", PerPage: 101, Limit: 30},          // per_page > 100
		{Org: "o", Sort: "bogus", Limit: 30},         // bad sort
		{Org: "o", Direction: "sideways", Limit: 30}, // bad direction
	} {
		err := listRun(&opts)
		if err == nil {
			t.Fatalf("listRun(%+v) error = nil, want usage error", opts)
		}
		if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
			t.Fatalf("ExitCode = %d, want ExitUsage(%d)", got, cmdutil.ExitUsage)
		}
	}
}
