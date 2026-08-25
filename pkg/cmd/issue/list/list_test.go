package list

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "list default",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "list with state",
			args:    []string{"--state", "closed"},
			wantErr: false,
		},
		{
			name:    "list with limit",
			args:    []string{"--limit", "10"},
			wantErr: false,
		},
		{
			name:    "list with labels",
			args:    []string{"--label", "bug"},
			wantErr: false,
		},
		{
			name:    "list with milestone",
			args:    []string{"--milestone", "v1.0"},
			wantErr: false,
		},
		{
			name:    "list with assignee",
			args:    []string{"--assignee", "username"},
			wantErr: false,
		},
		{
			name:    "list with creator",
			args:    []string{"--creator", "username"},
			wantErr: false,
		},
		{
			name:    "list with sort",
			args:    []string{"--sort", "updated"},
			wantErr: false,
		},
		{
			name:    "list with direction",
			args:    []string{"--direction", "asc"},
			wantErr: false,
		},
		{
			name:    "list with search",
			args:    []string{"--search", "bug"},
			wantErr: false,
		},
		{
			name:    "list with created-after",
			args:    []string{"--created-after", "2024-01-01"},
			wantErr: false,
		},
		{
			name:    "list with updated-after",
			args:    []string{"--updated-after", "2024-01-01"},
			wantErr: false,
		},
		{
			name:    "list with combined filters",
			args:    []string{"--state", "open", "--sort", "updated", "--direction", "desc"},
			wantErr: false,
		},
		{
			name:    "list with json compatibility",
			args:    []string{"--json"},
			wantErr: false,
		},
		{
			name:    "list with format flag",
			args:    []string{"--format", "table"},
			wantErr: false,
		},
		{
			name:    "list with time format flag",
			args:    []string{"--time-format", "relative"},
			wantErr: false,
		},
		{
			name:    "list with template flag",
			args:    []string{"--template", "{{.Title}}"},
			wantErr: false,
		},
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

func TestResolveOutputOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		opts    *ListOptions
		wantErr bool
	}{
		{
			name: "json compatibility",
			opts: &ListOptions{JSON: true},
		},
		{
			name: "format json",
			opts: &ListOptions{Format: "json"},
		},
		{
			name: "time format relative",
			opts: &ListOptions{TimeFormat: "relative"},
		},
		{
			name:    "invalid format",
			opts:    &ListOptions{Format: "yaml"},
			wantErr: true,
		},
		{
			name:    "invalid time format",
			opts:    &ListOptions{TimeFormat: "iso"},
			wantErr: true,
		},
		{
			name:    "json with incompatible format",
			opts:    &ListOptions{JSON: true, Format: "table"},
			wantErr: true,
		},
		{
			name:    "json with template",
			opts:    &ListOptions{JSON: true, Template: "{{.Title}}"},
			wantErr: true,
		},
		{
			name:    "template with format",
			opts:    &ListOptions{Format: "simple", Template: "{{.Title}}"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveOutputOptions(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveOutputOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeIssueListTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		endOfDay bool
		want     string
		wantErr  bool
	}{
		{
			name:     "date only start of day",
			input:    "2026-03-31",
			endOfDay: false,
			want:     "2026-03-31T00:00:00Z",
		},
		{
			name:     "date only end of day",
			input:    "2026-03-31",
			endOfDay: true,
			want:     "2026-03-31T23:59:59Z",
		},
		{
			name:     "rfc3339",
			input:    "2026-03-31T12:30:00+08:00",
			endOfDay: true,
			want:     "2026-03-31T12:30:00+08:00",
		},
		{
			name:     "invalid",
			input:    "2026/03/31",
			endOfDay: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeIssueListTime(tt.input, tt.endOfDay)
			if tt.wantErr {
				if err == nil {
					t.Fatal("normalizeIssueListTime() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeIssueListTime() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeIssueListTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListRunRejectsOutputUsageErrorsBeforeHTTP(t *testing.T) {
	t.Parallel()
	httpCalled := false
	opts := &ListOptions{
		IO: cmdutil.TestFactory().IOStreams,
		HttpClient: func() (*http.Client, error) {
			httpCalled = true
			return &http.Client{}, nil
		},
		Format: "yaml",
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want usage error")
	}
	if httpCalled {
		t.Fatal("listRun() called HttpClient before validating output flags")
	}
}

func TestListRunAllowsTemplateOutputForEmptyResults(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, stdout, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`[]`)),
				}, nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Template:   "{{len .}} issues",
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if got := stdout.String(); got != "0 issues" {
		t.Fatalf("stdout = %q, want %q", got, "0 issues")
	}
}

func TestListRunUsesMilestoneTitleWithoutExtraRequest(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var requests int
	ioStreams, _, stdout, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.Path != "/api/v5/repos/owner/repo/issues" {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				if got := req.URL.Query().Get("milestone"); got != "v1.0" {
					t.Fatalf("milestone query = %q, want %q", got, "v1.0")
				}
				return jsonResponse(http.StatusOK, `[]`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Milestone:  "v1.0",
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("stdout = %q, want empty JSON array", stdout.String())
	}
}

func TestListRunResolvesMilestoneNumberToTitle(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var requests []string
	ioStreams, _, stdout, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.URL.Path)
				switch req.URL.Path {
				case "/api/v5/repos/owner/repo/milestones/123":
					return jsonResponse(http.StatusOK, `{"number":123,"title":"v1.0"}`), nil
				case "/api/v5/repos/owner/repo/issues":
					if got := req.URL.Query().Get("milestone"); got != "v1.0" {
						t.Fatalf("milestone query = %q, want resolved title v1.0", got)
					}
					return jsonResponse(http.StatusOK, `[]`), nil
				default:
					t.Fatalf("unexpected path: %s", req.URL.Path)
					return nil, nil
				}
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Milestone:  "000123",
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	wantRequests := []string{
		"/api/v5/repos/owner/repo/milestones/123",
		"/api/v5/repos/owner/repo/issues",
	}
	if strings.Join(requests, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("stdout = %q, want empty JSON array", stdout.String())
	}
}

func TestListRunReturnsNotFoundForUnknownMilestoneNumber(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/api/v5/repos/owner/repo/milestones/999" {
					t.Fatalf("unexpected path: %s", req.URL.Path)
				}
				return jsonResponse(http.StatusNotFound, `{"message":"not found"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Milestone:  "999",
		JSON:       true,
	}

	err := listRun(opts)
	if err == nil || !strings.Contains(err.Error(), "milestone #999 not found") {
		t.Fatalf("listRun() error = %v, want milestone not found error", err)
	}
	var cliErr *cmdutil.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("listRun() error type = %T, want *cmdutil.CLIError", err)
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitNotFound)
	}
}

func TestResolveMilestoneFilterNumericBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "empty", value: "", want: ""},
		{name: "title", value: "v1.0", want: "v1.0"},
		{name: "plus sign is title", value: "+1", want: "+1"},
		{name: "minus sign is title", value: "-1", want: "-1"},
		{name: "zero is invalid number", value: "0", wantErr: true},
		{name: "32-bit overflow is invalid", value: "2147483648", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveMilestoneFilter(nil, "owner", "repo", tt.value)
			if tt.wantErr {
				if err == nil || cmdutil.ExitCode(err) != cmdutil.ExitUsage {
					t.Fatalf("resolveMilestoneFilter() error = %v, want usage error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveMilestoneFilter() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveMilestoneFilter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListRunPreservesNonNotFoundMilestoneError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{"message":"temporary failure"}`), nil
			})}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Milestone:  "123",
		JSON:       true,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want server error")
	}
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf("listRun() error = %v, must preserve non-404 failure", err)
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitError)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestResolvePerPage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		perPageSet bool
		perPage    int
		limit      int
		want       int
	}{
		{name: "per-page set wins", perPageSet: true, perPage: 50, limit: 30, want: 50},
		{name: "per-page unset falls back to limit", perPageSet: false, perPage: 0, limit: 30, want: 30},
		{name: "per-page set but zero falls back to limit", perPageSet: true, perPage: 0, limit: 20, want: 20},
		{name: "per-page set negative falls back to limit", perPageSet: true, perPage: -5, limit: 15, want: 15},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := &ListOptions{Limit: tt.limit, PerPage: tt.perPage, PerPageSet: tt.perPageSet}
			if got := resolvePerPage(opts); got != tt.want {
				t.Fatalf("resolvePerPage() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTrimIssues(t *testing.T) {
	t.Parallel()
	mk := func(n int) []api.Issue { return make([]api.Issue, n) }
	tests := []struct {
		name       string
		issues     []api.Issue
		limit      int
		limitSet   bool
		perPageSet bool
		wantLen    int
	}{
		{name: "both set truncates to limit", issues: mk(10), limit: 3, limitSet: true, perPageSet: true, wantLen: 3},
		{name: "only per-page set no truncation", issues: mk(10), limit: 3, limitSet: false, perPageSet: true, wantLen: 10},
		{name: "only limit set no truncation", issues: mk(10), limit: 3, limitSet: true, perPageSet: false, wantLen: 10},
		{name: "neither set no truncation", issues: mk(5), limit: 3, limitSet: false, perPageSet: false, wantLen: 5},
		{name: "under limit no truncation", issues: mk(2), limit: 5, limitSet: true, perPageSet: true, wantLen: 2},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := &ListOptions{Limit: tt.limit, LimitSet: tt.limitSet, PerPageSet: tt.perPageSet}
			got := trimIssues(tt.issues, opts)
			if len(got) != tt.wantLen {
				t.Fatalf("trimIssues() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestListRunPerPageValidation(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		PerPage:    -1,
	}
	err := listRun(opts)
	if err == nil || cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("listRun(PerPage=-1) err=%v, want ExitUsage", err)
	}
}
