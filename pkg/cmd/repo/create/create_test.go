package create

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "create with name",
			args:    []string{"my-repo", "--public"},
			wantErr: false,
		},
		{
			name:    "create with description",
			args:    []string{"my-repo", "--description", "Test repo"},
			wantErr: false,
		},
		{
			name:    "create with json output",
			args:    []string{"my-repo", "--json"},
			wantErr: false,
		},
		{
			name:    "no name",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
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

func TestCreateRunJSONWritesCreatedRepo(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:          f.IOStreams,
		HttpClient:  repoCreateHTTPClient(t, `{"name":"my-repo","full_name":"owner/my-repo","web_url":"https://gitcode.com/owner/my-repo","http_url_to_repo":"https://gitcode.com/owner/my-repo.git","ssh_url_to_repo":"git@gitcode.com:owner/my-repo.git"}`),
		Name:        "my-repo",
		Description: "Test repo",
		JSON:        true,
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	var got api.Repository
	out := f.IOStreams.Out.(*bytes.Buffer).String()
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out)
	}
	if got.FullName != "owner/my-repo" || got.HTMLURL != "https://gitcode.com/owner/my-repo" {
		t.Fatalf("JSON output = %+v", got)
	}
	if strings.Contains(out, "Created repository") {
		t.Fatalf("JSON output contains text banner: %q", out)
	}
}

func repoCreateHTTPClient(t *testing.T, body string) func() (*http.Client, error) {
	t.Helper()
	return func() (*http.Client, error) {
		return &http.Client{
			Transport: repoRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/api/v5/user/repos" {
					t.Fatalf("request path = %s, want /api/v5/user/repos", req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		}, nil
	}
}

type repoRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn repoRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
