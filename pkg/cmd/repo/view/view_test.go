package view

import (
	"bytes"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view with repo",
			args:    []string{"owner/repo"},
			wantErr: false,
		},
		{
			name:    "view with web flag",
			args:    []string{"owner/repo", "--web"},
			wantErr: false,
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

func TestParseRepo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		repo      string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "valid repo",
			repo:      "owner/repo",
			wantOwner: "owner",
			wantName:  "repo",
			wantErr:   false,
		},
		{
			name:    "empty repo requires explicit repo",
			repo:    "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			repo:    "invalid",
			wantErr: true,
		},
		{
			name:    "too many parts",
			repo:    "owner/repo/extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOwner, gotName, err := parseRepo(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotOwner != tt.wantOwner {
					t.Errorf("parseRepo() owner = %v, want %v", gotOwner, tt.wantOwner)
				}
				if gotName != tt.wantName {
					t.Errorf("parseRepo() name = %v, want %v", gotName, tt.wantName)
				}
			}
		})
	}
}

func TestViewRunUsesDetectedRepo(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	transport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v5/repos/owner/repo" {
			t.Fatalf("request path = %q, want %q", req.URL.Path, "/api/v5/repos/owner/repo")
		}

		body := `{"name":"repo","full_name":"owner/repo","web_url":"https://gitcode.com/owner/repo","default_branch":"main","language":"Go","stargazers_count":1,"forks_count":2,"open_issues_count":3}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	opts := &ViewOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: transport}, nil
		},
		BaseRepo: func() (string, error) {
			return "owner/repo", nil
		},
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	output, ok := f.IOStreams.Out.(*bytes.Buffer)
	if !ok {
		t.Fatalf("output writer type = %T, want *bytes.Buffer", f.IOStreams.Out)
	}
	if !strings.Contains(output.String(), "owner/repo") {
		t.Fatalf("output = %q, want repository name", output.String())
	}
}
