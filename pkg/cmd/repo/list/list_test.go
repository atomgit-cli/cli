package list

import (
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/output"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		return nil
	})

	if cmd == nil {
		t.Fatal("NewCmdList returned nil")
	}
	if cmd.Use != "list" {
		t.Errorf("Expected Use 'list', got %q", cmd.Use)
	}
}

func TestListOptions(t *testing.T) {
	t.Parallel()
	opts := &ListOptions{
		Limit:      30,
		Visibility: "public",
	}

	if opts.Limit != 30 {
		t.Errorf("Expected Limit 30, got %d", opts.Limit)
	}
	if opts.Visibility != "public" {
		t.Errorf("Expected Visibility 'public', got %q", opts.Visibility)
	}
}

func TestListRunUsesOrgEndpointForOwner(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotPath string
	opts := &ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Limit:      50,
		Visibility: "private",
		Owner:      "infra-test",
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if gotPath != "/api/v5/orgs/infra-test/repos?per_page=50&visibility=private" {
		t.Fatalf("request path = %q, want %q", gotPath, "/api/v5/orgs/infra-test/repos?per_page=50&visibility=private")
	}
}

func TestListRunUsesUserEndpointWithVisibility(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotPath string
	opts := &ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Limit:      25,
		Visibility: "public",
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if gotPath != "/api/v5/user/repos?per_page=25&visibility=public" {
		t.Fatalf("request path = %q, want %q", gotPath, "/api/v5/user/repos?per_page=25&visibility=public")
	}
}

func TestResolveOutputFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		jsonFlag bool
		raw      string
		want     output.Format
		wantErr  bool
	}{
		{name: "default", want: output.FormatSimple},
		{name: "table", raw: "table", want: output.FormatTable},
		{name: "json flag", jsonFlag: true, want: output.FormatJSON},
		{name: "invalid format", raw: "yaml", wantErr: true},
		{name: "json incompatible format", jsonFlag: true, raw: "table", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveOutputFormat(tt.jsonFlag, tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveOutputFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("resolveOutputFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func listTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
