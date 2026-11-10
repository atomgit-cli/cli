package auth

import (
	"io"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
)

func TestGitCredentialProtocol(t *testing.T) {
	tests := []struct {
		name, operation, input, env, want string
		wantErr                           bool
	}{
		{name: "stored", operation: "get", input: "protocol=https\nhost=gitcode.com\n\n", want: "username=tester\npassword=stored-value\n\n"},
		{name: "environment", operation: "get", input: "protocol=https\nhost=gitcode.com\n\n", env: "environment-value", want: "username=tester\npassword=environment-value\n\n"},
		{name: "custom host stored only", operation: "get", input: "protocol=https\nhost=enterprise.example.com\n\n", env: "environment-value", want: "username=enterprise-user\npassword=enterprise-value\n\n"},
		{name: "unknown host", operation: "get", input: "protocol=https\nhost=unknown.example.com\n\n", env: "environment-value"},
		{name: "http", operation: "get", input: "protocol=http\nhost=gitcode.com\n\n"},
		{name: "missing host", operation: "get", input: "protocol=https\n\n", env: "environment-value"},
		{name: "malformed", operation: "get", input: "protocol=https\nsecret-input\n\n", wantErr: true},
		{name: "duplicate host", operation: "get", input: "protocol=https\nhost=evil.example\nhost=gitcode.com\n\n", wantErr: true},
		{name: "oversized", operation: "get", input: strings.Repeat("x", 70000), wantErr: true},
		{name: "token injection", operation: "get", input: "protocol=https\nhost=gitcode.com\n\n", env: "value\npassword=injected", wantErr: true},
		{name: "store", operation: "store", input: "protocol=https\nhost=gitcode.com\npassword=replacement\n\n"},
		{name: "erase", operation: "erase", input: "protocol=https\nhost=gitcode.com\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_CONFIG_DIR", t.TempDir())
			t.Setenv("GC_TOKEN", tt.env)
			t.Setenv("GITCODE_TOKEN", "")
			cfg := config.New()
			for _, login := range []struct{ host, user, token string }{{"gitcode.com", "tester", "stored-value"}, {"enterprise.example.com", "enterprise-user", "enterprise-value"}} {
				if _, err := cfg.Authentication().Login(login.host, login.user, login.token, "https", false); err != nil {
					t.Fatal(err)
				}
			}
			f := cmdutil.TestFactory()
			out := &strings.Builder{}
			f.IOStreams.Out = out
			f.IOStreams.ErrOut = io.Discard
			f.IOStreams.In = io.NopCloser(strings.NewReader(tt.input))
			f.Config = func() (config.Config, error) { return cfg, nil }
			cmd := NewCmdAuth(f)
			cmd.SetArgs([]string{"git-credential", tt.operation})
			cmd.SetOut(out)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if out.String() != tt.want {
				t.Fatalf("unexpected credential response (length %d, wanted %d)", out.Len(), len(tt.want))
			}
			if err != nil && (strings.Contains(err.Error(), "secret-input") || strings.Contains(err.Error(), "injected")) {
				t.Fatal("error exposes input")
			}
			token, _ := cfg.Authentication().StoredToken("gitcode.com")
			if token != "stored-value" {
				t.Fatal("helper modified stored login")
			}
		})
	}
}
