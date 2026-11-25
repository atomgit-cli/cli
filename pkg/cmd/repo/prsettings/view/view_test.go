package view

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

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	cmd := NewCmdView(cmdutil.TestFactory(), func(opts *ViewOptions) error { return nil })
	if cmd.Use != "view" {
		t.Fatalf("Use = %q, want view", cmd.Use)
	}
}

func TestViewRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return viewResp(http.StatusOK, `{"merge_method":"merge","approval_required_reviewers":2,"only_allow_merge_if_pipeline_succeeds":true}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		JSON:       true,
	}
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	var s map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &s); err != nil {
		t.Fatalf("invalid JSON: %v; raw: %s", err, out.String())
	}
	if s["merge_method"] != "merge" {
		t.Fatalf("merge_method = %v, want merge", s["merge_method"])
	}
}

func TestViewRunHumanReadable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return viewResp(http.StatusOK, `{"merge_method":"ff","approval_required_reviewers":3,"only_allow_merge_if_pipeline_succeeds":true,"disable_merge_by_self":true}`), nil
			})}, nil
		},
		Repository: "owner/repo",
	}
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"Pipeline required", "Disable self-merge", "Min reviewers"} {
		want := want
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; output=%s", want, got)
		}
	}
}

func TestViewRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return viewResp(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
	}
	err := viewRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to get PR settings") {
		t.Fatalf("error = %v", err)
	}
}

func viewResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
