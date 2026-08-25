package list

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdListFlags(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error {
		if opts.Limit != 12 || opts.Page != 2 || opts.PerPage != 20 || !opts.JSON {
			t.Fatalf("options = %+v", opts)
		}
		return nil
	})
	cmd.SetArgs([]string{"--limit", "12", "--page", "2", "--per-page", "20", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &ListOptions{IO: f.IOStreams, Limit: 1, Page: 1, JSON: true, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.RawQuery != "page=1&per_page=1" {
				t.Fatalf("query = %q", req.URL.RawQuery)
			}
			return listResponse(http.StatusOK, "[{\"id\":7,\"title\":\"laptop\"}]"), nil
		})}, nil
	}}
	if err := listRun(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"id\": 7") {
		t.Fatalf("output = %s", out)
	}
}

func TestListRunSanitizesTextOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &ListOptions{IO: f.IOStreams, Limit: 30, Page: 1, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return listResponse(http.StatusOK, `[{"id":7,"title":"lap\u001b[31mtop","created_at":"today\nnow"}]`), nil
		})}, nil
	}}
	if err := listRun(opts); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || out.String() != "7\tlap[31mtop\ttodaynow\n" {
		t.Fatalf("output = %q", out)
	}
}

func TestListRunValidatesPagination(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	err := listRun(&ListOptions{IO: f.IOStreams, Limit: 30, Page: 1, PerPage: 101})
	if err == nil || !strings.Contains(err.Error(), "must not exceed 100") {
		t.Fatalf("error = %v", err)
	}
}

func listResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
