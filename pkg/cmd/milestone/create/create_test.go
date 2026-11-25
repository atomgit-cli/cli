package create

import (
	"net/http"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "create milestone",
			args:    []string{"v1.0"},
			wantErr: false,
		},
		{
			name:    "create with description",
			args:    []string{"v1.0", "--description", "First release"},
			wantErr: false,
		},
		{
			name:    "create with due date",
			args:    []string{"v2.0", "--due-date", "2024-12-31"},
			wantErr: false,
		},
		{
			name:    "no title",
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

func TestCreateRun_InvalidDueDateReturnsUsageExitCode(t *testing.T) {
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := createRun(&CreateOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Title:      "v1.0",
		DueDate:    "2024/12/31",
	})
	if err == nil {
		t.Fatal("expected invalid due date error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestCreateRun_MissingTokenReturnsAuthExitCode(t *testing.T) {
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	io, _, _, _ := testutil.NewTestIOStreams()

	err := createRun(&CreateOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Title:      "v1.0",
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitAuth)
	}
}
