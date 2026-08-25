package setdefault

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// testConfig returns a config backed by a temp dir (isolated from real config).
func testConfig(t *testing.T) config.Config {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	return config.New()
}

func TestNewCmdSetDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: false},
		{name: "explicit repo", args: []string{"owner/repo"}, wantErr: false},
		{name: "with --view", args: []string{"--view"}, wantErr: false},
		{name: "with --unset", args: []string{"--unset"}, wantErr: false},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdSetDefault(f, func(opts *SetDefaultOptions) error {
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

func TestNewCmdSetDefaultFlagsExist(t *testing.T) {
	t.Parallel()
	cmd := NewCmdSetDefault(cmdutil.TestFactory(), func(opts *SetDefaultOptions) error {
		return nil
	})
	for _, flag := range []string{"view", "unset", "json"} {
		flag := flag
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Fatalf("flag %q missing", flag)
		}
	}
}

func TestSetDefaultExplicit(t *testing.T) {
	io, _, _, errOut := iostreams.Test()
	cfg := testConfig(t)
	opts := &SetDefaultOptions{
		IO:         io,
		Config:     func() (config.Config, error) { return cfg, nil },
		Repository: "infra-test/gctest1",
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	got, err := cfg.Get(defaultHost, "default_repo")
	if err != nil {
		t.Fatalf("cfg.Get() error = %v", err)
	}
	if got != "infra-test/gctest1" {
		t.Fatalf("default_repo = %q, want %q", got, "infra-test/gctest1")
	}
	if !strings.Contains(errOut.String(), "Default repository set to") {
		t.Fatalf("stderr should contain confirmation; got: %s", errOut.String())
	}
}

func TestSetDefaultFromGitContext(t *testing.T) {
	io, _, _, errOut := iostreams.Test()
	cfg := testConfig(t)
	opts := &SetDefaultOptions{
		IO:       io,
		Config:   func() (config.Config, error) { return cfg, nil },
		BaseRepo: func() (string, error) { return "owner/repo", nil },
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	got, _ := cfg.Get(defaultHost, "default_repo")
	if got != "owner/repo" {
		t.Fatalf("default_repo = %q, want %q", got, "owner/repo")
	}
	if !strings.Contains(errOut.String(), "Default repository set to") {
		t.Fatalf("stderr should contain confirmation; got: %s", errOut.String())
	}
}

func TestViewDefaultEmpty(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	cfg := testConfig(t)
	opts := &SetDefaultOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
		View:   true,
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No default repository is set") {
		t.Fatalf("output should say no default; got: %s", out.String())
	}
}

func TestViewDefaultSet(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	cfg := testConfig(t)
	_ = cfg.Set(defaultHost, "default_repo", "owner/repo")
	opts := &SetDefaultOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
		View:   true,
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "owner/repo") {
		t.Fatalf("output should contain the repo; got: %s", out.String())
	}
}

func TestViewDefaultJSON(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	cfg := testConfig(t)
	_ = cfg.Set(defaultHost, "default_repo", "owner/repo")
	opts := &SetDefaultOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
		View:   true,
		JSON:   true,
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("output is not valid JSON: %s; raw: %s", err, out.String())
	}
	if got["default_repo"] != "owner/repo" {
		t.Fatalf("default_repo = %q, want %q", got["default_repo"], "owner/repo")
	}
}

func TestViewDefaultJSONEmpty(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	cfg := testConfig(t)
	opts := &SetDefaultOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
		View:   true,
		JSON:   true,
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("output must be valid JSON when --json is set, even if empty; got: %s", out.String())
	}
	if got["default_repo"] != "" {
		t.Fatalf("default_repo = %q, want empty string", got["default_repo"])
	}
}

func TestSetDefaultNoRepoNoGit(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &SetDefaultOptions{
		IO:       io,
		Config:   func() (config.Config, error) { return testConfig(t), nil },
		BaseRepo: func() (string, error) { return "", cmdutil.NewUsageError("not in a git repository") },
	}

	err := setDefaultRun(opts)
	if err == nil {
		t.Fatal("setDefaultRun() error = nil, want usage error")
	}
	var cliErr *cmdutil.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != cmdutil.ExitUsage {
		t.Fatalf("want ExitUsage(2), got %v", err)
	}
}

func TestSetDefaultInvalidRepoFormat(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &SetDefaultOptions{
		IO:         io,
		Config:     func() (config.Config, error) { return testConfig(t), nil },
		Repository: "invalid-no-slash",
	}

	err := setDefaultRun(opts)
	if err == nil {
		t.Fatal("setDefaultRun() error = nil, want usage error for invalid format")
	}
	var cliErr *cmdutil.CLIError
	if !errors.As(err, &cliErr) || cliErr.Code != cmdutil.ExitUsage {
		t.Fatalf("want ExitUsage(2), got %v", err)
	}
}

func TestUnsetDefault(t *testing.T) {
	io, _, _, errOut := iostreams.Test()
	cfg := testConfig(t)
	_ = cfg.Set(defaultHost, "default_repo", "owner/repo")
	opts := &SetDefaultOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
		Unset:  true,
	}

	if err := setDefaultRun(opts); err != nil {
		t.Fatalf("setDefaultRun() error = %v", err)
	}
	got, _ := cfg.Get(defaultHost, "default_repo")
	if got != "" {
		t.Fatalf("default_repo = %q, want empty after unset", got)
	}
	if !strings.Contains(errOut.String(), "cleared") {
		t.Fatalf("stderr should say cleared; got: %s", errOut.String())
	}
}

func TestSetDefaultEmptyString(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &SetDefaultOptions{
		IO:         io,
		Config:     func() (config.Config, error) { return testConfig(t), nil },
		Repository: "  ",
		BaseRepo:   func() (string, error) { return "", cmdutil.NewUsageError("no git") },
	}

	err := setDefaultRun(opts)
	if err == nil {
		t.Fatal("setDefaultRun() error = nil, want error for empty repo")
	}
}
