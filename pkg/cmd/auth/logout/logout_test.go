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
		name       string
		hostname   string
		set        map[string]string
		wantStdout []string
		wantStderr []string
		notStdout  []string
		notStderr  []string
	}{
		{
			name:       "no token environment variable",
			hostname:   "gitcode.com",
			set:        map[string]string{},
			wantStdout: []string{"Cleared stored authentication for gitcode.com"},
			wantStderr: nil,
			notStdout:  []string{"Environment token", "unset "},
			notStderr:  []string{"Environment token", "unset GC_TOKEN", "unset GITCODE_TOKEN"},
		},
		{
			name:       "GC_TOKEN only",
			hostname:   "gitcode.com",
			set:        map[string]string{"GC_TOKEN": "env-token"},
			wantStdout: []string{"Cleared stored authentication for gitcode.com"},
			wantStderr: []string{"Environment token is still active", "unset GC_TOKEN"},
			notStdout:  []string{"Environment token", "unset "},
			notStderr:  []string{"unset GITCODE_TOKEN"},
		},
		{
			name:     "both token variables",
			hostname: "gitcode.com",
			set: map[string]string{
				"GC_TOKEN":      "env-token",
				"GITCODE_TOKEN": "backup-token",
			},
			wantStdout: []string{"Cleared stored authentication for gitcode.com"},
			wantStderr: []string{"Environment token is still active", "unset GC_TOKEN", "unset GITCODE_TOKEN"},
			notStdout:  []string{"Environment token", "unset "},
			notStderr:  nil,
		},
		{
			name:       "custom host ignores token environment variables",
			hostname:   "enterprise.example.com",
			set:        map[string]string{"GC_TOKEN": "env-token"},
			wantStdout: []string{"Cleared stored authentication for enterprise.example.com"},
			wantStderr: nil,
			notStdout:  []string{"Environment token", "unset "},
			notStderr:  []string{"Environment token", "unset GC_TOKEN", "unset GITCODE_TOKEN"},
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
				Hostname: tt.hostname,
				Yes:      true,
			}

			if err := logoutRun(opts); err != nil {
				t.Fatalf("logoutRun() error = %v, want nil", err)
			}
			for _, sub := range tt.wantStdout {
				if !strings.Contains(out.String(), sub) {
					t.Errorf("stdout missing %q; stdout:\n%s", sub, out.String())
				}
			}
			for _, sub := range tt.wantStderr {
				if !strings.Contains(errOut.String(), sub) {
					t.Errorf("stderr missing %q; stderr:\n%s", sub, errOut.String())
				}
			}
			for _, sub := range tt.notStdout {
				if strings.Contains(out.String(), sub) {
					t.Errorf("stdout unexpectedly contains %q; stdout:\n%s", sub, out.String())
				}
			}
			for _, sub := range tt.notStderr {
				if strings.Contains(errOut.String(), sub) {
					t.Errorf("stderr unexpectedly contains %q; stderr:\n%s", sub, errOut.String())
				}
			}
		})
	}
}
