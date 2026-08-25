package view

import (
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
			name:    "with sha and repo",
			args:    []string{"abc123", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "with show-diff flag",
			args:    []string{"abc123", "-R", "owner/repo", "--show-diff"},
			wantErr: false,
		},
		{
			name:    "with json flag",
			args:    []string{"abc123", "-R", "owner/repo", "--json"},
			wantErr: false,
		},
		{
			name:    "missing sha",
			args:    []string{},
			wantErr: true,
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

func TestViewRun_CommitNotFoundReturnsNotFoundExitCode(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"commit not found"}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Repository: "owner/repo",
		SHA:        "missing",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitNotFound)
	}
}

func TestViewRun_CommitNotFoundEmbeddedErrorCodeReturnsNotFoundExitCode(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error_code":404,"error_code_name":"UN_KNOW","error_message":"404 Not Found Commit"}`))
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Repository: "owner/repo",
		SHA:        "missing",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitNotFound)
	}
}
