package list

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "list default", args: []string{}, wantErr: false},
		{name: "list by status", args: []string{"--status", "FAILED"}, wantErr: false},
		{name: "list by event", args: []string{"--event", "Push"}, wantErr: false},
		{name: "list with limit", args: []string{"--limit", "10"}, wantErr: false},
		{name: "list with branch", args: []string{"--branch", "main"}, wantErr: false},
		{name: "list with executor", args: []string{"--executor", "dev"}, wantErr: false},
		{name: "list with workflow", args: []string{"--workflow", "CI"}, wantErr: false},
		{name: "list with workflow-id", args: []string{"--workflow-id", "wf-1"}, wantErr: false},
		{name: "list with pr", args: []string{"--pr", "42"}, wantErr: false},
		{name: "list with created", args: []string{"--created", "2026-08-01"}, wantErr: false},
		{name: "list with commit", args: []string{"--commit", "abc123"}, wantErr: false},
		{name: "list with paginate", args: []string{"--paginate", "--per-page", "100"}, wantErr: false},
		{name: "list with page", args: []string{"--page", "2"}, wantErr: false},
		{name: "list with table format", args: []string{"--format", "table"}, wantErr: false},
		{name: "list with json", args: []string{"--json"}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdList(f, func(opts *ListOptions) error {
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

func TestNewCmdListStatusEnum(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error {
		return nil
	})

	flag := cmd.Flags().Lookup("status")
	if flag == nil {
		t.Fatal("status flag missing")
	}
	got := flag.Annotations[cmdutil.FlagEnumAnnotation]
	for _, want := range []string{"COMPLETED", "RUNNING", "FAILED", "CANCELED", "IGNORED", "PAUSED", "SUSPEND"} {
		want := want
		if !slices.Contains(got, want) {
			t.Fatalf("status enum = %v, missing %q", got, want)
		}
	}
}

func TestNewCmdListEventEnum(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error {
		return nil
	})

	flag := cmd.Flags().Lookup("event")
	if flag == nil {
		t.Fatal("event flag missing")
	}
	got := flag.Annotations[cmdutil.FlagEnumAnnotation]
	for _, want := range []string{"MR", "Push", "Manual"} {
		want := want
		if !slices.Contains(got, want) {
			t.Fatalf("event enum = %v, missing %q", got, want)
		}
	}
}

func TestListRunBuildsV8Query(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	var gotPath string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return listTestResponse(http.StatusOK, `{"total_count":2,"workflow_runs":[{"workflow_run_id":"r1","head_sha":"abc123"},{"workflow_run_id":"r2","head_sha":"def456"}]}`), nil
				}),
			}, nil
		},
		Repository:    "owner/repo",
		Status:        "FAILED",
		Event:         "Push",
		Branch:        "main",
		Executor:      "dev",
		WorkflowID:    "wf-1",
		WorkflowName:  "ci",
		PullRequestID: "42",
		Created:       "2026-08-01",
		Commit:        "ABC123",
		Limit:         25,
		Page:          2,
		JSON:          true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if !strings.HasPrefix(gotPath, "/api/v8/repos/owner/repo/actions/runs?") {
		t.Fatalf("request path = %q, want v8 prefix", gotPath)
	}
	assertNoAccessTokenQuery(t, gotPath)

	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	for _, key := range []string{
		"status", "event", "branch", "executor", "workflow_id", "workflow_name", "pull_request_id",
		"per_page", "page", "startTime",
	} {
		if _, ok := q[key]; !ok {
			t.Fatalf("query missing %s in %s", key, q.Encode())
		}
	}
	if q.Get("status") != "FAILED" {
		t.Fatalf("status = %q, want FAILED", q.Get("status"))
	}
	if q.Get("per_page") != "25" {
		t.Fatalf("per_page = %q, want 25", q.Get("per_page"))
	}
	if q.Get("page") != "2" {
		t.Fatalf("page = %q, want 2", q.Get("page"))
	}
	if q.Get("startTime") != "1785542400" {
		t.Fatalf("startTime = %q, want UTC start of 2026-08-01", q.Get("startTime"))
	}
	if strings.Contains(out.String(), "def456") || !strings.Contains(out.String(), "abc123") {
		t.Fatalf("commit-filtered output = %q, want only abc123", out.String())
	}
}

func TestParseCreatedDate(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "empty", value: "", want: 0},
		{name: "valid date", value: "2026-08-01", want: 1785542400},
		{name: "invalid calendar date", value: "2026-02-30", wantErr: true},
		{name: "datetime rejected", value: "2026-08-01T00:00:00Z", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreatedDate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCreatedDate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parseCreatedDate() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFilterRunsByCommit(t *testing.T) {
	runs := []api.WorkflowRun{
		{WorkflowRunID: "r1", HeadSHA: "abc123"},
		{WorkflowRunID: "r2", HeadSHA: "def456"},
	}

	got := filterRunsByCommit(runs, "ABC123")
	if len(got) != 1 || got[0].WorkflowRunID != "r1" {
		t.Fatalf("filterRunsByCommit() = %+v, want only r1", got)
	}
	if got := filterRunsByCommit(runs, "missing"); len(got) != 0 || got == nil {
		t.Fatalf("missing commit result = %#v, want non-nil empty slice", got)
	}
	if got := filterRunsByCommit(runs, "abc"); len(got) != 0 {
		t.Fatalf("abbreviated commit result = %#v, want exact SHA matching", got)
	}
}

func TestListRunInvalidCreatedDate(t *testing.T) {
	f := cmdutil.TestFactory()
	opts := &ListOptions{IO: f.IOStreams, Created: "08/01/2026"}

	err := listRun(opts)
	if err == nil || !strings.Contains(err.Error(), "invalid --created: expected YYYY-MM-DD") {
		t.Fatalf("listRun() error = %v, want invalid --created usage error", err)
	}
}

func TestListRunDefaultPerPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return listTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if gotPath != "/api/v8/repos/owner/repo/actions/runs?per_page=30" {
		t.Fatalf("request path = %q, want default per_page=30", gotPath)
	}
}

func TestListRunPaginateDefaultPerPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return listTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Paginate:   true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if got := parsed.Query().Get("per_page"); got != "100" {
		t.Fatalf("per_page = %q, want 100 (paginate default)", got)
	}
}

func TestListRunTrimsToLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":3,"workflow_runs":[{"workflow_run_id":"r1","run_number":1,"status":"FAILED"},{"workflow_run_id":"r2","run_number":2,"status":"COMPLETED"},{"workflow_run_id":"r3","run_number":3,"status":"RUNNING"}]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      2,
		LimitSet:   true,
		PerPage:    30,
		PerPageSet: true,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	var runs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &runs); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2 (trimmed to --limit)", len(runs))
	}
}

func TestListRunPaginatesUntilLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	var gotPaths []string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath := req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					gotPaths = append(gotPaths, gotPath)
					switch req.URL.Query().Get("page") {
					case "1":
						return listTestResponse(http.StatusOK, `{"total_count":4,"workflow_runs":[{"workflow_run_id":"r1","run_number":1,"status":"FAILED","head_sha":"abc"},{"workflow_run_id":"r2","run_number":2,"status":"COMPLETED","head_sha":"other"}]}`), nil
					case "2":
						return listTestResponse(http.StatusOK, `{"total_count":4,"workflow_runs":[{"workflow_run_id":"r3","run_number":3,"status":"RUNNING","head_sha":"ABC"},{"workflow_run_id":"r4","run_number":4,"status":"CANCELED","head_sha":"abc"}]}`), nil
					default:
						t.Fatalf("unexpected page %q", req.URL.Query().Get("page"))
						return nil, nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      3,
		LimitSet:   true,
		Paginate:   true,
		PerPage:    2,
		PerPageSet: true,
		Commit:     "abc",
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	for _, want := range []string{
		"/api/v8/repos/owner/repo/actions/runs?page=1&per_page=2",
		"/api/v8/repos/owner/repo/actions/runs?page=2&per_page=2",
	} {
		if !slices.Contains(gotPaths, want) {
			t.Fatalf("paths = %#v, missing %q", gotPaths, want)
		}
	}
	var runs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &runs); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(runs) != 3 {
		t.Fatalf("len(runs) = %d, want 3; output=%s", len(runs), out.String())
	}
}

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":1,"workflow_runs":[{"workflow_run_id":"run-1","workflow_id":"wf-1","workflow_name":"CI","title":"run CI","status":"COMPLETED","event":"Push","run_number":7,"head_branch":"main","head_sha":"abc","actor":{"id":"1","object_id":"u1","login":"dev","name":"Dev"},"start_time":1700000000,"end_time":1700000100,"pause_time":0}]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	var runs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &runs); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0]["status"] != "COMPLETED" {
		t.Fatalf("status = %v, want COMPLETED", runs[0]["status"])
	}
	if runs[0]["run_number"] != float64(7) {
		t.Fatalf("run_number = %v, want 7", runs[0]["run_number"])
	}
}

func TestListRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if out.String() != "No pipeline runs found\n" {
		t.Fatalf("output = %q, want empty message", out.String())
	}
}

func TestListRunEmptyJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if out.String() != "[]\n" {
		t.Fatalf("output = %q, want []\\n", out.String())
	}
}

// TestListRunPaginateEmptyResultsEmitsEmptyArray guards against #456: the
// paginate path accumulates into `var all []api.WorkflowRun`, which stays nil
// when every page is empty. Without normalization that marshals as `null`
// instead of `[]`, breaking --json consumers.
func TestListRunPaginateEmptyResultsEmitsEmptyArray(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					// Every page returns an empty result set.
					return listTestResponse(http.StatusOK, `{"total_count":0,"workflow_runs":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Paginate:   true,
		PerPage:    100,
		PerPageSet: true,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if out.String() != "[]\n" {
		t.Fatalf("paginate empty result = %q, want `[]\\n` (regression of #456: was `null`)", out.String())
	}
}

func TestListRunInvalidLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &ListOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Limit:      0,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error")
	}
}

func TestListRunPaginateWithPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &ListOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Limit:      30,
		Paginate:   true,
		Page:       2,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error for --paginate with --page")
	}
}

func TestResolveOutputFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		jsonFlag bool
		raw      string
		want     output.Format
		wantErr  bool
	}{
		{name: "default", want: output.FormatSimple},
		{name: "table", raw: "table", want: output.FormatTable},
		{name: "json flag", jsonFlag: true, want: output.FormatJSON},
		{name: "json with format json", jsonFlag: true, raw: "json", want: output.FormatJSON},
		{name: "invalid format", raw: "yaml", wantErr: true},
		{name: "json incompatible format", jsonFlag: true, raw: "table", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveOutputFormat(tt.jsonFlag, tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("resolveOutputFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func listTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func assertNoAccessTokenQuery(t *testing.T, path string) {
	t.Helper()
	if strings.Contains(path, "access_token=") {
		t.Fatalf("request path unexpectedly contains access_token query: %q", path)
	}
}
