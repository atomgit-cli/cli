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
				return listResp(http.StatusOK, `{"total_count":1,"workflows":[{"workflow_id":"wf-1","name":"CI","file_path":".gitcode/workflows/ci.yml","state":"active"}]}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		JSON:       true,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	var workflows []map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &workflows); err != nil {
		t.Fatalf("invalid JSON: %v; raw: %s", err, out.String())
	}
	if len(workflows) != 1 || workflows[0]["name"] != "CI" {
		t.Fatalf("unexpected: %v", workflows)
	}
}

func TestListRunHumanReadable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return listResp(http.StatusOK, `{"total_count":2,"workflows":[{"workflow_id":"wf-1","name":"CI","file_path":".gitcode/workflows/ci.yml","state":"active"},{"workflow_id":"wf-2","name":"Release","file_path":".gitcode/workflows/release.yml","state":"disabled"}]}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"CI", "Release", "ci.yml"} {
		want := want
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; output=%s", want, got)
		}
	}
}

func TestListRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return listResp(http.StatusOK, `{"total_count":0,"workflows":[]}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No workflows found") {
		t.Fatalf("output should say no workflows; got: %s", out.String())
	}
}

func TestListRunEmptyJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return listResp(http.StatusOK, `{"total_count":0,"workflows":[]}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		JSON:       true,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	var result []interface{}
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output should be valid JSON array; got: %s", out.String())
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
		Limit:      30,
	}
	err := listRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to list workflows") {
		t.Fatalf("error = %v", err)
	}
}

func listResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
