package list

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

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
		{name: "list with name filter", args: []string{"--name", "build"}, wantErr: false},
		{name: "list with sort+direction", args: []string{"--sort", "created", "--direction", "desc"}, wantErr: false},
		{name: "list with paginate", args: []string{"--paginate", "--per-page", "100"}, wantErr: false},
		{name: "list with table format", args: []string{"--format", "table"}, wantErr: false},
		{name: "list with json", args: []string{"--json"}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdList(f, func(opts *ListOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmdListSortEnum(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error { return nil })
	flag := cmd.Flags().Lookup("sort")
	if flag == nil {
		t.Fatal("sort flag missing")
	}
	got := flag.Annotations[cmdutil.FlagEnumAnnotation]
	if !slices.Contains(got, "created") {
		t.Fatalf("sort enum = %v, missing created", got)
	}
}

func TestListRunBuildsV8Query(t *testing.T) {
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
					return listTestResponse(http.StatusOK, artifactsResponseJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Name:       "build",
		Sort:       "created",
		Direction:  "desc",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if !strings.HasPrefix(gotPath, "/api/v8/repos/owner/repo/actions/artifacts?") {
		t.Fatalf("request path = %q, want v8 prefix", gotPath)
	}
	assertNoAccessTokenQuery(t, gotPath)
	parsed, err := url.Parse(gotPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	for _, key := range []string{"name", "sort", "direction", "per_page"} {
		key := key
		if _, ok := q[key]; !ok {
			t.Fatalf("query missing %s in %s", key, q.Encode())
		}
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"artifacts":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if gotPath != "/api/v8/repos/owner/repo/actions/artifacts?per_page=30" {
		t.Fatalf("request path = %q, want default per_page=30", gotPath)
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
					return listTestResponse(http.StatusOK, artifactsResponseJSON()), nil
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
	var arts []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arts); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(arts) != 1 {
		t.Fatalf("len(arts) = %d, want 1", len(arts))
	}
	if arts[0]["name"] != "build-output" {
		t.Fatalf("name = %v, want build-output", arts[0]["name"])
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"artifacts":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if out.String() != "No artifacts found\n" {
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"artifacts":[]}`), nil
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

func TestListRunTable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, artifactsResponseJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		Format:     "table",
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"NAME", "ID", "SIZE", "CREATED", "EXPIRES", "build-output", "art-1", "1.0 MiB", "2026-"} {
		want := want
		if !strings.Contains(got, want) {
			t.Errorf("table output missing %q; output=%s", want, got)
		}
	}
}

func TestListRunError(t *testing.T) {
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
		Repository: "owner/repo",
		Limit:      30,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (404 preserved through %%w)", got, cmdutil.ExitNotFound)
	}
}

func TestListRunInvalidLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{IO: io, HttpClient: func() (*http.Client, error) { return &http.Client{}, nil }, Repository: "owner/repo", Limit: 0}
	if err := listRun(opts); err == nil {
		t.Fatal("listRun() error = nil, want usage error for --limit 0")
	}
}

func TestListRunPaginateWithPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{IO: io, HttpClient: func() (*http.Client, error) { return &http.Client{}, nil }, Repository: "owner/repo", Limit: 30, Paginate: true, Page: 2}
	if err := listRun(opts); err == nil {
		t.Fatal("listRun() error = nil, want usage error for --paginate with --page")
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
					return listTestResponse(http.StatusOK, `{"total_count":0,"artifacts":[]}`), nil
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
						return listTestResponse(http.StatusOK, `{"total_count":3,"artifacts":[{"id":"a1","name":"a"},{"id":"a2","name":"b"}]}`), nil
					case "2":
						return listTestResponse(http.StatusOK, `{"total_count":3,"artifacts":[{"id":"a3","name":"c"},{"id":"a4","name":"d"}]}`), nil
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
		JSON:       true,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	for _, want := range []string{
		"/api/v8/repos/owner/repo/actions/artifacts?page=1&per_page=2",
		"/api/v8/repos/owner/repo/actions/artifacts?page=2&per_page=2",
	} {
		if !slices.Contains(gotPaths, want) {
			t.Fatalf("paths = %#v, missing %q", gotPaths, want)
		}
	}
	var arts []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arts); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(arts) != 3 {
		t.Fatalf("len(arts) = %d, want 3; output=%s", len(arts), out.String())
	}
}

func TestListRunPaginateWithRunFlag(t *testing.T) {
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
						return listTestResponse(http.StatusOK, `{"total_count":2,"artifacts":[{"id":"a1","name":"a"},{"id":"a2","name":"b"}]}`), nil
					default:
						return listTestResponse(http.StatusOK, `{"total_count":2,"artifacts":[]}`), nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Limit:      30,
		Paginate:   true,
		PerPage:    2,
		PerPageSet: true,
		JSON:       true,
	}
	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	// Verify all requests hit the run-scoped endpoint (not repo-level).
	for _, p := range gotPaths {
		p := p
		if !strings.HasPrefix(p, "/api/v8/repos/owner/repo/actions/runs/run-1/artifacts") {
			t.Fatalf("paginate+--run path = %q, want run-scoped prefix", p)
		}
	}
	var arts []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arts); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(arts) != 2 {
		t.Fatalf("len(arts) = %d, want 2", len(arts))
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
					return listTestResponse(http.StatusOK, `{"total_count":3,"artifacts":[{"id":"a1","name":"a","size_bytes":10},{"id":"a2","name":"b","size_bytes":20},{"id":"a3","name":"c","size_bytes":30}]}`), nil
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
	var arts []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arts); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(arts) != 2 {
		t.Fatalf("len(arts) = %d, want 2 (trimmed to --limit)", len(arts))
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
		{name: "json incompatible", jsonFlag: true, raw: "table", wantErr: true},
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

func TestListRunWithRunFlag(t *testing.T) {
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
					return listTestResponse(http.StatusOK, artifactsResponseJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		Limit:      30,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	wantPrefix := "/api/v8/repos/owner/repo/actions/runs/run-1/artifacts"
	if !strings.HasPrefix(gotPath, wantPrefix) {
		t.Fatalf("request path = %q, want run-scoped prefix %q", gotPath, wantPrefix)
	}
	assertNoAccessTokenQuery(t, gotPath)
	var arts []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arts); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("len(arts) = %d, want 1", len(arts))
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

func assertNoAccessTokenQuery(t *testing.T, path string) {
	t.Helper()
	if strings.Contains(path, "access_token=") {
		t.Fatalf("request path unexpectedly contains access_token query: %q", path)
	}
}
