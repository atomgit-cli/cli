package switchcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func loginTwoAccounts(t *testing.T) config.Config {
	t.Helper()
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "")
	cfg := config.New()
	auth := cfg.Authentication()
	if _, err := auth.Login("gitcode.com", "alice", "tokenA", "https", false); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Login("gitcode.com", "bob", "tokenB", "https", false); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestNewCmdSwitchAcceptsZeroOrOneArg(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdSwitch(f, nil)
	if cmd.Use != "switch [<username>]" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	if cmd.Flags().Lookup("hostname") == nil || cmd.Flags().Lookup("json") == nil {
		t.Fatal("hostname/json flags not registered")
	}
	if err := cobra.MaximumNArgs(1)(cmd, []string{"a", "b"}); err == nil {
		t.Fatal("two args should be rejected")
	}
}

func TestSwitchRunSwitchesActiveUser(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, out, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "bob", Hostname: "gitcode.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Switched to bob on gitcode.com") {
		t.Fatalf("output = %q", out.String())
	}
	active, err := cfg.Authentication().ActiveUser("gitcode.com")
	if err != nil || active != "bob" {
		t.Fatalf("ActiveUser() = %q (%v), want bob", active, err)
	}
}

func TestSwitchRunUnknownUserExit3WithAvailableList(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, _, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "carol", Hostname: "gitcode.com"})
	if err == nil {
		t.Fatal("switchRun() error = nil, want not found")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitNotFound)
	}
	for _, want := range []string{"carol", "available", "alice", "bob"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q mentioned", err.Error(), want)
		}
	}
}

func TestSwitchRunNoAccountsExit3WithLoginHint(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	io, _, _, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "alice", Hostname: "gitcode.com"})
	if err == nil {
		t.Fatal("switchRun() error = nil, want not found")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitNotFound)
	}
	for _, want := range []string{"no stored accounts", "gc auth login"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q mentioned", err.Error(), want)
		}
	}
}

func TestSwitchRunNonInteractiveWithoutUsernameExit2(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, _, _ := iostreams.Test() // not a TTY
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com"})
	if err == nil {
		t.Fatal("switchRun() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
	if !strings.Contains(err.Error(), "TTY") {
		t.Fatalf("error = %q, want TTY mention", err.Error())
	}
}

func TestSwitchRunInteractiveSelectionByNumber(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, _, errOut := iostreams.TestTTY()
	io.In = strings.NewReader("1\n") // alice is NOT the active user (bob is) — a real switch
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	rendered := errOut.String()
	for _, want := range []string{"Switch account on gitcode.com", "1. alice", "2. bob (current)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("prompt output missing %q: %q", want, rendered)
		}
	}
	active, _ := cfg.Authentication().ActiveUser("gitcode.com")
	if active != "alice" {
		t.Fatalf("ActiveUser() = %q, want alice (selected by number)", active)
	}
}

func TestSwitchRunInteractiveSelectionByName(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, _, _ := iostreams.TestTTY()
	io.In = strings.NewReader("alice\n")
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	active, _ := cfg.Authentication().ActiveUser("gitcode.com")
	if active != "alice" {
		t.Fatalf("ActiveUser() = %q, want alice (selected by name)", active)
	}
}

func TestSwitchRunInteractiveInvalidSelectionExit2(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, _, _ := iostreams.TestTTY()
	io.In = strings.NewReader("zzz\n")
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com"})
	if err == nil {
		t.Fatal("switchRun() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestSwitchRunJSONOutput(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, out, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "bob", Hostname: "gitcode.com", JSON: true})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{`"hostname": "gitcode.com"`, `"active_user": "bob"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("json output missing %q: %q", want, got)
		}
	}
	if strings.Index(got, `"hostname"`) > strings.Index(got, `"active_user"`) {
		t.Fatalf("json field order wrong: %q", got)
	}
}

func TestSwitchRunResolvesDefaultHost(t *testing.T) {
	t.Setenv("GC_HOST", "")
	cfg := loginTwoAccounts(t) // DefaultHost becomes gitcode.com
	io, _, out, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "bob"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "gitcode.com") {
		t.Fatalf("output should mention the resolved host: %q", out.String())
	}
}

func TestSwitchRunInteractivePromptsDoNotPolluteJSONStdout(t *testing.T) {
	// --json + TTY + no username: the prompt block goes to stderr so the
	// JSON output stays machine-readable (issue 646 review).
	cfg := loginTwoAccounts(t)
	io, _, out, _ := iostreams.TestTTY()
	io.In = strings.NewReader("bob\n")
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com", JSON: true})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	got := out.String()
	if strings.TrimSpace(got) != `{
  "hostname": "gitcode.com",
  "active_user": "bob"
}` {
		t.Fatalf("stdout polluted by prompt text: %q", got)
	}
}

func TestSwitchRunRejectsControlCharAccountNames(t *testing.T) {
	// A corrupted auth.json must not push terminal escape sequences
	// through the account listing or hints.
	// The fixture uses the JSON escape \u001b so the username parses to a
	// real ESC byte: the guard must fire before any echo.
	corrupt := `{"version":1,"default_host":"gitcode.com","hosts":{"gitcode.com":{"active_user":"a\u001bb","users":{"a\u001bb":{"token":"t"},"ok":{"token":"t2"}}}}}`
	tmp := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", tmp)
	if err := os.WriteFile(filepath.Join(tmp, "auth.json"), []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.New()
	io, _, _, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "ok", Hostname: "gitcode.com"})
	if err == nil {
		t.Fatal("switchRun() error = nil, want corrupted-state error")
	}
	if !strings.Contains(err.Error(), "control characters") {
		t.Fatalf("error = %q, want control-characters mention", err.Error())
	}
}

func TestNewCmdSwitchBindsUsernameArg(t *testing.T) {
	// RunE must wire args[0] into opts.Username (Execute-level pin).
	f := cmdutil.TestFactory()
	var gotUser string
	cmd := NewCmdSwitch(f, func(opts *SwitchOptions) error {
		gotUser = opts.Username
		return nil
	})
	cmd.SetArgs([]string{"bob"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotUser != "bob" {
		t.Fatalf("Username = %q, want bob", gotUser)
	}
}

func TestSwitchRunInvalidHostnameFails(t *testing.T) {
	cfg := loginTwoAccounts(t)
	io, _, _, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "bob", Hostname: "bad/host"})
	if err == nil || !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("error = %v, want invalid host", err)
	}
}

func TestSwitchRunToCurrentUserIsIdempotent(t *testing.T) {
	cfg := loginTwoAccounts(t) // active = bob
	io, _, out, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "bob", Hostname: "gitcode.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Switched to bob") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestSwitchRunNoAccountsTakesPrecedenceOverTTYRequirement(t *testing.T) {
	// The empty-host check runs before the TTY check: no stored accounts
	// exits 3 even without a username in a non-interactive context (more
	// diagnostic than the generic usage error; documented in COMMANDS.md).
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	io, _, _, _ := iostreams.Test() // not a TTY
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com"})
	if err == nil {
		t.Fatal("switchRun() error = nil, want not found")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode() = %d, want %d (no-accounts takes precedence over the TTY usage error)", cmdutil.ExitCode(err), cmdutil.ExitNotFound)
	}
}

func TestSwitchRunDoesNotPromoteDefaultHost(t *testing.T) {
	// Switching on a non-default host is scoped: it must not move the
	// default host that bare commands target (UX review S1).
	t.Setenv("GC_HOST", "")
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	cfg := config.New()
	auth := cfg.Authentication()
	// Login the non-default host FIRST so the last login (gitcode.com)
	// leaves DefaultHost there; the switch must not move it.
	if _, err := auth.Login("enterprise.example.com", "svc-bot", "tokenS", "https", false); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Login("gitcode.com", "alice", "tokenA", "https", false); err != nil {
		t.Fatal(err)
	}
	io, _, _, _ := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "svc-bot", Hostname: "enterprise.example.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	if host, _ := cfg.Authentication().DefaultHost(); host != "gitcode.com" {
		t.Fatalf("DefaultHost() = %q after a scoped switch, want gitcode.com unchanged", host)
	}
}

func TestSwitchRunWarnsWhenEnvTokenOverrides(t *testing.T) {
	// A switch under GC_TOKEN succeeds but business commands keep using
	// the environment token; the warning mirrors the login/logout family.
	cfg := loginTwoAccounts(t)
	t.Setenv("GC_TOKEN", "env-token-do-not-use")
	io, _, _, errOut := iostreams.Test()
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Username: "bob", Hostname: "gitcode.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "GC_TOKEN") || !strings.Contains(errOut.String(), "environment token") {
		t.Fatalf("stderr warning missing: %q", errOut.String())
	}
}
