package edit

import (
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestNewCmdEdit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "id argument",
			args:    []string{"12345", "-R", "owner/repo", "--body", "updated"},
			wantErr: false,
		},
		{
			name:    "id flag",
			args:    []string{"--id", "12345", "-R", "owner/repo", "--body", "updated"},
			wantErr: false,
		},
		{
			name:    "body file",
			args:    []string{"12345", "-R", "owner/repo", "--body-file", "-"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdEdit(cmdutil.TestFactory(), func(opts *EditOptions) error {
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

func TestEditRunUsesPatchEndpoint(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO:       io,
		BaseRepo: func() (string, error) { return "owner/repo", nil },
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPatch {
						t.Fatalf("method = %s", req.Method)
					}
					if req.URL.Path != "/api/v5/repos/owner/repo/issues/comments/12345" {
						t.Fatalf("path = %s", req.URL.Path)
					}
					return jsonResponse(`{"id":12345,"body":"updated","updated_at":"2026-03-30T13:00:00+08:00"}`), nil
				}),
			}, nil
		},
		ID:   "12345",
		Body: "updated",
	}

	t.Setenv("GC_TOKEN", "test-token")

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Updated issue comment 12345") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestEditRunValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		opts *EditOptions
		want string
	}{
		{
			name: "missing id",
			opts: &EditOptions{},
			want: "comment ID is required",
		},
		{
			name: "missing body",
			opts: &EditOptions{ID: "12345", IO: iostreams.System()},
			want: "comment body is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.opts.IO == nil {
				io, _, _, _ := iostreams.Test()
				tt.opts.IO = io
			}
			err := editRun(tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("editRun() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestGetBody(t *testing.T) {
	t.Parallel()
	io, in, _, _ := iostreams.Test()
	_, _ = io, in
	tests := []struct {
		name string
		opts *EditOptions
		want string
		err  string
	}{
		{
			name: "body",
			opts: &EditOptions{IO: io, Body: "hello"},
			want: "hello",
		},
		{
			name: "stdin",
			opts: &EditOptions{IO: io, BodyFile: "-"},
			want: "from stdin",
		},
		{
			name: "both",
			opts: &EditOptions{IO: io, Body: "a", BodyFile: "-"},
			err:  "cannot use both",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.opts.BodyFile == "-" && tt.opts.Body == "" {
				tt.opts.IO.In = strings.NewReader("from stdin\n")
			}
			got, err := getBody(tt.opts)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("getBody() error = %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("getBody() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("getBody() = %q, want %q", got, tt.want)
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

func TestGetBodyScansInlineBodyForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	f := cmdutil.TestFactory()
	opts := &EditOptions{IO: f.IOStreams, Body: "leaked: secret-token-abc123"}
	_, err := getBody(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("getBody() error = %v, want secret detection error", err)
	}
}
