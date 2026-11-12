package setupgit

import (
	"errors"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
)

func TestNewCmdSetupGit(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdSetupGit(f, nil)
	if cmd == nil {
		t.Fatal("NewCmdSetupGit returned nil")
	}
	if cmd.Use != "setup-git" {
		t.Errorf("Use = %q, want setup-git", cmd.Use)
	}
	if cmd.Flags().Lookup("hostname") == nil {
		t.Fatal("--hostname flag not registered")
	}
}

func TestNewCmdSetupGitHelpDocumentsDeliveredHelper(t *testing.T) {
	// The git-credential subcommand is delivered; the help text must not
	// claim a follow-up release is still pending.
	f := cmdutil.TestFactory()
	cmd := NewCmdSetupGit(f, nil)
	if strings.Contains(cmd.Long, "follow-up") {
		t.Errorf("Long still mentions a follow-up release: %q", cmd.Long)
	}
	if !strings.Contains(cmd.Long, "git-credential") {
		t.Errorf("Long does not mention the git-credential subcommand: %q", cmd.Long)
	}
}

func TestSetupGitRunConfiguresDefaultHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	var calls [][]string
	opts := &SetupGitOptions{
		IO:     f.IOStreams,
		Config: func() (config.Config, error) { return config.New(), nil },
		runGitConfig: func(args ...string) (string, error) {
			calls = append(calls, args)
			return "", nil
		},
		resolveExecutable: func() (string, error) { return "/usr/local/bin/gc", nil },
	}

	if err := setupGitRun(opts); err != nil {
		t.Fatalf("setupGitRun() error = %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 git calls, got %d: %v", len(calls), calls)
	}
	wantKey := "credential.https://gitcode.com.helper"
	if calls[0][0] != "--global" || calls[0][1] != "--get-all" || calls[0][2] != wantKey {
		t.Fatalf("first call = %v, want --global --get-all %s", calls[0], wantKey)
	}
	if calls[1][0] != "--global" || calls[1][1] != "--add" || calls[1][2] != wantKey {
		t.Fatalf("second call = %v, want --global --add %s", calls[1], wantKey)
	}
	wantHelper := `!"/usr/local/bin/gc" auth git-credential`
	if calls[1][3] != wantHelper {
		t.Fatalf("add value = %q, want %q", calls[1][3], wantHelper)
	}
	if !strings.Contains(out.String(), "Configured git credential helper") {
		t.Fatalf("output = %q", out.String())
	}
	if strings.Contains(out.String(), "follow-up") {
		t.Fatalf("output still mentions a follow-up release: %q", out.String())
	}
}

func TestSetupGitRunCustomHostname(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())

	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	var calls [][]string
	opts := &SetupGitOptions{
		IO:          f.IOStreams,
		Hostname:    "other.example.com",
		HostnameSet: true,
		Config:      func() (config.Config, error) { return config.New(), nil },
		runGitConfig: func(args ...string) (string, error) {
			calls = append(calls, args)
			return "", nil
		},
		resolveExecutable: func() (string, error) { return "/usr/local/bin/gc", nil },
	}

	if err := setupGitRun(opts); err != nil {
		t.Fatalf("setupGitRun() error = %v", err)
	}

	wantKey := "credential.https://other.example.com.helper"
	if len(calls) != 2 || calls[0][2] != wantKey || calls[1][2] != wantKey {
		t.Fatalf("calls = %v, want key %q", calls, wantKey)
	}
}

func TestSetupGitRunIdempotentSkipsExisting(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	helper := `!"/usr/local/bin/gc" auth git-credential`
	var calls int
	opts := &SetupGitOptions{
		IO:       f.IOStreams,
		Hostname: "gitcode.com",
		Config:   func() (config.Config, error) { return config.New(), nil },
		runGitConfig: func(args ...string) (string, error) {
			calls++
			if args[1] == "--get-all" {
				return helper, nil
			}
			return "", nil
		},
		resolveExecutable: func() (string, error) { return "/usr/local/bin/gc", nil },
	}

	if err := setupGitRun(opts); err != nil {
		t.Fatalf("setupGitRun() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 git call (get-all only), got %d", calls)
	}
	if !strings.Contains(out.String(), "already configured") {
		t.Fatalf("output = %q", out.String())
	}
	if strings.Contains(out.String(), "follow-up") {
		t.Fatalf("output still mentions a follow-up release: %q", out.String())
	}
}

func TestSetupGitRunAppendsWhenDifferentHelperExists(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())

	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	var calls [][]string
	opts := &SetupGitOptions{
		IO:       f.IOStreams,
		Hostname: "gitcode.com",
		Config:   func() (config.Config, error) { return config.New(), nil },
		runGitConfig: func(args ...string) (string, error) {
			calls = append(calls, args)
			if args[1] == "--get-all" {
				return "!other-tool get", nil
			}
			return "", nil
		},
		resolveExecutable: func() (string, error) { return "/usr/local/bin/gc", nil },
	}

	if err := setupGitRun(opts); err != nil {
		t.Fatalf("setupGitRun() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected get-all + --add, got %d calls", len(calls))
	}
}

func TestSetupGitRunConfigErrorReturnsError(t *testing.T) {
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}
	opts := &SetupGitOptions{
		IO: f.IOStreams,
		Config: func() (config.Config, error) {
			return nil, errors.New("config boom")
		},
	}
	err := setupGitRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to read config") {
		t.Fatalf("err = %v, want failed to read config", err)
	}
}

func TestSetupGitRunResolveExecutableErrorReturnsError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}
	opts := &SetupGitOptions{
		IO:       f.IOStreams,
		Hostname: "gitcode.com",
		Config:   func() (config.Config, error) { return config.New(), nil },
		resolveExecutable: func() (string, error) {
			return "", errors.New("exec boom")
		},
	}
	err := setupGitRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to resolve gc executable path") {
		t.Fatalf("err = %v, want resolve error", err)
	}
}

func TestSetupGitRunAddFailsReturnsError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}
	opts := &SetupGitOptions{
		IO:       f.IOStreams,
		Hostname: "gitcode.com",
		Config:   func() (config.Config, error) { return config.New(), nil },
		runGitConfig: func(args ...string) (string, error) {
			if args[1] == "--get-all" {
				return "", nil
			}
			return "", errors.New("add boom")
		},
		resolveExecutable: func() (string, error) { return "/usr/local/bin/gc", nil },
	}
	err := setupGitRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to configure git credential helper") {
		t.Fatalf("err = %v, want configure error", err)
	}
}

func TestSetupGitRunInvalidHostReturnsError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())

	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}

	opts := &SetupGitOptions{
		IO:       f.IOStreams,
		Hostname: "https://gitcode.com",
		Config:   func() (config.Config, error) { return config.New(), nil },
		runGitConfig: func(args ...string) (string, error) {
			t.Fatal("git config should not be invoked for an invalid host")
			return "", nil
		},
		resolveExecutable: func() (string, error) { return "/usr/local/bin/gc", nil },
	}

	err := setupGitRun(opts)
	if err == nil {
		t.Fatal("setupGitRun() error = nil, want invalid host error")
	}
	if !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("error = %q, want invalid host", err.Error())
	}
}

func TestHasHelperValue(t *testing.T) {
	cases := []struct {
		name   string
		output string
		helper string
		want   bool
	}{
		{name: "empty", output: "", helper: "x", want: false},
		{name: "single match", output: "x", helper: "x", want: true},
		{name: "multiline match", output: "a\nx\nb", helper: "x", want: true},
		{name: "no match", output: "a\nb", helper: "x", want: false},
		{name: "whitespace trimmed", output: "  x  \n", helper: "x", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasHelperValue(tc.output, tc.helper); got != tc.want {
				t.Errorf("hasHelperValue(%q, %q) = %v, want %v", tc.output, tc.helper, got, tc.want)
			}
		})
	}
}
