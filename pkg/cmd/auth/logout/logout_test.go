package logout

import (
	"runtime"
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

func TestLogoutNormalizesAndValidatesHostname(t *testing.T) {
	newOpts := func(io *iostreams.IOStreams, hostname string) *LogoutOptions {
		return &LogoutOptions{
			IO: io,
			Config: func() (config.Config, error) {
				return config.New(), nil
			},
			Hostname: hostname,
			Yes:      true,
		}
	}

	t.Run("normalizes explicit hostname", func(t *testing.T) {
		t.Setenv("GC_CONFIG_DIR", t.TempDir())
		t.Setenv("GC_HOST", "")
		t.Setenv("GC_TOKEN", "")
		t.Setenv("GITCODE_TOKEN", "")

		io, _, out, _ := iostreams.Test()
		if err := logoutRun(newOpts(io, "GITCODE.COM")); err != nil {
			t.Fatalf("logoutRun() error = %v, want nil", err)
		}
		if !strings.Contains(out.String(), "Cleared stored authentication for gitcode.com") {
			t.Fatalf("output missing normalized hostname: %q", out.String())
		}
	})

	t.Run("rejects invalid explicit hostname", func(t *testing.T) {
		t.Setenv("GC_CONFIG_DIR", t.TempDir())
		t.Setenv("GC_HOST", "")
		t.Setenv("GC_TOKEN", "")
		t.Setenv("GITCODE_TOKEN", "")

		io, _, out, _ := iostreams.Test()
		err := logoutRun(newOpts(io, "https://gitcode.com"))
		if err == nil {
			t.Fatal("logoutRun() error = nil, want invalid-host error (exit 1)")
		}
		// Invalid hostname exits 1 like the auth family (login/status/
		// token/switch), unified by issue 658.
		if code := cmdutil.ExitCode(err); code != cmdutil.ExitError {
			t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitError)
		}
		if out.String() != "" {
			t.Fatalf("output = %q, want empty", out.String())
		}
	})

	t.Run("rejects invalid default hostname", func(t *testing.T) {
		t.Setenv("GC_CONFIG_DIR", t.TempDir())
		t.Setenv("GC_HOST", "https://invalid.example.com")
		t.Setenv("GC_TOKEN", "")
		t.Setenv("GITCODE_TOKEN", "")

		io, _, out, _ := iostreams.Test()
		err := logoutRun(newOpts(io, ""))
		if err == nil {
			t.Fatal("logoutRun() error = nil, want invalid-host error (exit 1)")
		}
		// Invalid hostname exits 1 like the auth family (login/status/
		// token/switch), unified by issue 658.
		if code := cmdutil.ExitCode(err); code != cmdutil.ExitError {
			t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitError)
		}
		if out.String() != "" {
			t.Fatalf("output = %q, want empty", out.String())
		}
	})
}

func TestLogoutDefaultCustomHostIgnoresTokenEnvironmentVariables(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "enterprise.example.com")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, out, errOut := iostreams.Test()
	opts := &LogoutOptions{
		IO: io,
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
		Yes: true,
	}

	if err := logoutRun(opts); err != nil {
		t.Fatalf("logoutRun() error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "Cleared stored authentication for enterprise.example.com") {
		t.Fatalf("output missing custom host: %q", out.String())
	}
	if strings.Contains(errOut.String(), "Environment token") || strings.Contains(out.String(), "Environment token") {
		t.Fatalf("custom host output contains environment hint; stdout:\n%s\nstderr:\n%s", out.String(), errOut.String())
	}
}

func TestLogoutEnvHintGoesToStderrAndListsOnlySetVars(t *testing.T) {
	clearGC := "unset GC_TOKEN"
	clearGitcode := "unset GITCODE_TOKEN"
	if runtime.GOOS == "windows" {
		clearGC = "Remove-Item Env:GC_TOKEN"
		clearGitcode = "Remove-Item Env:GITCODE_TOKEN"
	}

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
			notStdout:  []string{"Environment token", clearGC, clearGitcode},
			notStderr:  []string{"Environment token", clearGC, clearGitcode},
		},
		{
			name:       "GC_TOKEN only",
			hostname:   "gitcode.com",
			set:        map[string]string{"GC_TOKEN": "env-token"},
			wantStdout: []string{"Cleared stored authentication for gitcode.com"},
			wantStderr: []string{
				"Environment token is still active in this terminal",
				"Clear it for the current session",
				clearGC,
				"remove or update that definition as well",
			},
			notStdout: []string{"Environment token", clearGC, clearGitcode},
			notStderr: []string{clearGitcode},
		},
		{
			name:     "both token variables",
			hostname: "gitcode.com",
			set: map[string]string{
				"GC_TOKEN":      "env-token",
				"GITCODE_TOKEN": "backup-token",
			},
			wantStdout: []string{"Cleared stored authentication for gitcode.com"},
			wantStderr: []string{
				"Environment token is still active in this terminal",
				"Clear it for the current session",
				clearGC,
				clearGitcode,
				"remove or update that definition as well",
			},
			notStdout: []string{"Environment token", clearGC, clearGitcode},
			notStderr: nil,
		},
		{
			name:       "custom host ignores token environment variables",
			hostname:   "enterprise.example.com",
			set:        map[string]string{"GC_TOKEN": "env-token"},
			wantStdout: []string{"Cleared stored authentication for enterprise.example.com"},
			wantStderr: nil,
			notStdout:  []string{"Environment token", clearGC, clearGitcode},
			notStderr:  []string{"Environment token", clearGC, clearGitcode},
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

func TestLogoutCmdBindsUsernameArg(t *testing.T) {
	// RunE must wire args[0] into opts.Username: a positional username must
	// log out that account, never silently fall back to the active user
	// (issue 658).
	f := cmdutil.TestFactory()
	var gotUser string
	cmd := NewCmdLogout(f, func(opts *LogoutOptions) error {
		gotUser = opts.Username
		return nil
	})
	cmd.SetArgs([]string{"bob"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotUser != "bob" {
		t.Fatalf("Username = %q, want bob (positional arg must bind)", gotUser)
	}
}

func TestLogoutRunPositionalUsernameLogsOutThatUser(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	auth := cfg.Authentication()
	if _, err := auth.Login("gitcode.com", "alice", "tokenA", "https", false); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Login("gitcode.com", "bob", "tokenB", "https", false); err != nil {
		t.Fatal(err)
	}
	io, _, _, _ := iostreams.Test()
	// Log out alice — the NON-active user (bob is active after the two
	// logins): the outcome distinguishes this fix from the old bug, which
	// silently logged out the active account instead.
	err := logoutRun(&LogoutOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com", Username: "alice", Yes: true})
	if err != nil {
		t.Fatalf("logoutRun() error = %v", err)
	}
	users := auth.LoginUsers("gitcode.com")
	if len(users) != 1 || users[0] != "bob" {
		t.Fatalf("after logout of alice, users = %v, want [bob]", users)
	}
	active, _ := auth.ActiveUser("gitcode.com")
	if active != "bob" {
		t.Fatalf("active after logout of alice = %q, want bob (unchanged)", active)
	}
}

func TestLogoutCmdConflictingUsernamesExit2(t *testing.T) {
	// A positional username and --username with different values must be
	// a usage error (never a silent flag-wins), naming both values.
	f := cmdutil.TestFactory()
	cmd := NewCmdLogout(f, func(opts *LogoutOptions) error {
		t.Fatal("runF must not run when usernames conflict")
		return nil
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"bob", "-u", "alice"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
	for _, want := range []string{`"alice"`, `"bob"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want both values mentioned", err.Error())
		}
	}
}

func TestLogoutCmdTooManyArgsExit2(t *testing.T) {
	// The guard must be the argument validation, not a downstream
	// confirmation gate: runF fatals if RunE executes at all (the old
	// nil-runF form passed on main via the confirmation error instead).
	f := cmdutil.TestFactory()
	cmd := NewCmdLogout(f, func(opts *LogoutOptions) error {
		t.Fatal("runF must not run when arguments exceed the limit")
		return nil
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"bob", "alice"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want cobra arg validation error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d (MaximumNArgs)", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestLogoutCmdEmptyUsernameArgExit2(t *testing.T) {
	// An explicitly empty positional argument (script's unset variable)
	// must be a usage error, never a silent active-account logout —
	// the same bug class issue 658 kills.
	f := cmdutil.TestFactory()
	cmd := NewCmdLogout(f, func(opts *LogoutOptions) error {
		t.Fatal("runF must not run for an empty username argument")
		return nil
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{""})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestLogoutCmdEmptyFlagUsernameExit2(t *testing.T) {
	// An explicitly empty --username is a usage error too, not a silent
	// active-account logout — the docs promise the two forms are
	// equivalent, and the boundary review reproduced the active-account
	// wipe with exactly this spelling (incident evidence for issue 658).
	f := cmdutil.TestFactory()
	cmd := NewCmdLogout(f, func(opts *LogoutOptions) error {
		t.Fatal("runF must not run for an empty --username")
		return nil
	})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"-u", ""})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}
