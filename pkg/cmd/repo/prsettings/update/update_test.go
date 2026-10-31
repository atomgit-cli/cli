package update

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

func TestNewCmdUpdate(t *testing.T) {
	cmd := NewCmdUpdate(cmdutil.TestFactory(), func(opts *UpdateOptions) error { return nil })
	if cmd.Use != "update" {
		t.Fatalf("Use = %q, want update", cmd.Use)
	}
}

func TestUpdateRunSuccess(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	callCount := 0
	opts := &UpdateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount == 1 {
					return updateResp(http.StatusOK, `{}`), nil
				}
				return updateResp(http.StatusOK, `{"merge_method":"ff","only_allow_merge_if_pipeline_succeeds":true}`), nil
			})}, nil
		},
		Repository:  "owner/repo",
		PipelineReq: true,
		PipelineSet: true,
	}
	if err := updateRun(opts); err != nil {
		t.Fatalf("updateRun() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "Updated PR settings") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestUpdateRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	callCount := 0
	opts := &UpdateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount == 1 {
					return updateResp(http.StatusOK, `{}`), nil
				}
				return updateResp(http.StatusOK, `{"merge_method":"ff","approval_required_reviewers":2}`), nil
			})}, nil
		},
		Repository:   "owner/repo",
		Reviewers:    2,
		ReviewersSet: true,
		JSON:         true,
	}
	if err := updateRun(opts); err != nil {
		t.Fatalf("updateRun() error = %v", err)
	}
	var s map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &s); err != nil {
		t.Fatalf("invalid JSON: %v; raw: %s", err, out.String())
	}
	if s["merge_method"] != "ff" {
		t.Fatalf("merge_method = %v, want ff", s["merge_method"])
	}
}

func TestUpdateRunNoFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &UpdateOptions{
		IO:         io,
		Repository: "owner/repo",
	}
	err := updateRun(opts)
	if err == nil {
		t.Fatal("error = nil, want usage error for no fields")
	}
}

func TestUpdateRunCloseIssueOnMerge(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	streams, _, _, errOut := iostreams.Test()
	var putBody string
	callCount := 0
	opts := &UpdateOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if req.Method == "PUT" {
					b, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read PUT body: %v", err)
					}
					putBody = string(b)
					return updateResp(http.StatusOK, `{}`), nil
				}
				return updateResp(http.StatusOK, `{"merge_request_setting":{"close_issue_when_mr_merged":true},"merge_method":"merge"}`), nil
			})}, nil
		},
		Repository:    "owner/repo",
		CloseIssue:    true,
		CloseIssueSet: true,
	}
	if err := updateRun(opts); err != nil {
		t.Fatalf("updateRun() error = %v", err)
	}
	if !strings.Contains(putBody, `"close_issue_when_mr_merged":true`) {
		t.Fatalf("PUT body missing close_issue_when_mr_merged: %s", putBody)
	}
	if !strings.Contains(errOut.String(), "Updated PR settings") {
		t.Fatalf("stderr: %s", errOut.String())
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2 (PUT + re-read)", callCount)
	}
}

func TestUpdateRunCloseIssueOnMergeDisable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	streams, _, _, errOut := iostreams.Test()
	var putBody string
	opts := &UpdateOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == "PUT" {
					b, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read PUT body: %v", err)
					}
					putBody = string(b)
					return updateResp(http.StatusOK, `{}`), nil
				}
				return updateResp(http.StatusOK, `{}`), nil
			})}, nil
		},
		Repository:    "owner/repo",
		CloseIssue:    false,
		CloseIssueSet: true,
	}
	if err := updateRun(opts); err != nil {
		t.Fatalf("updateRun() error = %v", err)
	}
	if !strings.Contains(putBody, `"close_issue_when_mr_merged":false`) {
		t.Fatalf("PUT body missing explicit close_issue_when_mr_merged=false: %s", putBody)
	}
	if !strings.Contains(errOut.String(), "Updated PR settings") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestUpdateRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &UpdateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return updateResp(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
			})}, nil
		},
		Repository:  "owner/repo",
		PipelineReq: true,
		PipelineSet: true,
	}
	err := updateRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to update PR settings") {
		t.Fatalf("error = %v", err)
	}
}

func updateResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
