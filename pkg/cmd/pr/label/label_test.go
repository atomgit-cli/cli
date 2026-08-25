package label

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "add labels",
			args:    []string{"123", "--add", "bug,enhancement"},
			wantErr: false,
		},
		{
			name:    "remove label",
			args:    []string{"123", "--remove", "bug"},
			wantErr: false,
		},
		{
			name:    "list labels",
			args:    []string{"123", "--list"},
			wantErr: false,
		},
		{
			name:    "no issue number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid issue number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "no action specified",
			args:    []string{"123"},
			wantErr: false, // Command runs, error in run function
		},
		{
			name:    "add with repo",
			args:    []string{"123", "--add", "bug", "-R", "owner/repo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdLabel(f, func(opts *LabelOptions) error {
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

// labelRoundTrip captures the request URL path so tests can assert the command
// targets the PR (merge_requests/pulls) endpoints and never the issue endpoints.
func labelRoundTrip(t *testing.T, gotPath *string, status int, body string) *http.Client {
	t.Helper()
	return &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			*gotPath = req.URL.Path
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}

func httpFactory(client *http.Client) func() (*http.Client, error) {
	return func() (*http.Client, error) { return client, nil }
}

// assertNotIssueEndpoint fails the test if the captured request path targets
// the issue labels endpoints, which is the root cause of issue #475.
func assertNotIssueEndpoint(t *testing.T, path string) {
	t.Helper()
	if strings.Contains(path, "/issues/") {
		t.Fatalf("request hit issue endpoint %q; pr label must target merge_requests/pulls, not issues (issue #475)", path)
	}
}

func TestLabelRunListUsesPREndpoint(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	prBody := `{"number":123,"title":"fix bug","labels":[{"name":"bug"},{"name":"risk/low"}]}`
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(labelRoundTrip(t, &gotPath, http.StatusOK, prBody)),
		Repository: "owner/repo",
		Number:     123,
		List:       true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if !strings.Contains(gotPath, "/pulls/123") {
		t.Fatalf("request path = %q, want to contain /pulls/123 (PR endpoint)", gotPath)
	}
	assertNotIssueEndpoint(t, gotPath)
	if !strings.Contains(out.String(), "Labels on PR #123") {
		t.Fatalf("output = %q, want 'Labels on PR #123'", out.String())
	}
	if !strings.Contains(out.String(), "bug") || !strings.Contains(out.String(), "risk/low") {
		t.Fatalf("output = %q, want both label names", out.String())
	}
}

func TestLabelRunListEmptyPR(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	prBody := `{"number":123,"title":"fix bug","labels":[]}`
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(labelRoundTrip(t, &gotPath, http.StatusOK, prBody)),
		Repository: "owner/repo",
		Number:     123,
		List:       true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if !strings.Contains(gotPath, "/pulls/123") {
		t.Fatalf("request path = %q, want /pulls/123", gotPath)
	}
	assertNotIssueEndpoint(t, gotPath)
	if !strings.Contains(out.String(), "No labels on PR #123") {
		t.Fatalf("output = %q, want 'No labels on PR #123'", out.String())
	}
}

func TestLabelRunListJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	prBody := `{"number":123,"labels":[{"name":"bug"},{"name":"enhancement"}]}`
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(labelRoundTrip(t, &gotPath, http.StatusOK, prBody)),
		Repository: "owner/repo",
		Number:     123,
		List:       true,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if !strings.Contains(gotPath, "/pulls/123") {
		t.Fatalf("request path = %q, want /pulls/123", gotPath)
	}
	assertNotIssueEndpoint(t, gotPath)
	if !strings.Contains(out.String(), `"action": "list"`) || !strings.Contains(out.String(), `"bug"`) {
		t.Fatalf("output = %q, want JSON list result", out.String())
	}
}

func TestLabelRunAddUsesPREndpoint(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	var gotMethod string
	var gotBody string
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			gotMethod = req.Method
			body, _ := io.ReadAll(req.Body)
			gotBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[{"name":"bug"},{"name":"enhancement"}]`)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Add:        []string{"bug,enhancement"},
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "/pulls/123/labels") {
		t.Fatalf("request path = %q, want to contain /pulls/123/labels", gotPath)
	}
	assertNotIssueEndpoint(t, gotPath)
	// Body must be the bare JSON array of label names per the official OpenAPI,
	// not the legacy {"labels":[...]} object (regression guard for issue #475).
	if gotBody != `["bug","enhancement"]` {
		t.Fatalf("request body = %q, want bare JSON array [\"bug\",\"enhancement\"]", gotBody)
	}
	if strings.Contains(gotBody, "labels") {
		t.Fatalf("request body %q must not wrap labels in an object", gotBody)
	}
	if !strings.Contains(out.String(), "Added labels to PR #123") {
		t.Fatalf("output = %q, want 'Added labels to PR #123'", out.String())
	}
}

// An --add value that normalizes to no valid label names must return a usage
// error without any remote write (issue #498 P2: integration path for empty
// normalized result).
func TestLabelRunAddEmptyAfterNormalizeDoesNotMutate(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	for _, add := range []string{"", ",", " , , "} {
		add := add
		client := &http.Client{
			Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("no remote request expected for add %q; got %s %s", add, req.Method, req.URL.Path)
				return nil, nil
			}),
		}
		ios, _, _, _ := iostreams.Test()
		err := labelRun(&LabelOptions{
			IO:         ios,
			HttpClient: httpFactory(client),
			Repository: "owner/repo",
			Number:     123,
			Add:        []string{add},
		})
		if err == nil {
			t.Fatalf("add=%q: expected usage error, got nil", add)
		}
		if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
			t.Fatalf("add=%q: ExitCode = %d, want ExitUsage(%d)", add, got, cmdutil.ExitUsage)
		}
	}
}

func TestLabelRunAddJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[{"name":"bug"},{"name":"enhancement"}]`)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Add:        []string{"bug"},
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if !strings.Contains(gotPath, "/pulls/123/labels") {
		t.Fatalf("request path = %q, want /pulls/123/labels", gotPath)
	}
	assertNotIssueEndpoint(t, gotPath)
	if !strings.Contains(out.String(), `"action": "add"`) || !strings.Contains(out.String(), `"bug"`) {
		t.Fatalf("output = %q, want JSON add result", out.String())
	}
}

func TestLabelRunRemoveUsesPREndpoint(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	var gotEscaped string
	var gotMethod string
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			gotEscaped = req.URL.EscapedPath()
			gotMethod = req.Method
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "status/draft",
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if !strings.Contains(gotPath, "/pulls/123/labels/status/draft") {
		t.Fatalf("request path = %q, want to contain /pulls/123/labels/status/draft", gotPath)
	}
	// EscapedPath must encode the slash in the label name so the label is a
	// single path segment (status%2Fdraft), not two segments (status/draft),
	// which would target a different resource. Regression guard for
	// url.PathEscape in RemoveLabelFromPR.
	if !strings.Contains(gotEscaped, "/pulls/123/labels/status%2Fdraft") {
		t.Fatalf("escaped path = %q, want to contain /pulls/123/labels/status%%2Fdraft (label slash must be encoded)", gotEscaped)
	}
	assertNotIssueEndpoint(t, gotPath)
	if !strings.Contains(out.String(), "Removed label 'status/draft' from PR #123") {
		t.Fatalf("output = %q, want removal confirmation", out.String())
	}
}

func TestLabelRunRemoveJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "bug",
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if !strings.Contains(gotPath, "/pulls/123/labels/bug") {
		t.Fatalf("request path = %q, want /pulls/123/labels/bug", gotPath)
	}
	assertNotIssueEndpoint(t, gotPath)
	if !strings.Contains(out.String(), `"action": "remove"`) || !strings.Contains(out.String(), `"bug"`) {
		t.Fatalf("output = %q, want JSON remove result", out.String())
	}
}
