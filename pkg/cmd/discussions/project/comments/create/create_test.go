package create

import (
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdCreate(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: true},
		{name: "missing body", args: []string{"42", "-R", "owner/repo"}, wantErr: false},
		{name: "with required args", args: []string{"42", "-R", "owner/repo", "--body", "test"}, wantErr: false},
		{name: "with body-file", args: []string{"42", "-R", "owner/repo", "--body-file", "comment.md"}, wantErr: false},
		{name: "with json", args: []string{"42", "-R", "owner/repo", "--body", "test", "--json"}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdCreate(f, func(opts *CreateOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
