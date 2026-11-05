package watch

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdWatch(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "watch run", args: []string{"run-1"}, wantErr: false},
		{name: "watch with json", args: []string{"--json", "run-1"}, wantErr: false},
		{name: "watch with interval", args: []string{"--interval", "5", "run-1"}, wantErr: false},
		{name: "watch compact", args: []string{"--compact", "run-1"}, wantErr: false},
		{name: "watch exit-status", args: []string{"--exit-status", "run-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdWatch(f, func(opts *WatchOptions) error {
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

func TestNewCmdWatchEmptyRunID(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdWatch(f, nil)
	cmd.SetArgs([]string{""})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for empty run id")
	}
}

func TestNewCmdWatchInvalidInterval(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdWatch(f, nil)
	cmd.SetArgs([]string{"--interval", "0", "run-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for interval < 1")
	}
}

func TestNewCmdWatchFlagsExist(t *testing.T) {
	cmd := NewCmdWatch(cmdutil.TestFactory(), func(opts *WatchOptions) error {
		return nil
	})
	for _, flag := range []string{"repo", "interval", "compact", "exit-status", "json"} {
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("flag %q missing", flag)
		}
	}
}

func TestWatchRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					return watchTestResponse(http.StatusOK, completedRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
	}

	if err := watchRun(opts); err != nil {
		t.Fatalf("watchRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
}

func TestWatchRunNonTTYTerminalSnapshot(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return watchTestResponse(http.StatusOK, completedRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
	}

	if err := watchRun(opts); err != nil {
		t.Fatalf("watchRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"CI", "COMPLETED", "run id", "build", "compile"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; output=%s", want, got)
		}
	}
}

func TestWatchRunNonTTYRunningExitsImmediately(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	callCount := 0
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					return watchTestResponse(http.StatusOK, runningRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
	}

	if err := watchRun(opts); err != nil {
		t.Fatalf("watchRun() error = %v", err)
	}
	if callCount != 1 {
		t.Fatalf("API call count = %d, want 1 (single snapshot in non-TTY for non-terminal state)", callCount)
	}
	if !strings.Contains(out.String(), "RUNNING") {
		t.Errorf("output should contain RUNNING status; got: %s", out.String())
	}
}

func TestWatchRunExitStatusOnFailed(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return watchTestResponse(http.StatusOK, failedRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
		ExitStatus: true,
	}

	err := watchRun(opts)
	if err == nil {
		t.Fatal("watchRun() error = nil, want error for failed run with --exit-status")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitError)
	}
}

func TestWatchRunExitStatusOnCompleted(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return watchTestResponse(http.StatusOK, completedRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
		ExitStatus: true,
	}

	if err := watchRun(opts); err != nil {
		t.Fatalf("watchRun() error = %v, want nil for completed run with --exit-status", err)
	}
}

func TestWatchRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	body := completedRunJSON()
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return watchTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
		JSON:       true,
	}

	if err := watchRun(opts); err != nil {
		t.Fatalf("watchRun() error = %v", err)
	}
	if out.String() != body+"\n" {
		t.Fatalf("JSON output not faithful: got len %d, want len %d", len(out.String()), len(body)+1)
	}
}

func TestWatchRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return watchTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "missing",
		Interval:   3,
	}

	err := watchRun(opts)
	if err == nil {
		t.Fatal("watchRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to get pipeline run") {
		t.Fatalf("error = %q, want to wrap pipeline run failure", err.Error())
	}
}

func TestTerminalStatuses(t *testing.T) {
	for _, status := range []string{"COMPLETED", "FAILED", "CANCELED", "IGNORED", "PAUSED", "SUSPEND"} {
		if !terminalStatuses[status] {
			t.Errorf("terminalStatuses[%q] = false, want true", status)
		}
	}
	for _, status := range []string{"RUNNING", "", "PENDING"} {
		if terminalStatuses[status] {
			t.Errorf("terminalStatuses[%q] = true, want false", status)
		}
	}
}

func TestFailedStatuses(t *testing.T) {
	for _, status := range []string{"FAILED", "CANCELED"} {
		if !failedStatuses[status] {
			t.Errorf("failedStatuses[%q] = false, want true", status)
		}
	}
	for _, status := range []string{"COMPLETED", "RUNNING", "PAUSED"} {
		if failedStatuses[status] {
			t.Errorf("failedStatuses[%q] = true, want false", status)
		}
	}
}

func TestHasFailedJobs(t *testing.T) {
	stage := apiStageWithJobs("build", "COMPLETED", []apiJobSpec{
		{name: "compile", status: "COMPLETED"},
		{name: "test", status: "FAILED"},
	})
	if !hasFailedJobs(stage) {
		t.Error("hasFailedJobs = false, want true when a job is FAILED")
	}

	stageOK := apiStageWithJobs("build", "COMPLETED", []apiJobSpec{
		{name: "compile", status: "COMPLETED"},
		{name: "test", status: "COMPLETED"},
	})
	if hasFailedJobs(stageOK) {
		t.Error("hasFailedJobs = true, want false when all jobs completed")
	}
}

func TestHasFailedSteps(t *testing.T) {
	job := apiWorkflowRunJob("compile", "FAILED", []string{"FAILED", "COMPLETED"})
	if !hasFailedSteps(job) {
		t.Error("hasFailedSteps = false, want true when a step is FAILED")
	}

	jobOK := apiWorkflowRunJob("compile", "COMPLETED", []string{"COMPLETED", "COMPLETED"})
	if hasFailedSteps(jobOK) {
		t.Error("hasFailedSteps = true, want false when all steps completed")
	}
}

func watchTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type apiJobSpec struct {
	name   string
	status string
}

func apiStageWithJobs(name, status string, jobs []apiJobSpec) api.WorkflowRunStage {
	stage := api.WorkflowRunStage{Name: name, Status: status}
	for _, j := range jobs {
		stage.Jobs = append(stage.Jobs, api.WorkflowRunJob{Name: j.name, Status: j.status})
	}
	return stage
}

func apiWorkflowRunJob(name, status string, stepStatuses []string) api.WorkflowRunJob {
	job := api.WorkflowRunJob{Name: name, Status: status}
	for _, s := range stepStatuses {
		job.Steps = append(job.Steps, api.WorkflowRunStep{Name: "step", Status: s})
	}
	return job
}

func completedRunJSON() string {
	return `{
		"workflow_run_id":"run-1","workflow_id":"wf-1","workflow_name":"CI",
		"file_path":".gitcode/workflows/ci.yml","title":"run CI","status":"COMPLETED",
		"event":"Push","run_number":7,"head_branch":"main","head_sha":"abc123",
		"actor":{"id":"1","object_id":"u1","login":"dev","name":"Dev"},
		"start_time":1700000000,"end_time":1700000100,"pause_time":0,
		"exist_in_default_branch":true,
		"stages":[{"id":"stg-1","category":"ci","name":"build","identifier":"build",
			"status":"COMPLETED","jobs":[{"id":"job-1","name":"compile","identifier":"compile",
			"status":"COMPLETED","steps":[{"id":"step-1","name":"checkout","status":"COMPLETED"}]}]}]
	}`
}

func runningRunJSON() string {
	return `{
		"workflow_run_id":"run-2","workflow_id":"wf-1","workflow_name":"CI",
		"file_path":".gitcode/workflows/ci.yml","title":"run CI","status":"RUNNING",
		"event":"Push","run_number":8,"head_branch":"main","head_sha":"def456",
		"actor":{"id":"1","object_id":"u1","login":"dev","name":"Dev"},
		"start_time":1700000000,"end_time":0,"pause_time":0,
		"exist_in_default_branch":true,
		"stages":[{"id":"stg-1","category":"ci","name":"build","identifier":"build",
			"status":"RUNNING","jobs":[{"id":"job-1","name":"compile","identifier":"compile",
			"status":"RUNNING","steps":[{"id":"step-1","name":"checkout","status":"COMPLETED"}]}]}]
	}`
}

func failedRunJSON() string {
	return `{
		"workflow_run_id":"run-3","workflow_id":"wf-1","workflow_name":"CI",
		"file_path":".gitcode/workflows/ci.yml","title":"run CI","status":"FAILED",
		"event":"Push","run_number":9,"head_branch":"main","head_sha":"ghi789",
		"actor":{"id":"1","object_id":"u1","login":"dev","name":"Dev"},
		"start_time":1700000000,"end_time":1700000200,"pause_time":0,
		"exist_in_default_branch":true,
		"stages":[{"id":"stg-1","category":"ci","name":"build","identifier":"build",
			"status":"FAILED","jobs":[{"id":"job-1","name":"compile","identifier":"compile",
			"status":"FAILED","steps":[{"id":"step-1","name":"checkout","status":"COMPLETED"},
			{"id":"step-2","name":"build","status":"FAILED"}]}]}]
	}`
}

func TestWatchRunNonTTYWaitPollsUntilTerminal(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	callCount := 0
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount < 3 {
						return watchTestResponse(http.StatusOK, runningRunJSON()), nil
					}
					return watchTestResponse(http.StatusOK, completedRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
		Wait:       true,
	}

	if err := watchRun(opts); err != nil {
		t.Fatalf("watchRun() error = %v", err)
	}
	if callCount != 3 {
		t.Fatalf("API call count = %d, want 3 (non-TTY --wait polls until terminal)", callCount)
	}
	got := out.String()
	if !strings.Contains(got, "RUNNING") {
		t.Errorf("output should contain the RUNNING interim snapshot; got: %s", got)
	}
	if !strings.Contains(got, "COMPLETED") {
		t.Errorf("output should contain the COMPLETED terminal snapshot; got: %s", got)
	}
}

func TestWatchRunNonTTYWaitExitStatusOnFailed(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	callCount := 0
	opts := &WatchOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount < 2 {
						return watchTestResponse(http.StatusOK, runningRunJSON()), nil
					}
					return watchTestResponse(http.StatusOK, failedRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Interval:   3,
		Wait:       true,
		ExitStatus: true,
	}

	err := watchRun(opts)
	if err == nil {
		t.Fatal("watchRun() error = nil, want error for failed run with --wait --exit-status")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitError)
	}
	if callCount != 2 {
		t.Fatalf("API call count = %d, want 2", callCount)
	}
}
