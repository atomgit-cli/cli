package list

import (
	"io"
	"net/http"
	"net/url"
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
		{name: "list with org", args: []string{"--org", "my-org"}, wantErr: false},
		{name: "list with keyword", args: []string{"--org", "my-org", "--keyword", "prod"}, wantErr: false},
		{name: "list with limit", args: []string{"--org", "my-org", "--limit", "10"}, wantErr: false},
		{name: "list with paginate", args: []string{"--org", "my-org", "--paginate", "--per-page", "100"}, wantErr: false},
		{name: "list with page", args: []string{"--org", "my-org", "--page", "2"}, wantErr: false},
		{name: "list with json", args: []string{"--org", "my-org", "--json"}, wantErr: false},
		{name: "list with table format", args: []string{"--org", "my-org", "--format", "table"}, wantErr: false},
		{name: "missing org", args: []string{}, wantErr: true},
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

func TestListRunBuildsV8Query(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotPath string
	opts := &ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return listTestResponse(http.StatusOK, `{"total_count":0,"runner_groups":[]}`), nil
				}),
			}, nil
		},
		Org:     "my-org",
		Keyword: "prod",
		Limit:   25,
		Page:    2,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if !strings.HasPrefix(gotPath, "/api/v8/orgs/my-org/actions/runner-groups?") {
		t.Fatalf("request path = %q, want v8 orgs runner-groups prefix", gotPath)
	}
	assertNoAccessTokenQuery(t, gotPath)

	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	for _, key := range []string{"keyword", "per_page", "page"} {
		key := key
		if _, ok := q[key]; !ok {
			t.Fatalf("query missing %s in %s", key, q.Encode())
		}
	}
	if q.Get("keyword") != "prod" {
		t.Fatalf("keyword = %q, want prod", q.Get("keyword"))
	}
	if q.Get("per_page") != "25" {
		t.Fatalf("per_page = %q, want 25", q.Get("per_page"))
	}
	if q.Get("page") != "2" {
		t.Fatalf("page = %q, want 2", q.Get("page"))
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"runner_groups":[]}`), nil
				}),
			}, nil
		},
		Org:   "my-org",
		Limit: 30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if gotPath != "/api/v8/orgs/my-org/actions/runner-groups?per_page=30" {
		t.Fatalf("request path = %q, want default per_page=30", gotPath)
	}
}

func TestListRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":1,"runner_groups":[{"id":"1","name":"prod","runner_group_name":"prod","namespace_id":"10","creator":"admin","create_time":1700000000,"runner_count":3,"namespace_type":"Organization","share_all":true}]}`), nil
				}),
			}, nil
		},
		Org:   "my-org",
		Limit: 30,
		JSON:  true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"id": "1"`) {
		t.Fatalf("JSON output = %q, missing id field", got)
	}
	if !strings.Contains(got, `"runner_group_name": "prod"`) {
		t.Fatalf("JSON output = %q, missing runner_group_name", got)
	}
	if !strings.Contains(got, `"share_all": true`) {
		t.Fatalf("JSON output = %q, missing share_all", got)
	}
}

func TestListRunEmptyResultsJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":0,"runner_groups":[]}`), nil
				}),
			}, nil
		},
		Org:   "my-org",
		Limit: 30,
		JSON:  true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "[]" {
		t.Fatalf("empty JSON output = %q, want []", got)
	}
}

func TestListRunEmptyResultsText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":0,"runner_groups":[]}`), nil
				}),
			}, nil
		},
		Org:   "my-org",
		Limit: 30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "No runner groups found" {
		t.Fatalf("empty text output = %q, want 'No runner groups found'", got)
	}
}

func TestListRunPaginateEmptyResultsEmitsEmptyArray(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":0,"runner_groups":[]}`), nil
				}),
			}, nil
		},
		Org:        "my-org",
		Limit:      30,
		Paginate:   true,
		PerPage:    100,
		PerPageSet: true,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "[]" {
		t.Fatalf("paginate empty JSON = %q, want []", got)
	}
}

func TestListRunTextOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":1,"runner_groups":[{"id":"1","name":"prod","runner_group_name":"prod-group","namespace_id":"10","creator":"admin","create_time":1700000000,"runner_count":3,"namespace_type":"Organization","share_all":true}]}`), nil
				}),
			}, nil
		},
		Org:   "my-org",
		Limit: 30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "prod-group") {
		t.Fatalf("text output = %q, missing group name", got)
	}
	if !strings.Contains(got, "runners=3") {
		t.Fatalf("text output = %q, missing runner count", got)
	}
	if !strings.Contains(got, "shared-all") {
		t.Fatalf("text output = %q, missing share status", got)
	}
}

func TestListRunMissingOrg(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Org:        "",
		Limit:      30,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error for missing --org")
	}
}

func TestListRunInvalidLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Org:        "my-org",
		Limit:      0,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error for --limit <= 0")
	}
}

func TestListRunPaginateWithPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Org:        "my-org",
		Limit:      30,
		Paginate:   true,
		Page:       1,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error for --paginate + --page")
	}
}

func TestListRunAPIError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Org:   "nonexistent",
		Limit: 30,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error for 404")
	}
}

func TestListRunPaginateMultiplePages(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	pageCount := 0
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					pageCount++
					if pageCount == 1 {
						return listTestResponse(http.StatusOK, `{"total_count":2,"runner_groups":[{"id":"1","name":"g1","runner_group_name":"g1","runner_count":1,"share_all":false}]}`), nil
					}
					return listTestResponse(http.StatusOK, `{"total_count":2,"runner_groups":[]}`), nil
				}),
			}, nil
		},
		Org:        "my-org",
		Limit:      30,
		Paginate:   true,
		PerPage:    1,
		PerPageSet: true,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if pageCount != 2 {
		t.Fatalf("page count = %d, want 2", pageCount)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"id": "1"`) {
		t.Fatalf("JSON output = %q, missing id from page 1", got)
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
		{name: "default", jsonFlag: false, raw: "", want: output.FormatSimple},
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

func TestTrimGroups(t *testing.T) {
	t.Parallel()
	opts := &ListOptions{Limit: 1, PerPage: 10, LimitSet: true, PerPageSet: true}
	tests := []struct {
		name   string
		groups []api.RunnerGroup
		opts   *ListOptions
		want   int
	}{
		{name: "nil groups returns empty", groups: nil, opts: &ListOptions{}, want: 0},
		{name: "trim to limit", groups: make([]api.RunnerGroup, 5), opts: opts, want: 1},
		{name: "no trim without perPageSet", groups: make([]api.RunnerGroup, 5), opts: &ListOptions{Limit: 1, LimitSet: true}, want: 5},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := trimGroups(tt.groups, tt.opts)
			if len(got) != tt.want {
				t.Fatalf("trimGroups() len = %d, want %d", len(got), tt.want)
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
