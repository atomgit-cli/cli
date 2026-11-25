package replies

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

func TestNewCmdReplies(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdReplies(f, func(opts *RepliesOptions) error { return nil })
	cmd.SetArgs([]string{"42", "c1", "--org", "my-org"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}
}

func TestRepliesRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"r1","content":"a reply","author":{"login":"bob"},"created_at":"2026-07-13T10:00:00Z"}]`
	var gotPath string
	client := &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ios, _, out, _ := iostreams.Test()
	err := repliesRun(&RepliesOptions{IO: ios, HttpClient: httpFactory(client), Org: "my-org", Number: 42, CommentID: "c1"})
	if err != nil {
		t.Fatalf("repliesRun() error = %v", err)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/42/comment/c1/reply" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(out.String(), "a reply") || !strings.Contains(out.String(), "bob") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRepliesRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"r1","content":"a reply"}]`
	client := &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ios, _, out, _ := iostreams.Test()
	err := repliesRun(&RepliesOptions{IO: ios, HttpClient: httpFactory(client), Org: "my-org", Number: 42, CommentID: "c1", JSON: true})
	if err != nil {
		t.Fatalf("repliesRun() error = %v", err)
	}
	if !strings.Contains(out.String(), `"a reply"`) {
		t.Fatalf("output = %q, want JSON", out.String())
	}
}

func TestRepliesRunValidation(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	for _, opts := range []RepliesOptions{
		{Org: "", Number: 42, CommentID: "c1"},
		{Org: "o", Number: 0, CommentID: "c1"},
		{Org: "o", Number: 42, CommentID: ""},
		{Org: "o", Number: 42, CommentID: "c1", Page: -1},
	} {
		err := repliesRun(&opts)
		if err == nil || cmdutil.ExitCode(err) != cmdutil.ExitUsage {
			t.Fatalf("repliesRun(%+v) err=%v, want ExitUsage", opts, err)
		}
	}
}
