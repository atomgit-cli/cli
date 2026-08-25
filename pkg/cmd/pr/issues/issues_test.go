package issues

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "list PR issues",
			args:    []string{"123"},
			wantErr: false,
		},
		{
			name:    "list PR issues with repo",
			args:    []string{"123", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "list PR issues with json",
			args:    []string{"123", "--json"},
			wantErr: false,
		},
		{
			name:    "no PR number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "zero PR number",
			args:    []string{"0"},
			wantErr: true,
		},
		{
			name:    "negative PR number",
			args:    []string{"-1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdIssues(f, func(opts *IssuesOptions) error {
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

func TestIssuesRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/1/issues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","number":"42","title":"Fix login bug","state":"open","html_url":"https://gitcode.com/owner/repo/issues/42"}]`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if len(result) != 1 || result[0]["number"] != "42" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestIssuesRunJSONEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/1/issues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	if out.String() != "[]\n" {
		t.Fatalf("expected empty JSON array, got: %q", out.String())
	}
}

func TestIssuesRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/1/issues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No linked issues") {
		t.Fatalf("expected empty message, got: %q", out.String())
	}
}

func TestIssuesRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/1/issues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","number":"42","title":"Fix login bug","state":"open","html_url":"https://gitcode.com/owner/repo/issues/42"}]`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "#42") || !strings.Contains(output, "Fix login bug") {
		t.Fatalf("expected issue info in output, got: %q", output)
	}
}

func TestIssuesRunTextClosedState(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/1/issues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","number":"42","title":"Fixed bug","state":"closed"},{"id":"2","number":"43","title":"Merged feature","state":"merged"}]`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "#42") || !strings.Contains(output, "Fixed bug") {
		t.Fatalf("expected closed issue in output, got: %q", output)
	}
	if !strings.Contains(output, "#43") || !strings.Contains(output, "Merged feature") {
		t.Fatalf("expected merged issue in output, got: %q", output)
	}
}

func TestIssuesRunNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     999,
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent PR")
	}
	if !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "owner/repo") {
		t.Fatalf("error should reference PR number and repo, got: %v", err)
	}
}

func TestIssuesRunEmptyFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/1/issues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","number":"","title":"","state":""}]`))
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "#?") || !strings.Contains(output, "(no title)") || !strings.Contains(output, "unknown") {
		t.Fatalf("expected fallback for empty fields, got: %q", output)
	}
}

func TestIssuesRunAuthError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestIssuesRunHttpClientError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return nil, fmt.Errorf("client error") },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error when HttpClient fails")
	}
}

func TestIssuesRunBaseRepoError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "", fmt.Errorf("no remote") },
		Number:     1,
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error when BaseRepo fails")
	}
}

func TestIssuesRunParseRepoError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	err := issuesRun(&IssuesOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "invalid", nil },
		Number:     1,
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
}
