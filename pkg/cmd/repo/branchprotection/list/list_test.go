package list

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error { return nil })
	if cmd.Use != "list" {
		t.Fatalf("Use = %q, want list", cmd.Use)
	}
}

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return listResp(http.StatusOK, `[{"name":"main","push_users":"admin","merge_users":"admin"}]`), nil
			})}, nil
		},
		Repository: "owner/repo",
		JSON:       true,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	var rules []map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &rules); err != nil {
		t.Fatalf("invalid JSON: %v; raw: %s", err, out.String())
	}
	if len(rules) != 1 || rules[0]["name"] != "main" {
		t.Fatalf("unexpected: %v", rules)
	}
}

func TestListRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return listResp(http.StatusOK, `[]`), nil
			})}, nil
		},
		Repository: "owner/repo",
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No branch protection rules") {
		t.Fatalf("output should say no rules; got: %s", out.String())
	}
}

func TestListRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return listResp(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
	}
	err := listRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to list branch protection") {
		t.Fatalf("error = %v, want list failure", err)
	}
}

func listResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
