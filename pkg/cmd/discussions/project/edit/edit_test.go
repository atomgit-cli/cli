package edit

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdEdit(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		// runF is injected so editRun validation is skipped; only cobra-level
		// errors (missing args, bad number) surface.
		{name: "no args", args: []string{}, wantErr: true},
		{name: "invalid number", args: []string{"abc", "-R", "owner/repo", "--title", "T"}, wantErr: true},
		{name: "valid with title", args: []string{"42", "-R", "owner/repo", "--title", "T"}, wantErr: false},
		{name: "valid with body", args: []string{"42", "-R", "owner/repo", "--body", "B"}, wantErr: false},
		{name: "valid with category", args: []string{"42", "-R", "owner/repo", "--category", "C"}, wantErr: false},
		{name: "no changes", args: []string{"42", "-R", "owner/repo"}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdEdit(f, func(opts *EditOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEditRunValidatesNoChanges(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{}, nil
		},
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		Number:   42,
	}
	if err := editRun(opts); err == nil {
		t.Errorf("editRun() expected usage error when no changes specified")
	}
}

func TestEditRunValidatesEmptyBody(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{}, nil
		},
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		Number:   42,
		BodyFile: "-",
	}
	if err := editRun(opts); err == nil {
		t.Errorf("editRun() expected usage error when body resolves to empty")
	}
}

func TestEditRunUpdatesRepoDiscussion(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotMethod, gotPath, gotBody string
	ioStreams, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotMethod = req.Method
				gotPath = req.URL.Path
				b, _ := io.ReadAll(req.Body)
				gotBody = string(b)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"id":"d1","number":42,"title":"Updated title","created_at":"2026-01-01","updated_at":"2026-01-02","author":{"login":"user1"}}`)),
				}, nil
			})}, nil
		},
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		Number:   42,
		Title:    "Updated title",
		JSON:     true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/42" {
		t.Fatalf("path = %s, want /api/v5/repos/owner/repo/discuss/42", gotPath)
	}
	var req map[string]string
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request body is not JSON: %v; body=%q", err, gotBody)
	}
	if req["title"] != "Updated title" {
		t.Fatalf("request body = %q, want title", gotBody)
	}
	if _, hasBody := req["md_content"]; hasBody {
		t.Fatalf("request body = %q, want only changed fields", gotBody)
	}

	var result struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if result.Number != 42 || result.Title != "Updated title" {
		t.Fatalf("result = %#v, want updated discussion", result)
	}
}

func TestEditRunUpdatesRepoDiscussionBodyAndCategory(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotBody string
	ioStreams, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				b, _ := io.ReadAll(req.Body)
				gotBody = string(b)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"id":"d1","number":42,"title":"T","created_at":"2026-01-01","updated_at":"2026-01-02"}`)),
				}, nil
			})}, nil
		},
		BaseRepo:     func() (string, error) { return "owner/repo", nil },
		Number:       42,
		Body:         "New body",
		CategoryName: "Q&A",
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}

	var req map[string]string
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request body is not JSON: %v; body=%q", err, gotBody)
	}
	if req["md_content"] != "New body" || req["category_name"] != "Q&A" {
		t.Fatalf("request body = %q, want md_content and category_name", gotBody)
	}
	if !strings.Contains(out.String(), "Discussion") {
		t.Fatalf("stdout = %q, want rendered discussion", out.String())
	}
}

func TestEditRunAPIError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     http.StatusText(http.StatusNotFound),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
				}, nil
			})}, nil
		},
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		Number:   42,
		Title:    "T",
	}

	if err := editRun(opts); err == nil {
		t.Fatal("editRun() expected error on API failure")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial output", out.String())
	}
}
