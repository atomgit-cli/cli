package view

import (
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view with tag",
			args:    []string{"v1.0.0"},
			wantErr: false,
		},
		{
			name:    "view with web flag",
			args:    []string{"v1.0.0", "--web"},
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

func TestAssetSizeLabel(t *testing.T) {
	t.Parallel()
	if got := assetSizeLabel(api.ReleaseAsset{Size: 0}); got != "unknown size" {
		t.Fatalf("assetSizeLabel() = %q", got)
	}
	if got := assetSizeLabel(api.ReleaseAsset{Size: 42}); got != "42 bytes" {
		t.Fatalf("assetSizeLabel() = %q", got)
	}
}
