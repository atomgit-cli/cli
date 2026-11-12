package gitcredential

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

const (
	testHost     = "gitcode.com"
	testUser     = "testuser"
	testToken    = "test-token-do-not-use"
	secretMarker = "SECRET-TOKEN-VALUE"
)

// newLoggedInConfig creates a config with a stored login for testHost and
// returns the Config. GC_CONFIG_DIR must be set before calling config.New().
func newLoggedInConfig(t *testing.T, user, token string) config.Config {
	t.Helper()
	cfg := config.New()
	authCfg := cfg.Authentication()
	if _, err := authCfg.Login(testHost, user, token, "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return cfg
}

// newTestIO returns IOStreams with the given stdin content and its buffers.
func newTestIO(input string) (*iostreams.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	io, _, out, errOut := iostreams.Test()
	io.In = strings.NewReader(input)
	return io, out, errOut
}

// assertSilentNoCredential verifies the no-credential contract: silent
// non-zero exit with nothing written to stdout or stderr.
func assertSilentNoCredential(t *testing.T, err error, out, errOut *bytes.Buffer) {
	t.Helper()
	if err == nil {
		t.Fatal("gitCredentialRun() error = nil, want silent no-credential error")
	}
	if !cmdutil.IsSilent(err) {
		t.Fatalf("error %v is not silent; stderr noise breaks the helper protocol", err)
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitError)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on no-credential", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on no-credential (silent exit)", errOut.String())
	}
}

func TestNewCmdGitCredential(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdGitCredential(f, nil)
	if cmd == nil {
		t.Fatal("NewCmdGitCredential returned nil")
	}
	if cmd.Use != "git-credential [operation]" {
		t.Errorf("Use = %q, want git-credential [operation]", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}
}

func TestGitCredentialCmdAcceptsNoArgsAndOneArg(t *testing.T) {
	f := cmdutil.TestFactory()

	for _, args := range [][]string{{}, {"get"}, {"store"}, {"erase"}} {
		cmd := NewCmdGitCredential(f, func(opts *GitCredentialOptions) error {
			if len(args) > 0 && opts.Operation != args[0] {
				t.Errorf("Operation = %q, want %q", opts.Operation, args[0])
			}
			return nil
		})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
	}
}

func TestGitCredentialCmdRejectsTwoArgs(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdGitCredential(f, func(opts *GitCredentialOptions) error { return nil })
	// Standalone subcommand execution does not inherit the root's
	// SilenceErrors/SilenceUsage; set them so cobra error/usage output does
	// not leak to the real stderr as test noise.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"get", "extra"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() with 2 args = nil error, want usage error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestGitCredentialRunFillsStoredCredentials(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")

	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}

	want := "username=testuser\npassword=test-token-do-not-use\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty (credentials only on stdout)", errOut.String())
	}
}

func TestGitCredentialRunFillsWithoutTrailingBlankLine(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("protocol=https\nhost=gitcode.com\n")

	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username=testuser") || !strings.Contains(out.String(), "password="+testToken) {
		t.Fatalf("stdout = %q, want filled credentials", out.String())
	}
}

func TestGitCredentialRunEmptyConfigDirReturnsNoCredential(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunEnvTokenNotServed(t *testing.T) {
	// Credentials come from the stored login only: environment token
	// overrides must not leak into the helper, so that GC_CONFIG_DIR
	// isolation for wrapper/multi-account setups is deterministic.
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "env-token-should-not-be-served")
	t.Setenv("GITCODE_TOKEN", "")

	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunUnmatchedHostSilent(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, errOut := newTestIO("protocol=https\nhost=other.example.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunHostLookupCaseInsensitive(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("protocol=https\nhost=GITCODE.COM\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username="+testUser) {
		t.Fatalf("stdout = %q, want filled credentials for upper-case host", out.String())
	}
}

func TestGitCredentialRunUsernameMismatchSilent(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\nusername=someoneelse\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunUsernameMatchedFills(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("protocol=https\nhost=gitcode.com\nusername=testuser\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username=testuser\n") || !strings.Contains(out.String(), "password="+testToken) {
		t.Fatalf("stdout = %q, want filled credentials", out.String())
	}
}

func TestGitCredentialRunURLKeyParsesHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("url=https://gitcode.com/owner/repo.git\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username="+testUser) {
		t.Fatalf("stdout = %q, want credentials filled from url= key", out.String())
	}
}

func TestGitCredentialRunInvalidURLIgnored(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, errOut := newTestIO("url=://not-a-url\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunNoHostSilent(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	io, out, errOut := newTestIO("protocol=https\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunEmptyStdinSilent(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	io, out, errOut := newTestIO("")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunStoreOperationNoOp(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\nusername=testuser\npassword=whatever\n\n")
	opts := &GitCredentialOptions{
		IO:        io,
		Config:    func() (config.Config, error) { return cfg, nil },
		Operation: "store",
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun(store) error = %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("store op output stdout=%q stderr=%q, want none", out.String(), errOut.String())
	}
}

func TestGitCredentialRunEraseOperationNoOp(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\nusername=testuser\npassword=whatever\n\n")
	opts := &GitCredentialOptions{
		IO:        io,
		Config:    func() (config.Config, error) { return cfg, nil },
		Operation: "erase",
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun(erase) error = %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("erase op output stdout=%q stderr=%q, want none", out.String(), errOut.String())
	}
}

func TestGitCredentialRunUnsupportedOperationUsageError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())

	io, out, _ := newTestIO("")
	opts := &GitCredentialOptions{
		IO:        io,
		Config:    func() (config.Config, error) { return config.New(), nil },
		Operation: "bogus",
	}
	err := gitCredentialRun(opts)
	if err == nil {
		t.Fatal("gitCredentialRun(bogus) error = nil, want usage error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
	if !strings.Contains(err.Error(), "bogus") || !strings.Contains(err.Error(), "operation") {
		t.Fatalf("error = %q, want unsupported operation message", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestGitCredentialRunConfigErrorReturnsError(t *testing.T) {
	io, out, _ := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO: io,
		Config: func() (config.Config, error) {
			return nil, os.ErrInvalid
		},
	}
	err := gitCredentialRun(opts)
	if err == nil {
		t.Fatal("gitCredentialRun() error = nil, want config error")
	}
	if !strings.Contains(err.Error(), "failed to read config") {
		t.Fatalf("error = %q, want failed to read config", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestGitCredentialRunCorruptAuthStateSilentAndNoSecretLeak(t *testing.T) {
	// Red-team path: a corrupt auth.json must fail silently and must never
	// echo file content (which could contain a token) into any stream.
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	garbage := `{"version":1,"hosts":{"gitcode.com":{"active_user":"u","users":{"u":{"token":"` + secretMarker + `"}}}` + ",BROKEN"
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(garbage), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	err := gitCredentialRun(opts)

	// Silent no-credential is the contract for a corrupt auth state; and
	// whatever is returned must never carry the secret from the file.
	assertSilentNoCredential(t, err, out, errOut)
	if err != nil {
		if got := err.Error(); strings.Contains(got, secretMarker) {
			t.Fatalf("error leaks secret: %q", got)
		}
	}
}

func TestGitCredentialRunIgnoresUnknownKeys(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("capability[]=authtype\nprotocol=https\nhost=gitcode.com\npath=/owner/repo.git\nwwwauth[]=Basic realm=x\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username="+testUser) {
		t.Fatalf("stdout = %q, want filled credentials despite unknown keys", out.String())
	}
}

func TestGitCredentialRunRejectsNewlineInStoredUsername(t *testing.T) {
	// A stored username containing a newline would inject extra protocol
	// lines into the git credential stream; it must be rejected silently.
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, "bad\nuser", testToken)
	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunRejectsNewlineInStoredToken(t *testing.T) {
	// A stored token containing control characters must never reach the
	// protocol output.
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, "bad\ntoken")
	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunOperationCaseInsensitive(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:        io,
		Config:    func() (config.Config, error) { return cfg, nil },
		Operation: "GET",
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun(GET) error = %v", err)
	}
	if !strings.Contains(out.String(), "username="+testUser) {
		t.Fatalf("stdout = %q, want filled credentials for upper-case operation", out.String())
	}
}

func TestGitCredentialRunHandlesCRLFLines(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("protocol=https\r\nhost=gitcode.com\r\n\r\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username="+testUser) {
		t.Fatalf("stdout = %q, want filled credentials for CRLF request", out.String())
	}
}

func TestGitCredentialRunEmptyGCConfigDirValueFailsClosed(t *testing.T) {
	// An explicitly empty GC_CONFIG_DIR is "no configuration" for the
	// helper: it must not fall back to the default directory, so a wrapper
	// whose variable expands to an empty string never silently serves the
	// primary account's credentials.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GC_CONFIG_DIR", "")
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	// Seed a login in the HOME default config directory.
	authCfg := config.New().Authentication()
	if _, err := authCfg.Login(testHost, testUser, testToken, "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	io, out, errOut := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	assertSilentNoCredential(t, gitCredentialRun(opts), out, errOut)
}

func TestGitCredentialRunUnsetGCConfigDirUsesHomeDefault(t *testing.T) {
	// Control case for the empty-value test: with GC_CONFIG_DIR unset, the
	// HOME default directory is served as usual.
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Unsetenv("GC_CONFIG_DIR")
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	authCfg := config.New().Authentication()
	if _, err := authCfg.Login(testHost, testUser, testToken, "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	io, out, _ := newTestIO("protocol=https\nhost=gitcode.com\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return config.New(), nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "username="+testUser) {
		t.Fatalf("stdout = %q, want filled credentials from HOME default config", out.String())
	}
}

// TestGitCredentialRunStoresNoExtraKeys verifies the protocol output contains
// only username and password lines.
func TestGitCredentialRunOutputsOnlyUsernameAndPassword(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := newLoggedInConfig(t, testUser, testToken)
	io, out, _ := newTestIO("protocol=https\nhost=gitcode.com\npath=/owner/repo.git\n\n")
	opts := &GitCredentialOptions{
		IO:     io,
		Config: func() (config.Config, error) { return cfg, nil },
	}
	if err := gitCredentialRun(opts); err != nil {
		t.Fatalf("gitCredentialRun() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("output lines = %d (%q), want exactly 2", len(lines), out.String())
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "username=") && !strings.HasPrefix(line, "password=") {
			t.Fatalf("unexpected protocol line %q", line)
		}
	}
}

// flipAuthConfig simulates auth.json being rewritten between reads: every
// method call returns the next snapshot, alternating between two accounts.
// It counts calls so tests can pin the single-read contract (issue 649).
type flipAuthConfig struct {
	config.AuthConfig
	reads       int
	credentialC int
	activeUserC int
	storedTokC  int
}

func (f *flipAuthConfig) snap() (string, string) {
	if f.reads%2 == 1 {
		return "alice", "tokenA"
	}
	return "bob", "tokenB"
}

func (f *flipAuthConfig) StoredCredential(hostname string) (string, string) {
	f.reads++
	f.credentialC++
	return f.snap()
}

func (f *flipAuthConfig) ActiveUser(hostname string) (string, error) {
	f.reads++
	f.activeUserC++
	user, _ := f.snap()
	return user, nil
}

func (f *flipAuthConfig) StoredToken(hostname string) (string, string) {
	f.reads++
	f.storedTokC++
	_, token := f.snap()
	return token, "config"
}

type fakeCredentialConfig struct {
	config.Config
	auth config.AuthConfig
}

func (c *fakeCredentialConfig) Authentication() config.AuthConfig { return c.auth }

func TestGetCredentialSingleReadConsistentPair(t *testing.T) {
	// The helper must serve username and token from ONE auth.json snapshot:
	// with the old ActiveUser+StoredToken pair, a state rewrite between the
	// two reads served a mismatched pair (alice + tokenB); the single-read
	// StoredCredential API makes that impossible (issue 649).
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	fake := &flipAuthConfig{}
	io, _, out, _ := iostreams.Test()
	io.In = strings.NewReader("protocol=https\nhost=" + testHost + "\n\n")
	err := getCredential(&GitCredentialOptions{
		IO: io,
		Config: func() (config.Config, error) {
			return &fakeCredentialConfig{auth: fake}, nil
		},
	})
	if err != nil {
		t.Fatalf("getCredential() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "username=alice\n") || !strings.Contains(got, "password=tokenA\n") {
		t.Fatalf("served pair = %q, want the alice/tokenA snapshot (consistent pair)", got)
	}
	if fake.credentialC != 1 {
		t.Fatalf("StoredCredential calls = %d, want exactly 1", fake.credentialC)
	}
	if fake.activeUserC != 0 || fake.storedTokC != 0 {
		t.Fatalf("legacy reads used: ActiveUser=%d StoredToken=%d, want 0/0 (single-read contract)", fake.activeUserC, fake.storedTokC)
	}
}
