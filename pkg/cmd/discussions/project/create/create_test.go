package create

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

func TestNewCmdCreate(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		// runF is injected so createRun validation is skipped; cobra-level
		// arg/number errors surface (no flags are cobra-required here).
		{name: "no args", args: []string{}, wantErr: false},
		{name: "with required flags", args: []string{"-R", "owner/repo", "--title", "T", "--category", "C", "--body", "B"}, wantErr: false},
		{name: "with body-file", args: []string{"-R", "owner/repo", "--title", "T", "--category", "C", "--body-file", "idea.md"}, wantErr: false},
		{name: "with json", args: []string{"-R", "owner/repo", "--title", "T", "--category", "C", "--body", "B", "--json"}, wantErr: false},
		{name: "missing title", args: []string{"-R", "owner/repo", "--category", "C", "--body", "B"}, wantErr: false},
		{name: "missing category", args: []string{"-R", "owner/repo", "--title", "T", "--body", "B"}, wantErr: false},
		{name: "missing body", args: []string{"-R", "owner/repo", "--title", "T", "--category", "C"}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdCreate(f, func(opts *CreateOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateRunValidatesRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		opts *CreateOptions
	}{
		{name: "missing title", opts: &CreateOptions{Repository: "owner/repo", CategoryName: "C", Body: "B"}},
		{name: "missing category", opts: &CreateOptions{Repository: "owner/repo", Title: "T", Body: "B"}},
		{name: "missing body", opts: &CreateOptions{Repository: "owner/repo", Title: "T", CategoryName: "C"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, _ := iostreams.Test()
			tt.opts.IO = io
			tt.opts.HttpClient = func() (*http.Client, error) { return &http.Client{}, nil }
			tt.opts.BaseRepo = func() (string, error) { return "owner/repo", nil }
			err := createRun(tt.opts)
			if err == nil {
				t.Errorf("createRun() expected usage error for %s", tt.name)
			}
		})
	}
}

func TestCreateRunCreatesRepoDiscussion(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotMethod, gotPath, gotBody string
	ioStreams, _, out, _ := iostreams.Test()
	opts := &CreateOptions{
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
						`{"id":"d1","number":7,"title":"New idea","created_at":"2026-01-01","updated_at":"2026-01-01","author":{"login":"user1"}}`)),
				}, nil
			})}, nil
		},
		BaseRepo:     func() (string, error) { return "owner/repo", nil },
		Title:        "New idea",
		Body:         "Description",
		CategoryName: "Ideas",
		JSON:         true,
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss" {
		t.Fatalf("path = %s, want /api/v5/repos/owner/repo/discuss", gotPath)
	}
	var req map[string]string
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request body is not JSON: %v; body=%q", err, gotBody)
	}
	if req["title"] != "New idea" || req["md_content"] != "Description" || req["category_name"] != "Ideas" {
		t.Fatalf("request body = %q, want title/md_content/category_name", gotBody)
	}

	var result struct {
		ID     string `json:"id"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if result.ID != "d1" || result.Number != 7 {
		t.Fatalf("result = %#v, want created discussion", result)
	}
}

func TestCreateRunResolvesRepoFromFlagOverBaseRepo(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath string
	ioStreams, _, _, _ := iostreams.Test()
	opts := &CreateOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotPath = req.URL.Path
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"id":"d1","number":7,"title":"T","created_at":"2026-01-01","updated_at":"2026-01-01"}`)),
				}, nil
			})}, nil
		},
		BaseRepo:     func() (string, error) { return "other/base", nil },
		Repository:   "owner/repo",
		Title:        "T",
		Body:         "B",
		CategoryName: "C",
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss" {
		t.Fatalf("path = %s, want /api/v5/repos/owner/repo/discuss", gotPath)
	}
}

func TestCreateRunAPIError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	opts := &CreateOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Status:     http.StatusText(http.StatusInternalServerError),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"message":"boom"}`)),
				}, nil
			})}, nil
		},
		BaseRepo:     func() (string, error) { return "owner/repo", nil },
		Title:        "T",
		Body:         "B",
		CategoryName: "C",
	}

	if err := createRun(opts); err == nil {
		t.Fatal("createRun() expected error on API failure")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial output", out.String())
	}
}
