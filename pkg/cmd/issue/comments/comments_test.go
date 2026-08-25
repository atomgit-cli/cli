package comments

import (
	"encoding/json"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestNewCmdComments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid issue number",
			args:    []string{"123", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "with flags",
			args:    []string{"123", "-R", "owner/repo", "--limit", "5", "--order", "desc", "--since", "2024-01-01T00:00:00+08:00", "--json"},
			wantErr: false,
		},
		{
			name:    "missing issue number",
			args:    []string{"-R", "owner/repo"},
			wantErr: true,
		},
		{
			name:    "invalid issue number",
			args:    []string{"abc", "-R", "owner/repo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdComments(cmdutil.TestFactory(), func(opts *CommentsOptions) error {
				return nil
			})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommentsRunFormatsTextOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &CommentsOptions{
		IO:       io,
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if got := req.URL.RawQuery; got != "order=desc&page=1&per_page=100&since=2024-01-01T00%3A00%3A00%2B08%3A00" {
						t.Fatalf("RawQuery = %q", got)
					}
					return jsonResponse(`[{"id":166027129,"body":"first line\nsecond line","user":{"login":"aflyingto"},"created_at":"2026-03-22T19:05:07+08:00","updated_at":"2026-03-22T19:08:02+08:00"}]`), nil
				}),
			}, nil
		},
		Number: 123,
		Limit:  2,
		Order:  "desc",
		Since:  "2024-01-01T00:00:00+08:00",
	}

	t.Setenv("GC_TOKEN", "test-token")

	if err := commentsRun(opts); err != nil {
		t.Fatalf("commentsRun() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Comments on issue #123 (1 total):") {
		t.Fatalf("output missing header: %q", got)
	}
	if !strings.Contains(got, "ID: 166027129") {
		t.Fatalf("output missing comment ID: %q", got)
	}
	if !strings.Contains(got, "first line") || !strings.Contains(got, "second line") {
		t.Fatalf("output missing comment body: %q", got)
	}
}

func TestCommentsRunJSONOutput(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &CommentsOptions{
		IO:       io,
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return jsonResponse(`[{"id":"1","body":"json body","user":{"login":"tester"},"created_at":"2026-03-22T19:05:07+08:00"}]`), nil
				}),
			}, nil
		},
		Number: 123,
		JSON:   true,
	}

	t.Setenv("GC_TOKEN", "test-token")

	if err := commentsRun(opts); err != nil {
		t.Fatalf("commentsRun() error = %v", err)
	}

	var comments []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &comments); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if len(comments) != 1 || comments[0]["body"] != "json body" {
		t.Fatalf("unexpected JSON output: %#v", comments)
	}
}

func TestCommentsRunValidation(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &CommentsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		Order:      "bad",
	}

	t.Setenv("GC_TOKEN", "test-token")

	err := commentsRun(opts)
	if err == nil || !strings.Contains(err.Error(), "must be asc or desc") {
		t.Fatalf("commentsRun() error = %v, want invalid order error", err)
	}
}

func TestFormatID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{name: "string", in: "abc", want: "abc"},
		{name: "float64", in: float64(12), want: "12"},
		{name: "int", in: 12, want: "12"},
		{name: "int64", in: int64(12), want: "12"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cmdutil.FormatAPIID(tt.in); got != tt.want {
				t.Fatalf("FormatAPIID(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       ioNopCloser(body),
	}
}

func ioNopCloser(body string) *readCloser {
	return &readCloser{Reader: strings.NewReader(body)}
}

type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error { return nil }
