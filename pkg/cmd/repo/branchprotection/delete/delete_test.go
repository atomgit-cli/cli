package delete

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdDelete(t *testing.T) {
	t.Parallel()
	cmd := NewCmdDelete(cmdutil.TestFactory(), func(opts *DeleteOptions) error { return nil })
	if cmd.Use != "delete <wildcard>" {
		t.Fatalf("Use = %q, want delete", cmd.Use)
	}
}

func TestDeleteRunSuccess(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	var gotMethod string
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotMethod = req.Method
				return deleteResp(http.StatusNoContent, ``), nil
			})}, nil
		},
		Repository: "owner/repo",
		Wildcard:   "main",
		Yes:        true,
	}
	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	if gotMethod != "DELETE" {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if !strings.Contains(errOut.String(), "Deleted branch protection rule for main") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestDeleteRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return deleteResp(http.StatusNotFound, `{"message":"not found"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Wildcard:   "main",
		Yes:        true,
	}
	err := deleteRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to delete branch protection") {
		t.Fatalf("error = %v", err)
	}
}

func deleteResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
