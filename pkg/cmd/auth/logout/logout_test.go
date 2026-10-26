package logout

import (
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestLogoutRunRemovesStoredToken(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("gitcode.com", "tester", "stored-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &LogoutOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
		Yes: true, // Skip confirmation in test
	}

	if err := logoutRun(opts); err != nil {
		t.Fatalf("logoutRun() error = %v", err)
	}

	token, source := config.New().Authentication().ActiveToken("gitcode.com")
	if token != "" || source != "" {
		t.Fatalf("ActiveToken() after logout = %q, %q", token, source)
	}
	if !strings.Contains(out.String(), "Cleared stored authentication for gitcode.com") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLogoutRequiresConfirmation(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("gitcode.com", "tester", "stored-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &LogoutOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
		Yes: false, // Require confirmation
	}

	err := logoutRun(opts)
	if err == nil {
		t.Fatalf("logoutRun() expected error for non-interactive mode without --yes")
	}
	if !strings.Contains(err.Error(), "confirmation required in non-interactive mode") {
		t.Fatalf("logoutRun() error = %v, expected confirmation required error", err)
	}
}

func TestLogoutEnvHintGoesToStderrAndListsOnlySetVars(t *testing.T) {
	tests := []struct {
		name    string
		set     map[string]string
		wantSub []string
		notSub  []string
	}{
		{
			name:    "no token environment variable",
			set:     map[string]string{},
			wantSub: []string{"Cleared stored authentication for gitcode.com"},
			notSub:  []string{"Environment token", "unset GC_TOKEN", "unset GITCODE_TOKEN"},
		},
		{
			name:    "GC_TOKEN only",
			set:     map[string]string{"GC_TOKEN": "env-token"},
			wantSub: []string{"Environment token is still active", "unset GC_TOKEN"},
			notSub:  []string{"unset GITCODE_TOKEN"},
		},
		{
			name: "both token variables",
			set: map[string]string{
				"GC_TOKEN":      "env-token",
				"GITCODE_TOKEN": "backup-token",
			},
			wantSub: []string{"unset GC_TOKEN", "unset GITCODE_TOKEN"},
			notSub:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_CONFIG_DIR", t.TempDir())
			t.Setenv("GC_TOKEN", "")
			t.Setenv("GITCODE_TOKEN", "")
			for name, value := range tt.set {
				t.Setenv(name, value)
			}

			io, _, out, errOut := iostreams.Test()
			opts := &LogoutOptions{
				IO: io,
				Config: func() (config.Config, error) {
					return config.New(), nil
				},
				Hostname: "gitcode.com",
				Yes:      true,
			}

			if err := logoutRun(opts); err != nil {
				t.Fatalf("logoutRun() error = %v, want nil", err)
			}
			if strings.Contains(out.String(), "Environment token") {
				t.Fatalf("stdout contains environment hint: %q", out.String())
			}
			if strings.Contains(out.String(), "unset ") {
				t.Fatalf("stdout contains unset guidance: %q", out.String())
			}
			for _, sub := range tt.wantSub {
				if !strings.Contains(errOut.String(), sub) && !strings.Contains(out.String(), sub) {
					t.Errorf("output missing %q; stdout:\n%s\nstderr:\n%s", sub, out.String(), errOut.String())
				}
			}
			for _, sub := range tt.notSub {
				if strings.Contains(errOut.String(), sub) || strings.Contains(out.String(), sub) {
					t.Errorf("output unexpectedly contains %q; stdout:\n%s\nstderr:\n%s", sub, out.String(), errOut.String())
				}
			}
		})
	}
}
