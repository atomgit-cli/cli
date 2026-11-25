package create

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdCreate(t *testing.T) {
	t.Parallel()
	cmd := NewCmdCreate(cmdutil.TestFactory(), func(opts *CreateOptions) error { return nil })
	if cmd.Use != "create" {
		t.Fatalf("Use = %q, want create", cmd.Use)
	}
}

func TestCreateRunSuccess(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	var gotMethod string
	opts := &CreateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotMethod = req.Method
				return createResp(http.StatusOK, `{}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Wildcard:   "main",
		Pusher:     "admin",
		Merger:     "admin",
	}
	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if gotMethod != "PUT" {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if !strings.Contains(errOut.String(), "Created branch protection rule for main") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestCreateRunNoWildcard(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &CreateOptions{IO: io, Repository: "owner/repo"}
	err := createRun(opts)
	if err == nil {
		t.Fatal("error = nil, want usage error for missing wildcard")
	}
}

func TestCreateRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &CreateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return createResp(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Wildcard:   "main",
		Pusher:     "admin",
	}
	err := createRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to create branch protection") {
		t.Fatalf("error = %v", err)
	}
}

func createResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
