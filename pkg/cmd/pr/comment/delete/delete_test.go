package delete

import (
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "delete with ID and repo",
			args:    []string{"123", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "delete with --yes",
			args:    []string{"123", "-R", "owner/repo", "--yes"},
			wantErr: false,
		},
		{
			name:    "no comment ID",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid comment ID",
			args:    []string{"abc", "-R", "owner/repo"},
			wantErr: true,
		},
		{
			name:    "comment ID zero",
			args:    []string{"0", "-R", "owner/repo"},
			wantErr: true,
		},
		{
			name:    "comment ID negative",
			args:    []string{"-1", "-R", "owner/repo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdDelete(f, func(opts *DeleteOptions) error {
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
