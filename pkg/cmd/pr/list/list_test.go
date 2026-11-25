package list

import (
	"encoding/json"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
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
			name:    "list closed PRs",
			args:    []string{"--state", "closed"},
			wantErr: false,
		},
		{
			name:    "list with limit",
			args:    []string{"--limit", "10"},
			wantErr: false,
		},
		{
			name:    "list with base filter",
			args:    []string{"--base", "main"},
			wantErr: false,
		},
		{
			name:    "list with sort options",
			args:    []string{"--sort", "updated", "--direction", "desc", "--page", "2"},
			wantErr: false,
		},
		{
			name:    "list with paginate",
			args:    []string{"--paginate", "--per-page", "100"},
			wantErr: false,
		},
		{
			name:    "list with commit message filter",
			args:    []string{"--commit-message", "fix login"},
			wantErr: false,
		},
		{
			name:    "list with table format",
			args:    []string{"--format", "table"},
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

func TestListRunPaginatesUntilLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	var gotPaths []string
	opts := &ListOptions{
		IO: ioStreams,
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
						return listTestResponse(http.StatusOK, `[{"number":1,"title":"one"},{"number":2,"title":"two"}]`), nil
					case "2":
						return listTestResponse(http.StatusOK, `[{"number":3,"title":"three"},{"number":4,"title":"four"}]`), nil
					default:
						t.Fatalf("unexpected page %q", req.URL.Query().Get("page"))
						return nil, nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		State:      "open",
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
		"/api/v5/repos/owner/repo/pulls?page=1&per_page=2&state=open",
		"/api/v5/repos/owner/repo/pulls?page=2&per_page=2&state=open",
	} {
		if !slices.Contains(gotPaths, want) {
			t.Fatalf("paths = %#v, missing %q", gotPaths, want)
		}
	}
	var prs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &prs); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(prs) != 3 {
		t.Fatalf("len(prs) = %d, want 3; output=%s", len(prs), out.String())
	}
}

func TestListRunJSONNormalizesNullMergedField(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `[{
						"number":13,
						"state":"merged",
						"merged":null,
						"merged_at":"2026-04-30T22:43:31+08:00"
					}]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		State:      "merged",
		Limit:      30,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &prs); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(prs) != 1 || prs[0]["state"] != "merged" || prs[0]["merged"] != true {
		t.Fatalf("merged state is contradictory: %#v", prs)
	}
}

// TestListRunPaginateEmptyResultsEmitsEmptyArray guards against #456: the
// paginate path accumulates into `var all []api.PullRequest`, which stays nil
// when every page is empty. Without normalization that marshals as `null`
// instead of `[]`, breaking --json consumers.
func TestListRunPaginateEmptyResultsEmitsEmptyArray(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					// Every page returns an empty result set.
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		State:      "open",
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
		t.Fatalf("paginate empty result = %q, want `[]` (regression of #456: was `null`)", got)
	}
}

func TestListRunFiltersByCommitMessage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	var gotPaths []string
	opts := &ListOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPaths = append(gotPaths, req.URL.Path)
					switch req.URL.Path {
					case "/api/v5/repos/owner/repo/pulls":
						return listTestResponse(http.StatusOK, `[{"number":1,"title":"one"},{"number":2,"title":"two"}]`), nil
					case "/api/v5/repos/owner/repo/pulls/1/commits":
						return listTestResponse(http.StatusOK, `[{"sha":"one","message":"docs only"}]`), nil
					case "/api/v5/repos/owner/repo/pulls/2/commits":
						return listTestResponse(http.StatusOK, `[{"sha":"two","commit":{"message":"fix login flow"}}]`), nil
					default:
						t.Fatalf("unexpected path %s", req.URL.Path)
						return nil, nil
					}
				}),
			}, nil
		},
		Repository:    "owner/repo",
		State:         "open",
		Limit:         30,
		JSON:          true,
		CommitMessage: "LOGIN",
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if len(gotPaths) != 3 {
		t.Fatalf("request paths = %#v", gotPaths)
	}
	var prs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &prs); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(prs) != 1 || prs[0]["number"] != float64(2) {
		t.Fatalf("unexpected filtered output: %#v", prs)
	}
}

func TestNewCmdListStateEnumIncludesMerged(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error {
		return nil
	})

	flag := cmd.Flags().Lookup("state")
	if flag == nil {
		t.Fatal("state flag missing")
	}
	got := flag.Annotations[cmdutil.FlagEnumAnnotation]
	for _, want := range []string{"open", "closed", "merged", "all"} {
		want := want
		if !slices.Contains(got, want) {
			t.Fatalf("state enum = %v, missing %q", got, want)
		}
	}
}

func TestListRunBuildsFullQuery(t *testing.T) {
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
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		State:      "open",
		Head:       "feature/login",
		Base:       "main",
		Sort:       "updated",
		Direction:  "asc",
		Limit:      25,
		Page:       2,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	want := "/api/v5/repos/owner/repo/pulls?base=main&direction=asc&head=feature%2Flogin&page=2&per_page=25&sort=updated&state=open"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
}

func TestListRunUsesBaseRepoAndConfiguredHost(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GC_HOST", "enterprise.example.com")
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	if _, err := cfg.Authentication().Login("enterprise.example.com", "tester", "stored-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	var gotHost string
	var gotPath string
	opts := &ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotHost = req.URL.Host
					gotPath = req.URL.Path
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		BaseRepo: func() (string, error) {
			return "owner/repo", nil
		},
		State: "open",
		Limit: 30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	if gotHost != "api.enterprise.example.com" {
		t.Fatalf("request host = %q, want api.enterprise.example.com", gotHost)
	}
	if gotPath != "/api/v5/repos/owner/repo/pulls" {
		t.Fatalf("request path = %q, want /api/v5/repos/owner/repo/pulls", gotPath)
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
