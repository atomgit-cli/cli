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
		{name: "no args", args: []string{}, wantErr: true},
		{name: "one arg", args: []string{"42", "--org", "my-org", "--body", "B"}, wantErr: true},
		{name: "invalid number", args: []string{"abc", "cid", "--org", "my-org", "--body", "B"}, wantErr: true},
		{name: "valid with body", args: []string{"42", "cid", "--org", "my-org", "--body", "B"}, wantErr: false},
		{name: "valid with body-file", args: []string{"42", "cid", "--org", "my-org", "--body-file", "c.md"}, wantErr: false},
		{name: "valid with json", args: []string{"42", "cid", "--org", "my-org", "--body", "B", "--json"}, wantErr: false},
		{name: "missing org", args: []string{"42", "cid", "--body", "B"}, wantErr: true},
		{name: "missing body", args: []string{"42", "cid", "--org", "my-org"}, wantErr: false},
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

func TestEditRunUpdatesOrgComment(t *testing.T) {
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
						`{"id":"c1","created_at":"2026-01-01","author":{"login":"user1"},"content":"Updated text","md_content":"Updated text","like_total":1,"reply_total":0}`)),
				}, nil
			})}, nil
		},
		Org:       "my-org",
		Number:    42,
		CommentID: "c1",
		Body:      "Updated text",
		JSON:      true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/42/comment/c1" {
		t.Fatalf("path = %s, want /api/v5/orgs/my-org/discuss/42/comment/c1", gotPath)
	}
	var req map[string]string
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request body is not JSON: %v; body=%q", err, gotBody)
	}
	if req["md_content"] != "Updated text" {
		t.Fatalf("request body = %q, want md_content", gotBody)
	}

	var result struct {
		ID        string `json:"id"`
		MdContent string `json:"md_content"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if result.ID != "c1" || result.MdContent != "Updated text" {
		t.Fatalf("result = %#v, want updated comment", result)
	}
}

func TestEditRunValidatesRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		opts *EditOptions
	}{
		{name: "missing org", opts: &EditOptions{Number: 42, CommentID: "c1", Body: "B"}},
		{name: "missing comment-id", opts: &EditOptions{Org: "my-org", Number: 42, Body: "B"}},
		{name: "missing body", opts: &EditOptions{Org: "my-org", Number: 42, CommentID: "c1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, _ := iostreams.Test()
			tt.opts.IO = io
			tt.opts.HttpClient = func() (*http.Client, error) { return &http.Client{}, nil }
			err := editRun(tt.opts)
			if err == nil {
				t.Errorf("editRun() expected usage error for %s", tt.name)
			}
		})
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
		Org:       "my-org",
		Number:    42,
		CommentID: "c1",
		Body:      "B",
	}

	if err := editRun(opts); err == nil {
		t.Fatal("editRun() expected error on API failure")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial output", out.String())
	}
}
