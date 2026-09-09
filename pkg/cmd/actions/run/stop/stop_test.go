package stop

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdStop(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "stop run", args: []string{"run-1"}, wantErr: false},
		{name: "stop with json", args: []string{"--json", "run-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdStop(f, func(opts *StopOptions) error {
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

func TestNewCmdStopEmptyRunID(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdStop(f, nil)
	cmd.SetArgs([]string{""})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for empty run id")
	}
}

func TestNewCmdStopJSONFlag(t *testing.T) {
	cmd := NewCmdStop(cmdutil.TestFactory(), func(opts *StopOptions) error {
		return nil
	})
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("json flag missing")
	}
}

func TestStopRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotMethod, gotPath string
	opts := &StopOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotMethod = req.Method
					gotPath = req.URL.Path
					return stopTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
	}

	if err := stopRun(opts); err != nil {
		t.Fatalf("stopRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1/stop"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("request method = %q, want POST", gotMethod)
	}
	if strings.Contains(gotPath, "access_token=") {
		t.Fatalf("request path unexpectedly contains access_token: %q", gotPath)
	}
}

func TestStopRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &StopOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return stopTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JSON:       true,
	}

	if err := stopRun(opts); err != nil {
		t.Fatalf("stopRun() error = %v", err)
	}
	for _, want := range []string{`"run_id": "run-1"`, `"owner": "owner"`, `"repo": "repo"`, `"action": "stopped"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("JSON output missing %s; output=%s", want, out.String())
		}
	}
}

func TestStopRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &StopOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return stopTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "missing",
	}

	err := stopRun(opts)
	if err == nil {
		t.Fatal("stopRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to stop pipeline run") {
		t.Fatalf("error = %q, want to wrap stop failure", err.Error())
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (404 preserved through %%w wrap)", got, cmdutil.ExitNotFound)
	}
}

func stopTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
