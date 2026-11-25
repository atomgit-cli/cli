package view

import (
	"encoding/json"
	"net/http"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view comment",
			args:    []string{"123", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "view comment as json",
			args:    []string{"123", "-R", "owner/repo", "--json"},
			wantErr: false,
		},
		{
			name:    "missing comment id",
			args:    []string{"-R", "owner/repo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdView(cmdutil.TestFactory(), func(opts *ViewOptions) error {
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

func TestViewRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/comments/123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"body":"looks good","user":{"login":"tester"}}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		ID:         "123",
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	var comment map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &comment); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if comment["body"] != "looks good" {
		t.Fatalf("unexpected JSON output: %#v", comment)
	}
}
