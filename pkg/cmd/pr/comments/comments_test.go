// Package comments_test tests the pr comments command
package comments

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdComments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid PR number",
			args:    []string{"123", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "with limit",
			args:    []string{"123", "-R", "owner/repo", "--limit", "5"},
			wantErr: false,
		},
		{
			name:    "with json",
			args:    []string{"123", "-R", "owner/repo", "--json"},
			wantErr: false,
		},
		{
			name:    "missing PR number",
			args:    []string{"-R", "owner/repo"},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc", "-R", "owner/repo"},
			wantErr: true,
		},
		{
			name:    "missing repo",
			args:    []string{"123"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: io,
				HttpClient: func() (*http.Client, error) {
					return &http.Client{}, nil
				},
			}

			cmd := NewCmdComments(f, nil)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				// Note: This will fail because we don't have a real API
				// but we're testing that the command parses args correctly
				if err != nil {
					// Check that error is not "unknown flag"
					if contains(err.Error(), "unknown flag") {
						t.Errorf("unexpected error: %v", err)
					}
					if contains(err.Error(), "invalid PR number") {
						t.Errorf("unexpected error: %v", err)
					}
				}
			}
		})
	}
}

func TestCommentsFlags(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{}, nil
		},
	}

	cmd := NewCmdComments(f, nil)

	// Check that flags are registered
	limitFlag := cmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Error("--limit flag should be registered")
	} else if limitFlag.Shorthand != "L" {
		t.Errorf("--limit shorthand should be 'L', got '%s'", limitFlag.Shorthand)
	}

	repoFlag := cmd.Flags().Lookup("repo")
	if repoFlag == nil {
		t.Error("--repo flag should be registered")
	} else if repoFlag.Shorthand != "R" {
		t.Errorf("--repo shorthand should be 'R', got '%s'", repoFlag.Shorthand)
	}

	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("--json flag should be registered")
	}
}

func TestFormatID(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string ID",
			input:    "abc123",
			expected: "abc123",
		},
		{
			name:     "int ID",
			input:    12345,
			expected: "12345",
		},
		{
			name:     "int64 ID",
			input:    int64(123456789),
			expected: "123456789",
		},
		{
			name:     "float64 ID",
			input:    float64(123.0),
			expected: "123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmdutil.FormatAPIID(tt.input)
			if result != tt.expected {
				t.Errorf("FormatAPIID(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid repo",
			repo:      "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:    "empty repo requires explicit repo",
			repo:    "",
			wantErr: true,
		},
		{
			name:    "invalid format - no slash",
			repo:    "owner",
			wantErr: true,
		},
		{
			name:    "invalid format - too many slashes",
			repo:    "owner/repo/extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRepo(tt.repo)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if owner != tt.wantOwner {
					t.Errorf("owner = %s, want %s", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("repo = %s, want %s", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestCommentsOptions(t *testing.T) {
	opts := &CommentsOptions{
		Number: 123,
		Limit:  5,
		JSON:   true,
	}

	if opts.Number != 123 {
		t.Errorf("Number = %d, want 123", opts.Number)
	}
	if opts.Limit != 5 {
		t.Errorf("Limit = %d, want 5", opts.Limit)
	}
	if !opts.JSON {
		t.Error("JSON = false, want true")
	}
}

func TestCommentsRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/123/comments" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"body":"first"},{"id":2,"body":"second"}]`))
	}))

	err := commentsRun(&CommentsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
		Limit:      1,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("commentsRun() error = %v", err)
	}

	var comments []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &comments); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if len(comments) != 1 || comments[0]["body"] != "second" {
		t.Fatalf("unexpected JSON output: %#v", comments)
	}
}

func TestCommentsRunJSONEmptyOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))

	err := commentsRun(&CommentsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("commentsRun() error = %v", err)
	}
	if got := out.String(); got != "[]\n" {
		t.Fatalf("output = %q, want JSON empty array", got)
	}
}

func TestCommentsRunValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    *CommentsOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "invalid repo",
			opts: &CommentsOptions{
				Number:     123,
				Repository: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, _ := iostreams.Test()
			tt.opts.IO = io
			tt.opts.HttpClient = func() (*http.Client, error) {
				return &http.Client{}, nil
			}

			err := commentsRun(tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want containing %s", err, tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCommentsRunJSONIncludesReplies(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":169489896,"discussion_id":"ebeb577","body":"parent","comment_type":"pr_comment","resolved":false,
"reply":[{"id":200021583,"body":"the nested reply","created_at":"2026-11-11T18:52:41+08:00","updated_at":"2026-11-11T18:52:41+08:00","user":{"login":"aflyingto"}}]},{"id":2,"body":"no replies","reply":null}]`))
	}))

	err := commentsRun(&CommentsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("commentsRun() error = %v", err)
	}

	var comments []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &comments); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	reply, ok := comments[0]["reply"].([]interface{})
	if !ok || len(reply) != 1 {
		t.Fatalf("comment 0 reply = %#v, want one reply entry", comments[0]["reply"])
	}
	first := reply[0].(map[string]interface{})
	if first["body"] != "the nested reply" {
		t.Fatalf("reply body = %#v", first["body"])
	}
	if first["id"].(float64) != 200021583 {
		t.Fatalf("reply id = %#v", first["id"])
	}
}

func TestCommentsRunTextShowsReplies(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":169489896,"discussion_id":"ebeb577","body":"parent body","comment_type":"pr_comment","resolved":false,
"reply":[{"id":200021583,"body":"the nested reply","created_at":"2026-11-11T18:52:41+08:00","updated_at":"2026-11-11T18:52:41+08:00","user":{"login":"aflyingto"}}]}]`))
	}))

	err := commentsRun(&CommentsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
	})
	if err != nil {
		t.Fatalf("commentsRun() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Replies (1)") {
		t.Fatalf("text output missing reply count: %q", got)
	}
	if !strings.Contains(got, "the nested reply") {
		t.Fatalf("text output missing reply body: %q", got)
	}
	if !strings.Contains(got, "aflyingto") {
		t.Fatalf("text output missing reply author: %q", got)
	}
	if !strings.Contains(got, "200021583") {
		t.Fatalf("text output missing reply id: %q", got)
	}
}
