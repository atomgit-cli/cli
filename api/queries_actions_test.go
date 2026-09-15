package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestListActionsRunsBuildsV8Query(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
	})
	client.SetToken("test-token", "test")

	resp, err := ListActionsRuns(client, "owner", "repo", &ActionsListRunsOptions{
		Event:         "Push",
		Status:        "FAILED",
		Branch:        "main",
		Executor:      "dev",
		PullRequestID: "42",
		WorkflowID:    "wf-1",
		WorkflowName:  "ci",
		PerPage:       50,
		Page:          2,
		StartTime:     1700000000,
		EndTime:       1700001000,
	})
	if err != nil {
		t.Fatalf("ListActionsRuns() error = %v", err)
	}
	if resp.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", resp.TotalCount)
	}

	assertNoAccessTokenQuery(t, gotPath)

	wantPrefix := "/api/v8/repos/owner/repo/actions/runs?"
	if !strings.HasPrefix(gotPath, wantPrefix) {
		t.Fatalf("request path = %q, want prefix %q", gotPath, wantPrefix)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}

	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	for _, key := range []string{"event", "status", "branch", "executor", "pull_request_id", "workflow_id", "workflow_name", "per_page", "page", "startTime", "endTime"} {
		if _, ok := q[key]; !ok {
			t.Fatalf("query missing %s in %s", key, q.Encode())
		}
	}
	if q.Get("status") != "FAILED" {
		t.Fatalf("status param = %q, want FAILED", q.Get("status"))
	}
	if q.Get("per_page") != "50" {
		t.Fatalf("per_page param = %q, want 50", q.Get("per_page"))
	}
}

func TestStopActionsRunBuildsV8Path(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `{"success":true}`), nil
	})
	client.SetToken("test-token", "test")

	if err := StopActionsRun(client, "owner", "repo", "run-1"); err != nil {
		t.Fatalf("StopActionsRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1/stop"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("request method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	assertNoAccessTokenQuery(t, gotPath)
}

func TestRerunActionsRunBuildsV8Path(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `{"success":true}`), nil
	})
	client.SetToken("test-token", "test")

	if err := RerunActionsRun(client, "owner", "repo", "run-1"); err != nil {
		t.Fatalf("RerunActionsRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1/rerun"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("request method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	assertNoAccessTokenQuery(t, gotPath)
}

func TestRerunActionsRunEscapesPathParams(t *testing.T) {
	var gotEscapedPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotEscapedPath = req.URL.EscapedPath()
		return authTestResponse(http.StatusOK, `{"success":true}`), nil
	})
	client.SetToken("test-token", "test")

	if err := RerunActionsRun(client, "my owner", "repo", "run/1"); err != nil {
		t.Fatalf("RerunActionsRun() error = %v", err)
	}

	want := "/api/v8/repos/my%20owner/repo/actions/runs/run%2F1/rerun"
	if gotEscapedPath != want {
		t.Fatalf("escaped request path = %q, want %q", gotEscapedPath, want)
	}
}

func TestListActionsRunsNoOptions(t *testing.T) {
	var gotPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		return authTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
	})

	if _, err := ListActionsRuns(client, "owner", "repo", nil); err != nil {
		t.Fatalf("ListActionsRuns() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q (no query)", gotPath, want)
	}
}

func TestListActionsRunsParsesResponse(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{
			"total_count": 2,
			"workflow_runs": [
				{
					"workflow_run_id": "run-1",
					"workflow_id": "wf-1",
					"workflow_name": "CI",
					"file_path": ".gitcode/workflows/ci.yml",
					"title": "run CI",
					"status": "COMPLETED",
					"event": "Push",
					"run_number": 7,
					"head_branch": "main",
					"head_sha": "abc123",
					"actor": {"id": "1", "object_id": "u1", "login": "dev", "name": "Dev"},
					"start_time": 1700000000,
					"end_time": 1700000100,
					"pause_time": 0
				}
			]
		}`), nil
	})

	resp, err := ListActionsRuns(client, "owner", "repo", &ActionsListRunsOptions{Status: "COMPLETED"})
	if err != nil {
		t.Fatalf("ListActionsRuns() error = %v", err)
	}
	if resp.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if len(resp.WorkflowRuns) != 1 {
		t.Fatalf("len(WorkflowRuns) = %d, want 1", len(resp.WorkflowRuns))
	}
	run := resp.WorkflowRuns[0]
	if run.WorkflowRunID != "run-1" {
		t.Fatalf("WorkflowRunID = %q, want run-1", run.WorkflowRunID)
	}
	if run.RunNumber != 7 {
		t.Fatalf("RunNumber = %d, want 7", run.RunNumber)
	}
	if run.Actor == nil || run.Actor.Login != "dev" {
		t.Fatalf("Actor.Login = %v, want dev", run.Actor)
	}
	if run.StartTime != 1700000000 {
		t.Fatalf("StartTime = %d, want 1700000000", run.StartTime)
	}
}

func TestListActionsRunsEmptyRuns(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{"total_count":3,"workflow_runs":null}`), nil
	})

	resp, err := ListActionsRuns(client, "owner", "repo", nil)
	if err != nil {
		t.Fatalf("ListActionsRuns() error = %v", err)
	}
	if resp.TotalCount != 3 {
		t.Fatalf("TotalCount = %d, want 3", resp.TotalCount)
	}
	if resp.WorkflowRuns != nil {
		t.Fatalf("WorkflowRuns = %v, want nil", resp.WorkflowRuns)
	}
}

func TestListActionsRunsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusBadRequest, `{"message":"bad request"}`), nil
	})

	_, err := ListActionsRuns(client, "owner", "repo", nil)
	if err == nil {
		t.Fatal("ListActionsRuns() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError (unwrapped through %%w)", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestGetActionsRunBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, detailRunJSON()), nil
	})
	client.SetToken("test-token", "test")

	if _, _, err := GetActionsRun(client, "owner", "repo", "run-1"); err != nil {
		t.Fatalf("GetActionsRun() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	want := "/api/v8/repos/owner/repo/actions/runs/run-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestGetActionsRunParsesDetail(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, detailRunJSON()), nil
	})

	detail, raw, err := GetActionsRun(client, "owner", "repo", "run-1")
	if err != nil {
		t.Fatalf("GetActionsRun() error = %v", err)
	}
	if detail == nil {
		t.Fatal("detail = nil")
	}
	if detail.WorkflowRunID != "run-1" {
		t.Fatalf("WorkflowRunID = %q, want run-1", detail.WorkflowRunID)
	}
	if detail.RunNumber != 7 {
		t.Fatalf("RunNumber = %d, want 7", detail.RunNumber)
	}
	if detail.Status != "COMPLETED" {
		t.Fatalf("Status = %q, want COMPLETED", detail.Status)
	}
	if !detail.ExistInDefaultBranch {
		t.Fatal("ExistInDefaultBranch = false, want true")
	}
	if len(detail.Stages) != 1 {
		t.Fatalf("len(Stages) = %d, want 1", len(detail.Stages))
	}
	stage := detail.Stages[0]
	if stage.Name != "build" || stage.Status != "COMPLETED" {
		t.Fatalf("stage = %+v, want name=build status=COMPLETED", stage)
	}
	if len(stage.Jobs) != 1 {
		t.Fatalf("len(stage.Jobs) = %d, want 1", len(stage.Jobs))
	}
	job := stage.Jobs[0]
	if job.Name != "compile" || job.Status != "COMPLETED" {
		t.Fatalf("job = %+v, want name=compile status=COMPLETED", job)
	}
	if len(job.Steps) != 1 {
		t.Fatalf("len(job.Steps) = %d, want 1", len(job.Steps))
	}
	if job.Steps[0].Name != "checkout" {
		t.Fatalf("step name = %q, want checkout", job.Steps[0].Name)
	}
	if len(raw) == 0 {
		t.Fatal("raw = empty, want non-empty faithful response body")
	}
}

func TestGetActionsRunRawIsFaithful(t *testing.T) {
	body := detailRunJSON()
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, body), nil
	})

	_, raw, err := GetActionsRun(client, "owner", "repo", "run-1")
	if err != nil {
		t.Fatalf("GetActionsRun() error = %v", err)
	}
	if string(raw) != body {
		t.Fatalf("raw body not preserved verbatim (len %d vs %d)", len(raw), len(body))
	}
}

func TestGetActionsRunError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, _, err := GetActionsRun(client, "owner", "repo", "missing-run")
	if err == nil {
		t.Fatal("GetActionsRun() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func detailRunJSON() string {
	return `{
		"workflow_run_id":"run-1",
		"workflow_id":"wf-1",
		"workflow_name":"CI",
		"file_path":".gitcode/workflows/ci.yml",
		"title":"run CI",
		"status":"COMPLETED",
		"event":"Push",
		"run_number":7,
		"head_branch":"main",
		"head_sha":"abc123",
		"actor":{"id":"1","object_id":"u1","login":"dev","name":"Dev"},
		"start_time":1700000000,
		"end_time":1700000100,
		"pause_time":0,
		"exist_in_default_branch":true,
		"stages":[
			{
				"id":"stg-1","category":"ci","name":"build","identifier":"build",
				"run_always":true,"fail_fast":false,"parallel":null,"is_select":true,
				"sequence":1,"depends_on":[],"condition":null,"status":"COMPLETED",
				"start_time":1700000000,"end_time":1700000090,"pause_time":0,
				"pre":[],"post":[],
				"jobs":[
					{
						"id":"job-1","category":null,"sequence":1,"async":null,
						"name":"compile","identifier":"compile","depends_on":[],
						"condition":"","resource":"default","is_select":true,
						"timeout":null,"last_dispatch_id":"d1","execute_cost_time":80,
						"status":"COMPLETED","message":null,"start_time":1700000000,
						"end_time":1700000080,"exec_id":"e1","job_type":"normal",
						"steps":[
							{
								"id":"step-1","name":"checkout","task":"actions/checkout@v4",
								"identifier":"checkout","status":"COMPLETED","sequence":1,
								"job_run_id":"jr-1","last_dispatch_id":"d2",
								"start_time":1700000000,"end_time":1700000010,
								"runtime_attribution":null,"multi_step_editable":0,
								"official_task_version":null,"icon_url":null,"business_type":null,
								"inputs":[{"key":"ref","value":"main"}],"env":[],
								"endpoint_ids":null,"message":null,"daily_build_number":null,
								"timeout-minutes":null,"continue-on-error":null
							}
						],
						"max_parallel":null,"fail_fast":null,"from":null,"with":null,
						"secrets":null,"outputs_define":null,"concurrency":null,
						"timeout_minutes":null,"continue_on_error":null
					}
				]
			}
		]
	}`
}

func TestListActionsRunJobsBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, jobsResponseJSON()), nil
	})
	client.SetToken("test-token", "test")

	if _, err := ListActionsRunJobs(client, "owner", "repo", "run-1"); err != nil {
		t.Fatalf("ListActionsRunJobs() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	want := "/api/v8/repos/owner/repo/actions/runs/run-1/jobs"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestListActionsRunJobsParses(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, jobsResponseJSON()), nil
	})

	resp, err := ListActionsRunJobs(client, "owner", "repo", "run-1")
	if err != nil {
		t.Fatalf("ListActionsRunJobs() error = %v", err)
	}
	if resp.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if len(resp.Jobs) != 2 {
		t.Fatalf("len(Jobs) = %d, want 2", len(resp.Jobs))
	}
	j := resp.Jobs[0]
	if j.Name != "compile" || j.Status != "COMPLETED" {
		t.Fatalf("job0 = name=%q status=%q, want compile/COMPLETED", j.Name, j.Status)
	}
	if len(j.Steps) != 1 {
		t.Fatalf("len(job0.Steps) = %d, want 1", len(j.Steps))
	}
	if j.Steps[0].Name != "checkout" {
		t.Fatalf("step0 name = %q, want checkout", j.Steps[0].Name)
	}
}

func TestListActionsRunJobsEmpty(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{"total_count":0,"jobs":[]}`), nil
	})

	resp, err := ListActionsRunJobs(client, "owner", "repo", "run-1")
	if err != nil {
		t.Fatalf("ListActionsRunJobs() error = %v", err)
	}
	if resp.TotalCount != 0 || len(resp.Jobs) != 0 {
		t.Fatalf("TotalCount=%d len(Jobs)=%d, want 0/0", resp.TotalCount, len(resp.Jobs))
	}
}

func TestListActionsRunJobsNullJobs(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{"total_count":3,"jobs":null}`), nil
	})

	resp, err := ListActionsRunJobs(client, "owner", "repo", "run-1")
	if err != nil {
		t.Fatalf("ListActionsRunJobs() error = %v", err)
	}
	if resp.TotalCount != 3 {
		t.Fatalf("TotalCount = %d, want 3", resp.TotalCount)
	}
	if resp.Jobs != nil {
		t.Fatalf("Jobs = %v, want nil for null response", resp.Jobs)
	}
}

func TestListActionsRunJobsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, err := ListActionsRunJobs(client, "owner", "repo", "missing")
	if err == nil {
		t.Fatal("ListActionsRunJobs() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func jobsResponseJSON() string {
	return `{
		"total_count": 2,
		"jobs": [
			{"id":"job-1","name":"compile","identifier":"compile","status":"COMPLETED",
			 "sequence":1,"job_type":"normal","resource":"default","steps":[
				{"id":"step-1","name":"checkout","task":"actions/checkout@v4","status":"COMPLETED"}]},
			{"id":"job-2","name":"test","identifier":"test","status":"FAILED",
			 "sequence":2,"job_type":"normal","resource":"default","steps":[]}
		]
	}`
}

func TestGetActionsJobBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, jobDetailJSON()), nil
	})
	client.SetToken("test-token", "test")

	if _, _, err := GetActionsJob(client, "owner", "repo", "run-1", "job-1"); err != nil {
		t.Fatalf("GetActionsJob() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	want := "/api/v8/repos/owner/repo/actions/runs/run-1/jobs/job-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestGetActionsJobParses(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, jobDetailJSON()), nil
	})

	job, raw, err := GetActionsJob(client, "owner", "repo", "run-1", "job-1")
	if err != nil {
		t.Fatalf("GetActionsJob() error = %v", err)
	}
	if job == nil {
		t.Fatal("job = nil")
	}
	if job.ID != "job-1" || job.Name != "compile" || job.Status != "COMPLETED" {
		t.Fatalf("job = id=%q name=%q status=%q, want job-1/compile/COMPLETED", job.ID, job.Name, job.Status)
	}
	if len(job.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(job.Steps))
	}
	if job.Steps[0].Name != "checkout" {
		t.Fatalf("step0 name = %q, want checkout", job.Steps[0].Name)
	}
	if len(raw) == 0 {
		t.Fatal("raw = empty, want non-empty faithful response body")
	}
}

func TestGetActionsJobRawIsFaithful(t *testing.T) {
	body := jobDetailJSON()
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, body), nil
	})

	_, raw, err := GetActionsJob(client, "owner", "repo", "run-1", "job-1")
	if err != nil {
		t.Fatalf("GetActionsJob() error = %v", err)
	}
	if string(raw) != body {
		t.Fatalf("raw body not preserved verbatim (len %d vs %d)", len(raw), len(body))
	}
}

func TestGetActionsJobError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, _, err := GetActionsJob(client, "owner", "repo", "run-1", "missing")
	if err == nil {
		t.Fatal("GetActionsJob() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func jobDetailJSON() string {
	return `{
		"id":"job-1","name":"compile","identifier":"compile","status":"COMPLETED",
		"sequence":1,"job_type":"normal","resource":"default","condition":"",
		"is_select":true,"depends_on":[],"exec_id":"e1","last_dispatch_id":"d1",
		"execute_cost_time":80,"start_time":1700000000,"end_time":1700000080,
		"category":null,"async":null,"timeout":null,"message":null,
		"max_parallel":null,"fail_fast":null,"from":null,"with":null,"secrets":null,
		"outputs_define":null,"concurrency":null,"timeout_minutes":null,"continue_on_error":null,
		"steps":[
			{"id":"step-1","name":"checkout","task":"actions/checkout@v4","identifier":"checkout",
			 "status":"COMPLETED","sequence":1,"job_run_id":"jr-1","last_dispatch_id":"d2",
			 "start_time":1700000000,"end_time":1700000010,
			 "runtime_attribution":null,"multi_step_editable":0,"official_task_version":null,
			 "icon_url":null,"business_type":null,"inputs":[],"env":[],"endpoint_ids":null,
			 "message":null,"daily_build_number":null,"timeout-minutes":null,"continue-on-error":null}
		]
	}`
}

func TestGetActionsJobLogBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAccept string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAccept = req.Header.Get("Accept")
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, jobLogContent()), nil
	})
	client.SetToken("test-token", "test")

	if _, err := GetActionsJobLog(client, "owner", "repo", "run-1", "job-1"); err != nil {
		t.Fatalf("GetActionsJobLog() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	want := "/api/v8/repos/owner/repo/actions/runs/run-1/jobs/job-1/download_log"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotAccept != "*/*" {
		t.Fatalf("Accept = %q, want */* (raw log, not JSON)", gotAccept)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestGetActionsJobLogRaw(t *testing.T) {
	body := jobLogContent()
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, body), nil
	})

	got, err := GetActionsJobLog(client, "owner", "repo", "run-1", "job-1")
	if err != nil {
		t.Fatalf("GetActionsJobLog() error = %v", err)
	}
	if string(got) != body {
		t.Fatalf("log body not preserved verbatim (len %d vs %d)", len(got), len(body))
	}
}

func TestGetActionsJobLogError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, err := GetActionsJobLog(client, "owner", "repo", "run-1", "missing")
	if err == nil {
		t.Fatal("GetActionsJobLog() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func jobLogContent() string {
	return "2026-07-08T08:52:35Z [step] starting checkout\n" +
		"2026-07-08T08:52:39Z [step] checkout done\n" +
		"2026-07-08T08:53:51Z [job] COMPLETED\n"
}

func TestListActionsArtifactsBuildsV8Query(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, artifactsResponseJSON()), nil
	})
	client.SetToken("test-token", "test")

	resp, err := ListActionsArtifacts(client, "owner", "repo", &ActionsListArtifactsOptions{
		Name:      "build",
		Sort:      "created",
		Direction: "desc",
		PerPage:   50,
		Page:      2,
	})
	if err != nil {
		t.Fatalf("ListActionsArtifacts() error = %v", err)
	}
	if resp.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", resp.TotalCount)
	}

	assertNoAccessTokenQuery(t, gotPath)
	if !strings.HasPrefix(gotPath, "/api/v8/repos/owner/repo/actions/artifacts?") {
		t.Fatalf("request path = %q, want v8 prefix", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	for _, key := range []string{"name", "sort", "direction", "per_page", "page"} {
		if _, ok := q[key]; !ok {
			t.Fatalf("query missing %s in %s", key, q.Encode())
		}
	}
}

func TestListActionsArtifactsNoOptions(t *testing.T) {
	var gotPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		return authTestResponse(http.StatusOK, `{"total_count":0,"artifacts":[]}`), nil
	})

	if _, err := ListActionsArtifacts(client, "owner", "repo", nil); err != nil {
		t.Fatalf("ListActionsArtifacts() error = %v", err)
	}
	want := "/api/v8/repos/owner/repo/actions/artifacts"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q (no query)", gotPath, want)
	}
}

func TestListActionsArtifactsParses(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, artifactsResponseJSON()), nil
	})

	resp, err := ListActionsArtifacts(client, "owner", "repo", nil)
	if err != nil {
		t.Fatalf("ListActionsArtifacts() error = %v", err)
	}
	if len(resp.Artifacts) != 1 {
		t.Fatalf("len(Artifacts) = %d, want 1", len(resp.Artifacts))
	}
	a := resp.Artifacts[0]
	if a.ID != "art-1" || a.Name != "build-output" || a.SizeBytes != 1048576 {
		t.Fatalf("artifact = id=%q name=%q size=%d, want art-1/build-output/1048576", a.ID, a.Name, a.SizeBytes)
	}
	if a.WorkflowRunID != "run-1" || a.Digest != "sha256:abc" {
		t.Fatalf("artifact workflow_run_id=%q digest=%q, want run-1/sha256:abc", a.WorkflowRunID, a.Digest)
	}
	if a.CreatedAt != "1783500745000" {
		t.Fatalf("CreatedAt = %q, want 1783500745000", a.CreatedAt)
	}
}

func TestListActionsArtifactsEmpty(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{"total_count":0,"artifacts":null}`), nil
	})

	resp, err := ListActionsArtifacts(client, "owner", "repo", nil)
	if err != nil {
		t.Fatalf("ListActionsArtifacts() error = %v", err)
	}
	if resp.TotalCount != 0 || resp.Artifacts != nil {
		t.Fatalf("TotalCount=%d Artifacts=%v, want 0/nil", resp.TotalCount, resp.Artifacts)
	}
}

func TestListActionsArtifactsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, err := ListActionsArtifacts(client, "owner", "repo", nil)
	if err == nil {
		t.Fatal("ListActionsArtifacts() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func artifactsResponseJSON() string {
	return `{
		"total_count": 1,
		"artifacts": [
			{
				"id": "art-1",
				"name": "build-output",
				"size_bytes": 1048576,
				"workflow_id": "wf-1",
				"workflow_run_id": "run-1",
				"digest": "sha256:abc",
				"expires_at": "1783587145000",
				"created_at": "1783500745000",
				"updated_at": "1783500745000"
			}
		]
	}`
}

func TestListActionsRunArtifactsBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, artifactsResponseJSON()), nil
	})
	client.SetToken("test-token", "test")

	if _, err := ListActionsRunArtifacts(client, "owner", "repo", "run-1", &ActionsListArtifactsOptions{
		Name: "build", Sort: "created", Direction: "desc", PerPage: 50, Page: 2,
	}); err != nil {
		t.Fatalf("ListActionsRunArtifacts() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	wantPrefix := "/api/v8/repos/owner/repo/actions/runs/run-1/artifacts?"
	if !strings.HasPrefix(gotPath, wantPrefix) {
		t.Fatalf("request path = %q, want prefix %q", gotPath, wantPrefix)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestListActionsRunArtifactsParses(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, artifactsResponseJSON()), nil
	})

	resp, err := ListActionsRunArtifacts(client, "owner", "repo", "run-1", nil)
	if err != nil {
		t.Fatalf("ListActionsRunArtifacts() error = %v", err)
	}
	if resp.TotalCount != 1 || len(resp.Artifacts) != 1 {
		t.Fatalf("TotalCount=%d len(Artifacts)=%d, want 1/1", resp.TotalCount, len(resp.Artifacts))
	}
	if resp.Artifacts[0].ID != "art-1" {
		t.Fatalf("artifact id = %q, want art-1", resp.Artifacts[0].ID)
	}
}

func TestListActionsRunArtifactsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, err := ListActionsRunArtifacts(client, "owner", "repo", "missing", nil)
	if err == nil {
		t.Fatal("ListActionsRunArtifacts() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestGetActionsArtifactBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, artifactDetailJSON()), nil
	})
	client.SetToken("test-token", "test")

	if _, _, err := GetActionsArtifact(client, "owner", "repo", "art-1"); err != nil {
		t.Fatalf("GetActionsArtifact() error = %v", err)
	}
	assertNoAccessTokenQuery(t, gotPath)
	want := "/api/v8/repos/owner/repo/actions/artifacts/art-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestGetActionsArtifactParses(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, artifactDetailJSON()), nil
	})
	a, raw, err := GetActionsArtifact(client, "owner", "repo", "art-1")
	if err != nil {
		t.Fatalf("GetActionsArtifact() error = %v", err)
	}
	if a == nil {
		t.Fatal("artifact = nil")
	}
	if a.ID != "art-1" || a.Name != "build-output" || a.SizeBytes != 1048576 {
		t.Fatalf("artifact = id=%q name=%q size=%d, want art-1/build-output/1048576", a.ID, a.Name, a.SizeBytes)
	}
	if a.Digest != "sha256:abc" || a.WorkflowRunID != "run-1" {
		t.Fatalf("digest=%q workflow_run_id=%q, want sha256:abc/run-1", a.Digest, a.WorkflowRunID)
	}
	if len(raw) == 0 {
		t.Fatal("raw = empty, want non-empty")
	}
}

func TestGetActionsArtifactRawIsFaithful(t *testing.T) {
	body := artifactDetailJSON()
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, body), nil
	})
	_, raw, err := GetActionsArtifact(client, "owner", "repo", "art-1")
	if err != nil {
		t.Fatalf("GetActionsArtifact() error = %v", err)
	}
	if string(raw) != body {
		t.Fatalf("raw body not preserved verbatim (len %d vs %d)", len(raw), len(body))
	}
}

func TestGetActionsArtifactError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})
	_, _, err := GetActionsArtifact(client, "owner", "repo", "missing")
	if err == nil {
		t.Fatal("GetActionsArtifact() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func artifactDetailJSON() string {
	return `{
		"id":"art-1",
		"name":"build-output",
		"size_bytes":1048576,
		"workflow_id":"wf-1",
		"workflow_run_id":"run-1",
		"digest":"sha256:abc",
		"expires_at":"1783587145000",
		"created_at":"1783500745000",
		"updated_at":"1783500745000"
	}`
}

func TestValidateActionsWorkflowBuildsV8PathAndBody(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotBody string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		gotBody = string(body)
		return authTestResponse(http.StatusOK, validateResponseJSON()), nil
	})
	client.SetToken("test-token", "test")

	resp, raw, err := ValidateActionsWorkflow(client, "owner", "repo", []byte("name: ci\n"))
	if err != nil {
		t.Fatalf("ValidateActionsWorkflow() error = %v", err)
	}
	if resp == nil || !resp.Valid {
		t.Fatalf("resp = %+v, want valid=true", resp)
	}
	if len(resp.Diagnostics) != 1 {
		t.Fatalf("len(Diagnostics) = %d, want 1", len(resp.Diagnostics))
	}
	if resp.Diagnostics[0].Message != "bad yaml" {
		t.Fatalf("diagnostic message = %q, want bad yaml", resp.Diagnostics[0].Message)
	}

	assertNoAccessTokenQuery(t, gotPath)
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	want := "/api/v8/repos/owner/repo/actions/workflows/validate"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	var reqBody map[string]string
	if err := json.Unmarshal([]byte(gotBody), &reqBody); err != nil {
		t.Fatalf("request body is not JSON: %v; body=%q", err, gotBody)
	}
	decoded, err := base64.StdEncoding.DecodeString(reqBody["base64_content"])
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if string(decoded) != "name: ci\n" {
		t.Fatalf("decoded content = %q, want %q", string(decoded), "name: ci\n")
	}
	if string(raw) != validateResponseJSON() {
		t.Fatalf("raw body not preserved verbatim (len %d vs %d)", len(raw), len(validateResponseJSON()))
	}
}

func TestValidateActionsWorkflowError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, _, err := ValidateActionsWorkflow(client, "owner", "repo", []byte("name: ci\n"))
	if err == nil {
		t.Fatal("ValidateActionsWorkflow() error = nil, want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("apiErr.StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func validateResponseJSON() string {
	return `{
		"valid": true,
		"diagnostics": [
			{
				"range": {
					"start": {"line": 2, "column": 4},
					"end": {"line": 2, "column": 5}
				},
				"severity": "error",
				"message": "bad yaml"
			}
		]
	}`
}

func TestListActionsWorkflowsBuildsV8Path(t *testing.T) {
	var gotPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `{"total_count":0,"workflows":[]}`), nil
	})
	client.SetToken("test-token", "test")

	resp, err := ListActionsWorkflows(client, "owner", "repo", 2, 50)
	if err != nil {
		t.Fatalf("ListActionsWorkflows() error = %v", err)
	}
	if resp.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", resp.TotalCount)
	}

	assertNoAccessTokenQuery(t, gotPath)

	wantPrefix := "/api/v8/repos/owner/repo/actions/workflows?"
	if !strings.HasPrefix(gotPath, wantPrefix) {
		t.Fatalf("path = %q, want prefix %q", gotPath, wantPrefix)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}

	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	if q.Get("per_page") != "50" {
		t.Fatalf("per_page = %q, want 50", q.Get("per_page"))
	}
	if q.Get("page") != "2" {
		t.Fatalf("page = %q, want 2", q.Get("page"))
	}
}

func TestListActionsWorkflowsParsesResponse(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{
			"total_count": 2,
			"workflows": [
				{"workflow_id":"wf-1","name":"CI","file_path":".gitcode/workflows/ci.yml","state":"active"},
				{"workflow_id":"wf-2","name":"Release","file_path":".gitcode/workflows/release.yml","state":"disabled"}
			]
		}`), nil
	})
	client.SetToken("test-token", "test")

	resp, err := ListActionsWorkflows(client, "owner", "repo", 0, 20)
	if err != nil {
		t.Fatalf("ListActionsWorkflows() error = %v", err)
	}
	if resp.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if len(resp.Workflows) != 2 {
		t.Fatalf("len(Workflows) = %d, want 2", len(resp.Workflows))
	}
	if resp.Workflows[0].Name != "CI" {
		t.Fatalf("Workflows[0].Name = %q, want CI", resp.Workflows[0].Name)
	}
	if resp.Workflows[1].State != "disabled" {
		t.Fatalf("Workflows[1].State = %q, want disabled", resp.Workflows[1].State)
	}
}

func TestListActionsWorkflowsNullWorkflows(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{"total_count":0,"workflows":null}`), nil
	})
	client.SetToken("test-token", "test")

	resp, err := ListActionsWorkflows(client, "owner", "repo", 0, 20)
	if err != nil {
		t.Fatalf("ListActionsWorkflows() error = %v", err)
	}
	if resp.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", resp.TotalCount)
	}
	if len(resp.Workflows) != 0 {
		t.Fatalf("len(Workflows) = %d, want 0", len(resp.Workflows))
	}
}

func TestListActionsWorkflowsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListActionsWorkflows(client, "owner", "repo", 0, 20)
	if err == nil {
		t.Fatal("ListActionsWorkflows() error = nil, want error")
	}
}

func TestDispatchActionsWorkflowBuildsPath(t *testing.T) {
	var gotPath string
	var gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"workflow_id":"wf-1","workflow_run_id":"run-1"}`), nil
	})
	client.SetToken("test-token", "test")

	result, err := DispatchActionsWorkflow(client, "owner", "repo", "ci.yml", &DispatchWorkflowRequest{
		Ref:    "main",
		Inputs: map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatalf("DispatchActionsWorkflow() error = %v", err)
	}
	if result.WorkflowID != "wf-1" || result.WorkflowRunID != "run-1" {
		t.Fatalf("result = %+v, want {wf-1 run-1}", result)
	}

	assertNoAccessTokenQuery(t, gotPath)

	wantPath := "/api/v8/repos/owner/repo/actions/workflows/ci.yml/dispatches"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if gotMethod != "POST" {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
}

func TestDispatchActionsWorkflowError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"workflow not found"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := DispatchActionsWorkflow(client, "owner", "repo", "missing", &DispatchWorkflowRequest{Ref: "main"})
	if err == nil {
		t.Fatal("DispatchActionsWorkflow() error = nil, want error")
	}
}
