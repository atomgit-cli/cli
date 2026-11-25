package create

import (
	"testing"

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
			name:    "create label",
			args:    []string{"bug"},
			wantErr: false,
		},
		{
			name:    "create with color",
			args:    []string{"bug", "--color", "#ff0000"},
			wantErr: false,
		},
		{
			name:    "create with description",
			args:    []string{"enhancement", "--description", "New feature"},
			wantErr: false,
		},
		{
			name:    "no label name",
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
