package switchcmd

import (
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
	io, _, out, _ := iostreams.TestTTY()
	io.In = strings.NewReader("2\n")
	err := switchRun(&SwitchOptions{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Hostname: "gitcode.com"})
	if err != nil {
		t.Fatalf("switchRun() error = %v", err)
	}
	rendered := out.String()
	for _, want := range []string{"Switch account on gitcode.com", "1. alice", "2. bob", "(current)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("prompt output missing %q: %q", want, rendered)
		}
	}
	active, _ := cfg.Authentication().ActiveUser("gitcode.com")
	if active != "bob" {
		t.Fatalf("ActiveUser() = %q, want bob (selected by number)", active)
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
