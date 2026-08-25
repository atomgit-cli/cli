package delete

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

func TestNewCmdDeleteParsesID(t *testing.T) {
	t.Parallel()
	cmd := NewCmdDelete(cmdutil.TestFactory(), func(opts *DeleteOptions) error {
		if opts.ID != 42 || !opts.Yes || !opts.JSON {
			t.Fatalf("options = %+v", opts)
		}
		return nil
	})
	cmd.SetArgs([]string{"42", "--yes", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRunRequiresConfirmation(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	err := deleteRun(&DeleteOptions{IO: f.IOStreams, ID: 42})
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeleteRunDryRunDoesNotCallAPI(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	err := deleteRun(&DeleteOptions{IO: f.IOStreams, ID: 42, DryRun: true, HttpClient: func() (*http.Client, error) {
		t.Fatal("HttpClient must not be called during dry-run")
		return nil, nil
	}})
	if err != nil || !strings.Contains(out.String(), "Would delete SSH key 42") {
		t.Fatalf("output = %q, error = %v", out, err)
	}
}

func TestDeleteRunDryRunJSONDoesNotCallAPI(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	err := deleteRun(&DeleteOptions{IO: f.IOStreams, ID: 42, DryRun: true, JSON: true, HttpClient: func() (*http.Client, error) {
		t.Fatal("HttpClient must not be called during dry-run")
		return nil, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	var result deleteResult
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output = %q", err, out)
	}
	if result.ID != 42 || result.Action != "would-delete" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDeleteRunAcceptsInteractiveConfirmation(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	streams, in, out, _ := iostreams.TestTTY()
	in.WriteString("42\n")
	opts := &DeleteOptions{IO: streams, ID: 42, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodDelete || req.URL.Path != "/api/v5/user/keys/42" {
				t.Fatalf("request = %s %s", req.Method, req.URL.Path)
			}
			return deleteResponse(http.StatusNoContent, ""), nil
		})}, nil
	}}
	if err := deleteRun(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Deleted SSH key 42") {
		t.Fatalf("output = %q", out)
	}
}

func TestDeleteRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &DeleteOptions{IO: f.IOStreams, ID: 42, Yes: true, JSON: true, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodDelete || req.URL.Path != "/api/v5/user/keys/42" {
				t.Fatalf("request = %s %s", req.Method, req.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
		})}, nil
	}}
	if err := deleteRun(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"action\": \"deleted\"") {
		t.Fatalf("output = %s", out)
	}
}

func TestDeleteRunWrapsNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	err := deleteRun(&DeleteOptions{IO: f.IOStreams, ID: 99, Yes: true, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return deleteResponse(http.StatusNotFound, "{\"message\":\"missing\"}"), nil
		})}, nil
	}})
	if err == nil || !strings.Contains(err.Error(), "SSH key 99 not found") {
		t.Fatalf("error = %v", err)
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("exit code = %d, want %d", got, cmdutil.ExitNotFound)
	}
}

func TestDeleteRunPropagatesServerError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	f := cmdutil.TestFactory()
	err := deleteRun(&DeleteOptions{IO: f.IOStreams, ID: 42, Yes: true, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return deleteResponse(http.StatusInternalServerError, "{\"message\":\"unavailable\"}"), nil
		})}, nil
	}})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func deleteResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
