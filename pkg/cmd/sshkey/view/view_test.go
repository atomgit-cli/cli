package view

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdViewParsesID(t *testing.T) {
	t.Parallel()
	cmd := NewCmdView(cmdutil.TestFactory(), func(opts *ViewOptions) error {
		if opts.ID != 42 || !opts.JSON {
			t.Fatalf("options = %+v", opts)
		}
		return nil
	})
	cmd.SetArgs([]string{"42", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = NewCmdView(cmdutil.TestFactory(), func(*ViewOptions) error { return nil })
	cmd.SetArgs([]string{"invalid"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestViewRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &ViewOptions{IO: f.IOStreams, ID: 42, JSON: true, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v5/user/keys/42" {
				t.Fatalf("path = %q", req.URL.Path)
			}
			return viewResponse(http.StatusOK, "[{\"id\":42,\"title\":\"work\"}]"), nil
		})}, nil
	}}
	if err := viewRun(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"id\": 42") {
		t.Fatalf("output = %s", out)
	}
}

func TestViewRunSanitizesTextOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &ViewOptions{IO: f.IOStreams, ID: 42, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return viewResponse(http.StatusOK, `{"id":42,"title":"work\u001b[31m","key":"ssh-ed25519 AAAA comment\u0007","created_at":"today\nnow"}`), nil
		})}, nil
	}}
	if err := viewRun(opts); err != nil {
		t.Fatal(err)
	}
	want := "work[31m\n  ID:      42\n  Key:     ssh-ed25519 AAAA comment\n  Created: todaynow\n"
	if strings.Contains(out.String(), "\x1b") || out.String() != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}

func TestViewRunWrapsNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	opts := &ViewOptions{IO: f.IOStreams, ID: 99, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return viewResponse(http.StatusNotFound, "{\"message\":\"missing\"}"), nil
		})}, nil
	}}
	if err := viewRun(opts); err == nil || !strings.Contains(err.Error(), "SSH key 99 not found") {
		t.Fatalf("error = %v", err)
	}
}

func viewResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
