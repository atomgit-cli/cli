package update

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdUpdate(t *testing.T) {
	t.Parallel()
	cmd := NewCmdUpdate(cmdutil.TestFactory(), func(opts *UpdateOptions) error { return nil })
	if cmd.Use != "update <wildcard>" {
		t.Fatalf("Use = %q, want update", cmd.Use)
	}
}

func TestUpdateRunSuccess(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	opts := &UpdateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return updateResp(http.StatusOK, `{}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Wildcard:   "main",
		Pusher:     "develop",
	}
	if err := updateRun(opts); err != nil {
		t.Fatalf("updateRun() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "Updated branch protection rule for main") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestUpdateRunNoFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &UpdateOptions{IO: io, Repository: "owner/repo", Wildcard: "main"}
	err := updateRun(opts)
	if err == nil {
		t.Fatal("error = nil, want usage error for no fields")
	}
}

func TestUpdateRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &UpdateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return updateResp(http.StatusNotFound, `{"message":"not found"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Wildcard:   "main",
		Pusher:     "admin",
	}
	err := updateRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to update branch protection") {
		t.Fatalf("error = %v", err)
	}
}

func updateResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
