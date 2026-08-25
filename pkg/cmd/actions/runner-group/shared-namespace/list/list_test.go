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
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "list with org and id", args: []string{"rg-1", "--org", "my-org"}, wantErr: false},
		{name: "list with keyword", args: []string{"rg-1", "--org", "my-org", "--keyword", "prod"}, wantErr: false},
		{name: "list with json", args: []string{"rg-1", "--org", "my-org", "--json"}, wantErr: false},
		{name: "list with paginate", args: []string{"rg-1", "--org", "my-org", "--paginate"}, wantErr: false},
		{name: "missing org", args: []string{"rg-1"}, wantErr: true},
		{name: "missing id", args: []string{}, wantErr: true},
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"shared_namespaces":[]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Keyword:       "prod",
		Limit:         25,
		Page:          2,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	want := "/api/v8/orgs/my-org/actions/runner-groups/rg-1/shared-namespaces"
	if !strings.HasPrefix(gotPath, want+"?") {
		t.Fatalf("request path = %q, want v8 shared-namespaces prefix", gotPath)
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
}

func TestListRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `{"total_count":1,"shared_namespaces":[{"id":"sn1","runner_group_id":"rg1","from_namespace_id":"ns1","to_namespace_id":"ns2","type":"explicit","create_time":1700000000,"update_time":1700000100}]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
		JSON:          true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"id": "sn1"`) {
		t.Fatalf("JSON output = %q, missing id", got)
	}
	if !strings.Contains(got, `"from_namespace_id": "ns1"`) {
		t.Fatalf("JSON output = %q, missing from_namespace_id", got)
	}
	if !strings.Contains(got, `"type": "explicit"`) {
		t.Fatalf("JSON output = %q, missing type", got)
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"shared_namespaces":[]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
		JSON:          true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "[]" {
		t.Fatalf("empty JSON = %q, want []", got)
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"shared_namespaces":[]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "No shared namespaces found" {
		t.Fatalf("empty text = %q, want 'No shared namespaces found'", got)
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
					return listTestResponse(http.StatusOK, `{"total_count":1,"shared_namespaces":[{"id":"sn1","from_namespace_id":"ns1","to_namespace_id":"ns2","type":"explicit"}]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "ns1") {
		t.Fatalf("text output = %q, missing from_namespace_id", got)
	}
	if !strings.Contains(got, "ns2") {
		t.Fatalf("text output = %q, missing to_namespace_id", got)
	}
	if !strings.Contains(got, "explicit") {
		t.Fatalf("text output = %q, missing type", got)
	}
}

func TestListRunMissingOrg(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO:            io,
		HttpClient:    func() (*http.Client, error) { return &http.Client{}, nil },
		Org:           "",
		RunnerGroupID: "rg-1",
		Limit:         30,
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
		IO:            io,
		HttpClient:    func() (*http.Client, error) { return &http.Client{}, nil },
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         0,
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
		IO:            io,
		HttpClient:    func() (*http.Client, error) { return &http.Client{}, nil },
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
		Paginate:      true,
		Page:          1,
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
		Org:           "my-org",
		RunnerGroupID: "nonexistent",
		Limit:         30,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error for 404")
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"shared_namespaces":[]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
		Paginate:      true,
		PerPage:       100,
		PerPageSet:    true,
		JSON:          true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "[]" {
		t.Fatalf("paginate empty JSON = %q, want []", got)
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
						return listTestResponse(http.StatusOK, `{"total_count":2,"shared_namespaces":[{"id":"sn1","from_namespace_id":"ns1"}]}`), nil
					}
					return listTestResponse(http.StatusOK, `{"total_count":2,"shared_namespaces":[]}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
		Limit:         30,
		Paginate:      true,
		PerPage:       1,
		PerPageSet:    true,
		JSON:          true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if pageCount != 2 {
		t.Fatalf("page count = %d, want 2", pageCount)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"id": "sn1"`) {
		t.Fatalf("JSON output = %q, missing id from page 1", got)
	}
}

func TestTrimNamespaces(t *testing.T) {
	t.Parallel()
	opts := &ListOptions{Limit: 1, PerPage: 10, LimitSet: true, PerPageSet: true}
	tests := []struct {
		name       string
		namespaces []api.SharedNamespace
		opts       *ListOptions
		want       int
	}{
		{name: "nil returns empty", namespaces: nil, opts: &ListOptions{}, want: 0},
		{name: "trim to limit", namespaces: make([]api.SharedNamespace, 5), opts: opts, want: 1},
		{name: "no trim", namespaces: make([]api.SharedNamespace, 5), opts: &ListOptions{Limit: 1, LimitSet: true}, want: 5},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := trimNamespaces(tt.namespaces, tt.opts)
			if len(got) != tt.want {
				t.Fatalf("trimNamespaces() len = %d, want %d", len(got), tt.want)
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
