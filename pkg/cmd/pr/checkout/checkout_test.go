package checkout

import (
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdCheckout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "checkout PR",
			args:    []string{"123"},
			wantErr: false,
		},
		{
			name:    "checkout with branch name",
			args:    []string{"123", "--branch", "my-feature"},
			wantErr: false,
		},
		{
			name:    "no PR number",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdCheckout(f, func(opts *CheckoutOptions) error {
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

func TestFetchURLHost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		authority string
		want      string
	}{
		// Normal host with port
		{name: "host with port", authority: "gitcode.com:22/owner/repo.git", want: "gitcode.com"},
		{name: "host without port", authority: "gitcode.com/owner/repo.git", want: "gitcode.com"},
		{name: "host without path", authority: "gitcode.com", want: "gitcode.com"},
		{name: "user@host with port", authority: "git@gitcode.com:22", want: "gitcode.com"},
		// IPv6 literal addresses
		{name: "IPv6 loopback with port", authority: "[::1]:22/owner/repo.git", want: "::1"},
		{name: "IPv6 full address with port", authority: "[2001:db8::1]:2222/owner/repo.git", want: "2001:db8::1"},
		{name: "IPv6 without port", authority: "[::1]/owner/repo.git", want: "::1"},
		{name: "IPv6 without brackets no port", authority: "::1", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fetchURLHost(tt.authority)
			if got != tt.want {
				t.Errorf("fetchURLHost(%q) = %q, want %q", tt.authority, got, tt.want)
			}
		})
	}
}

func TestScpHost(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "scp-like url", rawURL: "git@gitcode.com:owner/repo.git", want: "gitcode.com"},
		{name: "scp-like without user", rawURL: "gitcode.com:owner/repo.git", want: "gitcode.com"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := scpHost(tt.rawURL)
			if got != tt.want {
				t.Errorf("scpHost(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}
