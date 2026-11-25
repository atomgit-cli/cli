package run

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

func TestNewCmdRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "with workflow id", args: []string{"wf-1", "--ref", "main"}, wantErr: false},
		{name: "with filename", args: []string{"ci.yml", "--ref", "main"}, wantErr: false},
		{name: "with fields", args: []string{"ci.yml", "--ref", "main", "-f", "k=v"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdRun(cmdutil.TestFactory(), func(opts *RunOptions) error {
				return nil
			})
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunRunSuccess(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	var gotMethod string
	var gotPath string
	opts := &RunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotMethod = req.Method
				gotPath = req.URL.Path
				return runResp(http.StatusOK, `{"workflow_id":"wf-1","workflow_run_id":"run-123"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		WorkflowID: "wf-1",
		Ref:        "main",
	}

	if err := runRun(opts); err != nil {
		t.Fatalf("runRun() error = %v", err)
	}
	if gotMethod != "POST" {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	wantPath := "/api/v8/repos/owner/repo/actions/workflows/wf-1/dispatches"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if !strings.Contains(errOut.String(), "Triggered workflow") {
		t.Fatalf("stderr: %s", errOut.String())
	}
}

func TestRunRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &RunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return runResp(http.StatusOK, `{"workflow_id":"wf-1","workflow_run_id":"run-456"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		WorkflowID: "ci.yml",
		Ref:        "main",
		JSON:       true,
	}

	if err := runRun(opts); err != nil {
		t.Fatalf("runRun() error = %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("invalid JSON: %v; raw: %s", err, out.String())
	}
	if result["workflow_run_id"] != "run-456" {
		t.Fatalf("workflow_run_id = %v, want run-456", result["workflow_run_id"])
	}
}

func TestRunRunNoRef(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &RunOptions{
		IO:         io,
		Repository: "owner/repo",
		WorkflowID: "wf-1",
	}

	err := runRun(opts)
	if err == nil {
		t.Fatal("error = nil, want usage error for missing --ref")
	}
}

func TestRunRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &RunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return runResp(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		WorkflowID: "wf-1",
		Ref:        "main",
	}

	err := runRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to trigger workflow") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRunSecretScan(t *testing.T) {
	t.Setenv("GC_TOKEN", "leaked-secret-xyz")
	f := cmdutil.TestFactory()
	cmd := NewCmdRun(f, nil)
	cmd.SetArgs([]string{"wf-1", "--ref", "main", "-f", "token=leaked-secret-xyz", "-R", "owner/repo"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error for secret in field value")
	}
}

func runResp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
