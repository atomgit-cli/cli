package retry

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdRetry(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "retry with job", args: []string{"run-1", "--job", "job-1"}, wantErr: false},
		{name: "retry with repeated jobs", args: []string{"run-1", "--job", "job-1", "--job", "job-2"}, wantErr: false},
		{name: "retry with failed", args: []string{"run-1", "--failed"}, wantErr: false},
		{name: "retry with json", args: []string{"run-1", "--failed", "--json"}, wantErr: false},
		{name: "retry with yes", args: []string{"run-1", "--failed", "--yes"}, wantErr: false},
		{name: "no args", args: []string{"--failed"}, wantErr: true},
		{name: "neither job nor failed", args: []string{"run-1"}, wantErr: true},
		{name: "job and failed are mutually exclusive", args: []string{"run-1", "--job", "job-1", "--failed"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdRetry(f, func(opts *RetryOptions) error {
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

const retryTestJobsBody = `{"total_count":3,"jobs":[` +
	`{"id":"job-ok","name":"ok_job","status":"COMPLETED"},` +
	`{"id":"job-fail","name":"fail_job","status":"FAILED"},` +
	`{"id":"job-cancel","name":"cancel_job","status":"CANCELED"}]}`

func TestRetryRunFailedFiltersRetryableJobs(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, out, _ := iostreams.Test()
	var gotMethod, gotPath, gotBody string
	opts := &RetryOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotMethod = req.Method
					gotPath = req.URL.Path
					if strings.HasSuffix(req.URL.Path, "/retry") {
						b, _ := io.ReadAll(req.Body)
						gotBody = string(b)
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
		Yes:        true,
	}

	if err := retryRun(opts); err != nil {
		t.Fatalf("retryRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1/retry"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("request method = %q, want POST", gotMethod)
	}
	if !strings.Contains(gotBody, `"job_run_ids":["job-fail","job-cancel"]`) {
		t.Fatalf("request body = %q, want only failed/canceled job ids", gotBody)
	}
	if !strings.Contains(out.String(), "Retried 2 job(s)") {
		t.Fatalf("output = %q, want retried count", out.String())
	}
}

func TestRetryRunFailedNoMatchingJobs(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	retryCalled := false
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						retryCalled = true
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, `{"total_count":1,"jobs":[{"id":"job-ok","name":"ok_job","status":"COMPLETED"}]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error when no retryable jobs")
	}
	if !strings.Contains(err.Error(), "no failed or canceled jobs") {
		t.Fatalf("error = %q, want no-retryable-jobs message", err.Error())
	}
	if retryCalled {
		t.Fatal("retry endpoint must not be called when no jobs match")
	}
}

func TestRetryRunJobValidatesOwnership(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	retryCalled := false
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						retryCalled = true
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobIDs:     []string{"job-fail", "foreign-job"},
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error for foreign job id")
	}
	if !strings.Contains(err.Error(), "do not belong to run") {
		t.Fatalf("error = %q, want ownership error", err.Error())
	}
	if retryCalled {
		t.Fatal("retry endpoint must not be called with foreign job ids")
	}
}

func TestRetryRunJobJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobIDs:     []string{"job-fail", "job-cancel"},
		Yes:        true,
		JSON:       true,
	}

	if err := retryRun(opts); err != nil {
		t.Fatalf("retryRun() error = %v", err)
	}
	for _, want := range []string{`"run_id": "run-1"`, `"action": "retried"`, `"job_run_ids": [`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("JSON output missing %s; output=%s", want, out.String())
		}
	}
}

func TestRetryRunCompletedRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						return retryTestResponse(http.StatusBadRequest, `{"message":"仅失败状态流水线支持重试操作"}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
		Yes:        true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error for COMPLETED run")
	}
	if !strings.Contains(err.Error(), "failed to retry pipeline run jobs") {
		t.Fatalf("error = %q, want to wrap retry failure", err.Error())
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode = %d, want %d (HTTP 400 maps to usage error per cmdutil contract)", got, cmdutil.ExitUsage)
	}
}

func TestNewCmdRetryYesFlag(t *testing.T) {
	cmd := NewCmdRetry(cmdutil.TestFactory(), func(opts *RetryOptions) error {
		return nil
	})
	if cmd.Flags().Lookup("yes") == nil {
		t.Fatal("yes flag missing")
	}
}

// TestRetryRunRequiresConfirmationBeforeWrite verifies the confirmation gate
// fires before the retry POST in non-interactive mode (spec §4). The
// read-only jobs listing may happen first; the write must not.
func TestRetryRunRequiresConfirmationBeforeWrite(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	retryCalled := false
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						retryCalled = true
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() without --yes in non-interactive mode = nil, want error")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %q, want mention of --yes", err.Error())
	}
	if retryCalled {
		t.Fatal("retry endpoint must not be called before confirmation")
	}
}

// TestRetryRunTTYConfirm exercises the interactive confirmation path: typing
// the expected value proceeds with the retry request.
func TestRetryRunTTYConfirm(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	streams, _, out, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("retry pipeline run run-1\n")
	retryCalled := false
	opts := &RetryOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						retryCalled = true
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
	}

	if err := retryRun(opts); err != nil {
		t.Fatalf("retryRun() with TTY confirmation error = %v", err)
	}
	if !retryCalled {
		t.Fatal("retry endpoint must be called after confirmation")
	}
	if !strings.Contains(out.String(), "Retried 2 job(s) of pipeline run run-1 in owner/repo") {
		t.Fatalf("human output missing retry summary; output=%q", out.String())
	}
}

// TestRetryRunTTYConfirmMismatch verifies a wrong confirmation input aborts
// before the retry POST.
func TestRetryRunTTYConfirmMismatch(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	streams, _, _, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("wrong input\n")
	retryCalled := false
	opts := &RetryOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						retryCalled = true
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() with mismatched confirmation = nil, want error")
	}
	if !strings.Contains(err.Error(), "did not match") {
		t.Fatalf("error = %q, want 'did not match'", err.Error())
	}
	if retryCalled {
		t.Fatal("retry endpoint must not be called after mismatched confirmation")
	}
}

// TestRetryRunUnauthorized verifies a 401 response preserves the ExitAuth
// exit code through the error wrap (exit-code contract matrix).
func TestRetryRunUnauthorized(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						return retryTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
		Yes:        true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode = %d, want %d (401 preserved through %%w wrap)", got, cmdutil.ExitAuth)
	}
}

// TestRetryRunInvalidRepo verifies an invalid --repo format fails in the
// ParseRepo branch before any HTTP request.
func TestRetryRunInvalidRepo(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	requests := 0
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "invalid",
		RunID:      "run-1",
		Failed:     true,
		Yes:        true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() with invalid repo = nil, want error")
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0 for invalid repo", requests)
	}
}

// TestRetryRunJobCollectsAllForeignIDs verifies that all foreign job ids are
// reported in a single error instead of failing fast on the first one.
func TestRetryRunJobCollectsAllForeignIDs(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobIDs:     []string{"foreign-1", "job-fail", "foreign-2"},
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error for foreign job ids")
	}
	for _, want := range []string{"foreign-1", "foreign-2"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want to list all foreign job ids incl %s", err.Error(), want)
		}
	}
}

// TestRetryRunJobDeduplicates verifies repeated --job values collapse into a
// single retry target.
func TestRetryRunJobDeduplicates(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, _, _ := iostreams.Test()
	var gotBody string
	opts := &RetryOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						b, _ := io.ReadAll(req.Body)
						gotBody = string(b)
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobIDs:     []string{"job-fail", "job-fail", "job-cancel"},
		Yes:        true,
	}

	if err := retryRun(opts); err != nil {
		t.Fatalf("retryRun() error = %v", err)
	}
	if !strings.Contains(gotBody, `"job_run_ids":["job-fail","job-cancel"]`) {
		t.Fatalf("request body = %q, want deduplicated job ids", gotBody)
	}
}

// TestRetryRunJobListTruncated verifies the truncation defense: when the
// server reports more jobs than returned, ownership validation must refuse to
// run against partial data (no retry POST).
func TestRetryRunJobListTruncated(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	retryCalled := false
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						retryCalled = true
						return retryTestResponse(http.StatusOK, `{"success":true}`), nil
					}
					// Server claims 5 jobs but only 3 are in the page.
					return retryTestResponse(http.StatusOK, `{"total_count":5,"jobs":[`+
						`{"id":"job-ok","name":"ok_job","status":"COMPLETED"},`+
						`{"id":"job-fail","name":"fail_job","status":"FAILED"},`+
						`{"id":"job-cancel","name":"cancel_job","status":"CANCELED"}]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JobIDs:     []string{"job-fail"},
		Yes:        true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want truncation error")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("error = %q, want truncation error", err.Error())
	}
	if retryCalled {
		t.Fatal("retry endpoint must not be called when the job list is truncated")
	}
}

// TestRetryRunNotFound verifies a 404 response preserves the ExitNotFound
// exit code through the error wrap (exit-code contract matrix).
func TestRetryRunNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return retryTestResponse(http.StatusNotFound, `{"message":"run not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-missing",
		Failed:     true,
		Yes:        true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error for 404")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (404 preserved through %%w wrap)", got, cmdutil.ExitNotFound)
	}
}

// TestRetryRunConflict verifies a 409 response preserves the ExitConflict
// exit code through the error wrap (exit-code contract matrix).
func TestRetryRunConflict(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &RetryOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if strings.HasSuffix(req.URL.Path, "/retry") {
						return retryTestResponse(http.StatusConflict, `{"message":"RETRY_NOT_ALLOWED_IN_CURRENT_STATUS"}`), nil
					}
					return retryTestResponse(http.StatusOK, retryTestJobsBody), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Failed:     true,
		Yes:        true,
	}

	err := retryRun(opts)
	if err == nil {
		t.Fatal("retryRun() error = nil, want error for 409")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitConflict {
		t.Fatalf("ExitCode = %d, want %d (409 preserved through %%w wrap)", got, cmdutil.ExitConflict)
	}
}

// TestNewCmdRetryEmptyRunID verifies the usage-error exit for an empty run-id
// argument (aligned with the rerun baseline), without invoking runF.
func TestNewCmdRetryEmptyRunID(t *testing.T) {
	called := false
	cmd := NewCmdRetry(cmdutil.TestFactory(), func(opts *RetryOptions) error {
		called = true
		return nil
	})
	cmd.SetArgs([]string{"   ", "--failed"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for empty run-id")
	}
	if called {
		t.Fatal("runF must not be called for empty run-id")
	}
}

// TestNewCmdRetryEmptyJobID verifies an empty --job value fails at the RunE
// layer (usage error) before any HTTP request.
func TestNewCmdRetryEmptyJobID(t *testing.T) {
	called := false
	cmd := NewCmdRetry(cmdutil.TestFactory(), func(opts *RetryOptions) error {
		called = true
		return nil
	})
	cmd.SetArgs([]string{"run-1", "--job", ""})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for empty --job")
	}
	if called {
		t.Fatal("runF must not be called for empty --job")
	}
}

func retryTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
