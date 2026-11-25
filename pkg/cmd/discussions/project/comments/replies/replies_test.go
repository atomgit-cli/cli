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
	cmd.SetArgs([]string{"42", "c1", "-R", "owner/repo"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("Execute() error = %v", err)
	}
}

func TestRepliesRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	body := `[{"id":"r1","content":"a reply","author":{"login":"bob"}}]`
	var gotPath string
	client := &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	ios, _, out, _ := iostreams.Test()
	err := repliesRun(&RepliesOptions{IO: ios, HttpClient: httpFactory(client), Repository: "owner/repo", Number: 42, CommentID: "c1"})
	if err != nil {
		t.Fatalf("repliesRun() error = %v", err)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/42/comment/c1/reply" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(out.String(), "a reply") {
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
	err := repliesRun(&RepliesOptions{IO: ios, HttpClient: httpFactory(client), Repository: "owner/repo", Number: 42, CommentID: "c1", JSON: true})
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
		{Repository: "o/repo", Number: 0, CommentID: "c1"},
		{Repository: "o/repo", Number: 42, CommentID: ""},
		{Repository: "o/repo", Number: 42, CommentID: "c1", Page: -1},
	} {
		err := repliesRun(&opts)
		if err == nil || cmdutil.ExitCode(err) != cmdutil.ExitUsage {
			t.Fatalf("repliesRun(%+v) err=%v, want ExitUsage", opts, err)
		}
	}
}
