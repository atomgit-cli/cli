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
	f := cmdutil.TestFactory()
	cmd := NewCmdList(f, func(opts *ListOptions) error { return nil })
	cmd.SetArgs([]string{"42", "--org", "my-org"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}
}

func TestListRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"c1","content":"hello","md_content":"hello","author":{"login":"alice"},"like_total":2,"reply_total":1,"created_at":"2026-07-13T10:00:00Z"}]`
	var gotPath string
	client := &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ios, _, out, _ := iostreams.Test()
	err := listRun(&ListOptions{IO: ios, HttpClient: httpFactory(client), Org: "my-org", Number: 42})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/42/comment" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(out.String(), "hello") || !strings.Contains(out.String(), "alice") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"c1","content":"hello","like_total":2}]`
	client := &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ios, _, out, _ := iostreams.Test()
	err := listRun(&ListOptions{IO: ios, HttpClient: httpFactory(client), Org: "my-org", Number: 42, JSON: true})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(out.String(), `"hello"`) || !strings.Contains(out.String(), `"like_total": 2`) {
		t.Fatalf("output = %q, want JSON", out.String())
	}
}

func TestListRunValidation(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	for _, opts := range []ListOptions{
		{Org: "", Number: 42},
		{Org: "o", Number: 0},
		{Org: "o", Number: 42, Page: -1},
		{Org: "o", Number: 42, PerPage: 101},
		{Org: "o", Number: 42, Order: "bogus"},
	} {
		err := listRun(&opts)
		if err == nil || cmdutil.ExitCode(err) != cmdutil.ExitUsage {
			t.Fatalf("listRun(%+v) err=%v, want ExitUsage", opts, err)
		}
	}
}

func TestListRunNotFoundExit3(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	client := &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: http.StatusText(http.StatusNotFound), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":"not found"}`))}, nil
	})}
	ios, _, _, _ := iostreams.Test()
	err := listRun(&ListOptions{IO: ios, HttpClient: httpFactory(client), Org: "my-org", Number: 999})
	if err == nil {
		t.Fatal("listRun() error = nil, want not-found")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want ExitNotFound(%d)", got, cmdutil.ExitNotFound)
	}
}
