package list

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func listTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewCmdList(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "list default", args: []string{}, wantErr: false},
		{name: "list with repo", args: []string{"-R", "owner/repo"}, wantErr: false},
		{name: "list with paginate", args: []string{"--paginate", "--per-page", "100"}, wantErr: false},
		{name: "list with json", args: []string{"--json"}, wantErr: false},
		{name: "list with format table", args: []string{"--format", "table"}, wantErr: false},
		{name: "list with limit", args: []string{"--limit", "10"}, wantErr: false},
		{name: "list with page", args: []string{"--page", "1"}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	pluginsJSON := `[{"name":"checkout","display_name":"Checkout","description":"checkout repo","version":"v1.0","custom":"preserved"}]`
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, pluginsJSON), nil
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

	var result []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON array: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1", len(result))
	}
	if result[0]["custom"] != "preserved" {
		t.Fatalf("custom field lost: %v", result[0]["custom"])
	}
}

func TestListRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	pluginsJSON := `[{"name":"checkout","display_name":"Checkout","description":"checkout repo","version":"v1.0"}]`
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, pluginsJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "checkout") {
		t.Fatalf("stdout = %q, want it to contain 'checkout'", out)
	}
	if !strings.Contains(out, "Checkout") {
		t.Fatalf("stdout = %q, want it to contain 'Checkout'", out)
	}
}

func TestListRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "No actions plugins found") {
		t.Fatalf("stdout = %q, want 'No actions plugins found'", stdout.String())
	}
}

func TestListRunEmptyJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `[]`), nil
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

	if stdout.String() != "[]\n" {
		t.Fatalf("stdout = %q, want '[]\\n'", stdout.String())
	}
}

func TestListRunV2HostAndBearer(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotHost string
	var gotAuth string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotHost = req.URL.Host
					gotAuth = req.Header.Get("Authorization")
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if gotHost != "web-api.gitcode.com" {
		t.Fatalf("host = %q, want web-api.gitcode.com", gotHost)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
}

func TestListRunValidateFlags(t *testing.T) {
	tests := []struct {
		name    string
		opts    *ListOptions
		wantErr string
	}{
		{name: "limit negative", opts: &ListOptions{Limit: -1, Repository: "o/r"}, wantErr: "--limit must be greater than or equal to 0"},
		{name: "page negative", opts: &ListOptions{Limit: 30, Page: -1, Repository: "o/r"}, wantErr: "--page must be greater than or equal to 0"},
		{name: "per-page negative", opts: &ListOptions{Limit: 30, PerPage: -1, Repository: "o/r"}, wantErr: "--per-page must be greater than or equal to 0"},
		{name: "paginate with page", opts: &ListOptions{Limit: 30, Paginate: true, Page: 1, Repository: "o/r"}, wantErr: "--paginate cannot be combined with --page"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.IO, _, _, _ = iostreams.Test()
			tt.opts.HttpClient = func() (*http.Client, error) { return &http.Client{}, nil }
			err := listRun(tt.opts)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		jsonFlag bool
		format   string
		wantErr  bool
		want     output.Format
	}{
		{name: "json flag", jsonFlag: true, format: "", want: output.FormatJSON},
		{name: "format json", jsonFlag: false, format: "json", want: output.FormatJSON},
		{name: "format table", jsonFlag: false, format: "table", want: output.FormatTable},
		{name: "json with table conflict", jsonFlag: true, format: "table", wantErr: true},
		{name: "default", jsonFlag: false, format: "", want: output.FormatSimple},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveOutputFormat(tt.jsonFlag, tt.format)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Fatalf("format = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListRunWritesToFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	pluginsJSON := `[{"name":"checkout","display_name":"Checkout","description":"checkout repo","version":"v1.0"}]`
	tmpFile := filepath.Join(t.TempDir(), "test-plugins-output.json")
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, pluginsJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      0,
		JSON:       true,
		Files:      tmpFile,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("file content is not valid JSON: %v", err)
	}
	if len(result) != 1 || result[0]["name"] != "checkout" {
		t.Fatalf("file content = %s, want 1 plugin named checkout", string(content))
	}
	os.Remove(tmpFile)
}

func TestListRunSinglePage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	pluginsJSON := `[{"name":"p1"},{"name":"p2"}]`
	callCount := 0
	var gotPage string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					parsed, _ := url.Parse(req.URL.String())
					gotPage = parsed.Query().Get("page")
					return listTestResponse(http.StatusOK, pluginsJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      0,
		Page:       2,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1 (single page)", callCount)
	}
	if gotPage != "2" {
		t.Fatalf("page param = %q, want 2", gotPage)
	}
	if !strings.Contains(stdout.String(), "p1") {
		t.Fatalf("stdout = %q, want p1", stdout.String())
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
					return listTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      0,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "failed to list actions plugins") {
		t.Fatalf("error = %v, want it to contain 'failed to list actions plugins'", err)
	}
}

func TestListRunPaginationLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	page1 := `[{"name":"p1"},{"name":"p2"}]`
	page2 := `[{"name":"p3"}]`
	callCount := 0
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					parsed, _ := url.Parse(req.URL.String())
					page := parsed.Query().Get("page")
					if page == "2" {
						return listTestResponse(http.StatusOK, page2), nil
					}
					return listTestResponse(http.StatusOK, page1), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      3,
		LimitSet:   true,
		PerPage:    2,
		PerPageSet: true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "p1") || !strings.Contains(out, "p3") {
		t.Fatalf("stdout = %q, want p1 and p3", out)
	}
}

func TestListRunContentPaginationUsesServerMetadata(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	page1 := `{"page_num":1,"page_size":2,"total":3,"page_count":2,"content":[{"name":"p1"},{"name":"p2"}]}`
	page2 := `{"page_num":2,"page_size":2,"total":3,"page_count":2,"content":[{"name":"p3"}]}`
	callCount := 0
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					parsed, _ := url.Parse(req.URL.String())
					if parsed.Query().Get("page") == "2" {
						return listTestResponse(http.StatusOK, page2), nil
					}
					return listTestResponse(http.StatusOK, page1), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2", callCount)
	}
	if len(entries) != 3 || entries[2]["name"] != "p3" {
		t.Fatalf("entries = %v, want three entries including p3", entries)
	}
}

func paginationEntries(count int) []json.RawMessage {
	entries := make([]json.RawMessage, 0, count)
	for i := 0; i < count; i++ {
		entries = append(entries, json.RawMessage(`{"name":"p"}`))
	}
	return entries
}

func TestShouldStopPagination(t *testing.T) {
	tests := []struct {
		name             string
		page             int
		response         *api.ActionsPluginsPage
		requestedPerPage int
		want             bool
	}{
		{
			name:             "server page count reached",
			page:             2,
			response:         &api.ActionsPluginsPage{PageCount: 2},
			requestedPerPage: 100,
			want:             true,
		},
		{
			name:             "server page count not reached",
			page:             1,
			response:         &api.ActionsPluginsPage{PageCount: 3},
			requestedPerPage: 100,
			want:             false,
		},
		{
			name:             "server page size wins over requested per page",
			page:             1,
			response:         &api.ActionsPluginsPage{PageSize: 50, Entries: paginationEntries(50)},
			requestedPerPage: 100,
			want:             false,
		},
		{
			name:             "server page size stops on short page",
			page:             1,
			response:         &api.ActionsPluginsPage{PageSize: 50, Entries: paginationEntries(49)},
			requestedPerPage: 100,
			want:             true,
		},
		{
			name:             "requested per page used when server omits metadata",
			page:             1,
			response:         &api.ActionsPluginsPage{Entries: paginationEntries(100)},
			requestedPerPage: 100,
			want:             false,
		},
		{
			name:             "requested per page stops on short page",
			page:             1,
			response:         &api.ActionsPluginsPage{Entries: paginationEntries(30)},
			requestedPerPage: 100,
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldStopPagination(tt.page, tt.response, tt.requestedPerPage); got != tt.want {
				t.Fatalf("shouldStopPagination() = %v, want %v", got, tt.want)
			}
		})
	}
}
