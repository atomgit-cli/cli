package branch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdBranchView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view branch",
			args:    []string{"main"},
			wantErr: false,
		},
		{
			name:    "view branch with repo",
			args:    []string{"main", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "view branch with json",
			args:    []string{"main", "--json"},
			wantErr: false,
		},
		{
			name:    "no branch name",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "whitespace branch name",
			args:    []string{"   "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdView(f, func(opts *ViewOptions) error {
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

func TestViewRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/branches/main" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"main","protected":false,"commit":{"id":"abc123def456","short_id":"abc123de","title":"Initial commit","author":{"id":"1","login":"dev"}}}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "main",
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if result["name"] != "main" {
		t.Fatalf("unexpected name: %#v", result["name"])
	}
	commit, ok := result["commit"].(map[string]interface{})
	if !ok || commit["id"] != "abc123def456" {
		t.Fatalf("unexpected commit: %#v", result["commit"])
	}
}

func TestViewRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/branches/main" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"main","protected":true,"commit":{"id":"abc123def456","short_id":"abc123de","title":"Initial commit","author":{"id":"1","login":"dev"}}}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "main",
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	output := out.String()
	for _, want := range []string{"main", "Protected: yes", "abc123def456", "Initial commit", "dev"} {
		want := want
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestViewRunTextUnprotected(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/branches/develop" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"develop","protected":false,"commit":{"id":"def789","short_id":"def789","title":"Add feature","author":{"id":"1","login":"dev"}}}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "develop",
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "develop") {
		t.Fatalf("output missing branch name: %s", output)
	}
	if strings.Contains(output, "Protected:") {
		t.Fatalf("output should not contain Protected for unprotected branch: %s", output)
	}
}

func TestViewRunNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Branch Not Found"}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "nonexistent",
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
	if !strings.Contains(err.Error(), "nonexistent") || !strings.Contains(err.Error(), "owner/repo") {
		t.Fatalf("error should reference branch name and repo, got: %v", err)
	}
}

func TestViewRunNoCommit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/branches/orphan" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"orphan","protected":false}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "orphan",
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "orphan") {
		t.Fatalf("output missing branch name: %s", output)
	}
	if strings.Contains(output, "Commit:") {
		t.Fatalf("output should not contain Commit when nil: %s", output)
	}
}

func TestViewRunCommitPartialFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/branches/feature" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"feature","protected":false,"commit":{"id":"abc123","short_id":"","title":"","author":null}}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "feature",
		JSON:       false,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "abc123") {
		t.Fatalf("expected commit ID in output, got: %q", output)
	}
	if strings.Contains(output, "Short ID:") || strings.Contains(output, "Title:") || strings.Contains(output, "Author:") {
		t.Fatalf("should not show empty Short ID/Title/Author, got: %q", output)
	}
}

func TestViewRunAuthError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "main",
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestViewRunHttpClientError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return nil, fmt.Errorf("client error") },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Branch:     "main",
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error when HttpClient fails")
	}
}

func TestViewRunBaseRepoError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "", fmt.Errorf("no remote") },
		Branch:     "main",
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error when BaseRepo fails")
	}
}

func TestViewRunParseRepoError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "invalid", nil },
		Branch:     "main",
		JSON:       false,
	})
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
}
