package rerun

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdRerun(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "rerun run", args: []string{"run-1"}, wantErr: false},
		{name: "rerun with json", args: []string{"--json", "run-1"}, wantErr: false},
		{name: "rerun with yes", args: []string{"--yes", "run-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdRerun(f, func(opts *RerunOptions) error {
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

func TestNewCmdRerunEmptyRunID(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdRerun(f, nil)
	cmd.SetArgs([]string{""})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for empty run id")
	}
}

func TestNewCmdRerunJSONFlag(t *testing.T) {
	cmd := NewCmdRerun(cmdutil.TestFactory(), func(opts *RerunOptions) error {
		return nil
	})
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("json flag missing")
	}
}

func TestNewCmdRerunYesFlag(t *testing.T) {
	cmd := NewCmdRerun(cmdutil.TestFactory(), func(opts *RerunOptions) error {
		return nil
	})
	if cmd.Flags().Lookup("yes") == nil {
		t.Fatal("yes flag missing")
	}
}

// TestRerunRunRequiresConfirmationBeforeWrite verifies the confirmation gate
// fires before any HTTP request in non-interactive mode (spec §4).
func TestRerunRunRequiresConfirmationBeforeWrite(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	requests := 0
	opts := &RerunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return rerunTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
	}

	err := rerunRun(opts)
	if err == nil {
		t.Fatal("rerunRun() without --yes in non-interactive mode = nil, want error")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %q, want mention of --yes", err.Error())
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0 before confirmation", requests)
	}
}

// TestRerunRunTTYConfirm exercises the interactive confirmation path: typing
// the expected value proceeds with the rerun request.
func TestRerunRunTTYConfirm(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	streams, _, out, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("rerun pipeline run run-1\n")
	requests := 0
	opts := &RerunOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return rerunTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
	}

	if err := rerunRun(opts); err != nil {
		t.Fatalf("rerunRun() with TTY confirmation error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("HTTP requests = %d, want 1 after confirmation", requests)
	}
	if !strings.Contains(out.String(), "Rerunning pipeline run run-1 in owner/repo") {
		t.Fatalf("human output missing rerun summary; output=%q", out.String())
	}
}

// TestRerunRunTTYConfirmMismatch verifies a wrong confirmation input aborts
// before any HTTP request.
func TestRerunRunTTYConfirmMismatch(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	streams, _, _, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("wrong input\n")
	requests := 0
	opts := &RerunOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return rerunTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
	}

	err := rerunRun(opts)
	if err == nil {
		t.Fatal("rerunRun() with mismatched confirmation = nil, want error")
	}
	if !strings.Contains(err.Error(), "did not match") {
		t.Fatalf("error = %q, want 'did not match'", err.Error())
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0 after mismatched confirmation", requests)
	}
}

func TestRerunRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotMethod, gotPath string
	opts := &RerunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotMethod = req.Method
					gotPath = req.URL.Path
					return rerunTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Yes:        true,
	}

	if err := rerunRun(opts); err != nil {
		t.Fatalf("rerunRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1/rerun"
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

func TestRerunRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &RerunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return rerunTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Yes:        true,
		JSON:       true,
	}

	if err := rerunRun(opts); err != nil {
		t.Fatalf("rerunRun() error = %v", err)
	}
	for _, want := range []string{`"run_id": "run-1"`, `"owner": "owner"`, `"repo": "repo"`, `"action": "rerun"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("JSON output missing %s; output=%s", want, out.String())
		}
	}
}

func TestRerunRunConflict(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RerunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return rerunTestResponse(http.StatusConflict, `{"message":"RE_RUN_ONLY_IN_COMPLETE_STATUS"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Yes:        true,
	}

	err := rerunRun(opts)
	if err == nil {
		t.Fatal("rerunRun() error = nil, want error for RUNNING run")
	}
	if !strings.Contains(err.Error(), "failed to rerun pipeline run") {
		t.Fatalf("error = %q, want to wrap rerun failure", err.Error())
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitConflict {
		t.Fatalf("ExitCode = %d, want %d (409 preserved through %%w wrap)", got, cmdutil.ExitConflict)
	}
}

// TestRerunRunUnauthorized verifies a 401 response preserves the ExitAuth
// exit code through the error wrap (exit-code contract matrix).
func TestRerunRunUnauthorized(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RerunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return rerunTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Yes:        true,
	}

	err := rerunRun(opts)
	if err == nil {
		t.Fatal("rerunRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode = %d, want %d (401 preserved through %%w wrap)", got, cmdutil.ExitAuth)
	}
}

// TestRerunRunInvalidRepo verifies an invalid --repo format fails in the
// ParseRepo branch before any HTTP request.
func TestRerunRunInvalidRepo(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	requests := 0
	opts := &RerunOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return rerunTestResponse(http.StatusOK, `{"success":true}`), nil
				}),
			}, nil
		},
		Repository: "invalid",
		RunID:      "run-1",
		Yes:        true,
	}

	err := rerunRun(opts)
	if err == nil {
		t.Fatal("rerunRun() with invalid repo = nil, want error")
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0 for invalid repo", requests)
	}
}

func rerunTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
