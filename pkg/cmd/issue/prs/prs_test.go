package prs

import (
	"encoding/json"
	"net/http"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdPrs(t *testing.T) {
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
			name:    "with json",
			args:    []string{"123", "-R", "owner/repo", "--json"},
			wantErr: false,
		},
		{
			name:    "no issue number",
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
			f := cmdutil.TestFactory()
			cmd := NewCmdPrs(f, func(opts *PrsOptions) error {
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

func TestPrsRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/issues/123/pull_requests" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "mode=1" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"number":7,"title":"fix","state":"open"}]`))
	}))

	err := prsRun(&PrsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Repository: "owner/repo",
		Number:     123,
		Mode:       1,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("prsRun() error = %v", err)
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &prs); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if len(prs) != 1 || prs[0]["title"] != "fix" {
		t.Fatalf("unexpected JSON output: %#v", prs)
	}
}

func TestPrsRunJSONEmptyOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))

	err := prsRun(&PrsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Repository: "owner/repo",
		Number:     123,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("prsRun() error = %v", err)
	}
	if got := out.String(); got != "[]\n" {
		t.Fatalf("output = %q, want JSON empty array", got)
	}
}
