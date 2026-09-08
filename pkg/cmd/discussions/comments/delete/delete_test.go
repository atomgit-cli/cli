package delete

import (
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdDelete(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: true},
		{name: "one arg", args: []string{"42"}, wantErr: true},
		{name: "missing org", args: []string{"42", "123"}, wantErr: true},
		{name: "with required args", args: []string{"42", "123", "--org", "my-org"}, wantErr: false},
		{name: "with yes flag", args: []string{"42", "123", "--org", "my-org", "--yes"}, wantErr: false},
		{name: "invalid number", args: []string{"abc", "123", "--org", "my-org"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdDelete(f, func(opts *DeleteOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
