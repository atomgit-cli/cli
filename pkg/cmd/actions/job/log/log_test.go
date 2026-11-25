package log

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdLog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "log run+job", args: []string{"run-1", "job-1"}, wantErr: false},
		{name: "log with output", args: []string{"-o", "out.log", "run-1", "job-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "one arg", args: []string{"run-1"}, wantErr: true},
		{name: "too many args", args: []string{"a", "b", "c"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdLog(f, func(opts *LogOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmdLogOutputFlag(t *testing.T) {
	t.Parallel()
	cmd := NewCmdLog(cmdutil.TestFactory(), func(opts *LogOptions) error { return nil })
	if cmd.Flags().Lookup("output") == nil {
		t.Fatal("output flag missing")
	}
}

func TestLogRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &LogOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return logTestResponse(http.StatusOK, jobLogContent()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobID:      "job-1",
	}

	if err := logRun(opts); err != nil {
		t.Fatalf("logRun() error = %v", err)
	}
	want := "/api/v8/repos/owner/repo/actions/runs/run-1/jobs/job-1/download_log"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if strings.Contains(gotPath, "access_token=") {
		t.Fatalf("path unexpectedly contains access_token: %q", gotPath)
	}
}

func TestLogRunStdout(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	body := jobLogContent()
	opts := &LogOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return logTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobID:      "job-1",
	}

	if err := logRun(opts); err != nil {
		t.Fatalf("logRun() error = %v", err)
	}
	if out.String() != body {
		t.Fatalf("stdout = %q, want raw log verbatim", out.String())
	}
}

func TestLogRunOutputFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	body := jobLogContent()
	outFile := filepath.Join(t.TempDir(), "job.log")
	opts := &LogOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return logTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobID:      "job-1",
		Output:     outFile,
	}

	if err := logRun(opts); err != nil {
		t.Fatalf("logRun() error = %v", err)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(got) != body {
		t.Fatalf("file content = %q, want raw log verbatim", string(got))
	}
}

func TestLogRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &LogOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return logTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobID:      "missing",
	}

	err := logRun(opts)
	if err == nil {
		t.Fatal("logRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (404 preserved through %%w)", got, cmdutil.ExitNotFound)
	}
}

func TestLogRunRefusesTTYWithoutOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.TestTTY() // IsStdoutTTY() == true
	apiCalled := false
	opts := &LogOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					apiCalled = true
					return logTestResponse(http.StatusOK, jobLogContent()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobID:      "job-1",
		// no --output
	}

	err := logRun(opts)
	if err == nil {
		t.Fatal("logRun() error = nil, want usage error on TTY without --output")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode = %d, want %d (usage: use --output/redirect for binary on TTY)", got, cmdutil.ExitUsage)
	}
	if apiCalled {
		t.Fatal("API was called; the TTY guard should refuse before downloading")
	}
}

func logTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func jobLogContent() string {
	return "2026-07-08T08:52:35Z [step] starting checkout\n" +
		"2026-07-08T08:52:39Z [step] checkout done\n" +
		"2026-07-08T08:53:51Z [job] COMPLETED\n"
}
