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
			name:    "delete with tag",
			args:    []string{"v1.0.0"},
			wantErr: false,
		},
		{
			name:    "delete with yes flag",
			args:    []string{"v1.0.0", "--yes"},
			wantErr: false,
		},
		{
			name:    "no tag specified",
			args:    []string{},
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

func TestDeleteBaseRepoInjected(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	var capturedBaseRepo func() (string, error)
	cmd := NewCmdDelete(f, func(opts *DeleteOptions) error {
		capturedBaseRepo = opts.BaseRepo
		return nil
	})
	cmd.SetArgs([]string{"v1.0.0", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if capturedBaseRepo == nil {
		t.Fatal("opts.BaseRepo is nil, want injected from f.BaseRepo")
	}
}
