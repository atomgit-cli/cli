package edit

import (
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdEdit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "edit with --body",
			args:    []string{"123", "--body", "Updated comment"},
			wantErr: false,
		},
		{
			name:    "edit with --body-file",
			args:    []string{"123", "--body-file", "comment.md"},
			wantErr: false,
		},
		{
			name:    "edit with repo flag",
			args:    []string{"123", "--body", "Updated comment", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "no comment ID",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid comment ID",
			args:    []string{"abc", "--body", "Updated comment"},
			wantErr: true,
		},
		{
			name:    "comment ID zero",
			args:    []string{"0", "--body", "Updated comment"},
			wantErr: true,
		},
		{
			name:    "comment ID negative",
			args:    []string{"-1", "--body", "Updated comment"},
			wantErr: true,
		},
		{
			name:    "missing both --body and --body-file",
			args:    []string{"123"},
			wantErr: false, // Cobra passes; validation happens in editRun
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdEdit(f, func(opts *EditOptions) error {
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

func TestGetBodyScansInlineBodyForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	f := cmdutil.TestFactory()
	opts := &EditOptions{IO: f.IOStreams, Body: "leaked: secret-token-abc123"}
	_, err := getBody(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("getBody() error = %v, want secret detection error", err)
	}
}
