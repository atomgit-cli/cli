package login

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/envtoken"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdLogin(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "login with with-token flag",
			args:    []string{"--with-token"},
			wantErr: false,
		},
		{
			name:    "login with hostname flag",
			args:    []string{"--hostname", "gitcode.com", "--with-token"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdLogin(f, func(opts *LoginOptions) error {
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

// TestNewCmdLoginHelpAvoidsUnsafeTokenExamples is a regression guard for
// issue #476: the login command help must not demonstrate putting a token
// literal on the command line (echo/printf "..." or '...') or reading it
// from a plaintext file (cat token-file), both of which leak the token to
// shell history/disk. The guard is intentionally narrow (substring-based)
// to avoid false positives on the safety guidance prose ("Never echo/cat a
// token literal", which has no space after echo/cat).
func TestNewCmdLoginHelpAvoidsUnsafeTokenExamples(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdLogin(f, nil)
	for _, source := range []string{cmd.Example, cmd.Long} {
		for _, bad := range []string{`echo "`, `echo '`, `printf "`, `printf '`, "token.txt", "cat token", "cat ~/.secret"} {
			if strings.Contains(source, bad) {
				t.Errorf("help contains unsafe token example %q: %q", bad, source)
			}
		}
	}
}

func TestNewCmdLoginBindsWebAndHostnameFlags(t *testing.T) {
	f := cmdutil.TestFactory()
	var gotWeb bool
	var gotHostname string
	cmd := NewCmdLogin(f, func(opts *LoginOptions) error {
		gotWeb = opts.Web
		gotHostname = opts.Hostname
		return nil
	})
	cmd.SetArgs([]string{"--web", "--hostname", "gitcode.com"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !gotWeb {
		t.Fatal("Web = false, want true")
	}
	if gotHostname != "gitcode.com" {
		t.Fatalf("Hostname = %q, want gitcode.com", gotHostname)
	}
}

func TestNewCmdLoginRejectsWebWithToken(t *testing.T) {
	f := cmdutil.TestFactory()
	f.IOStreams.In = bytes.NewBufferString("test-token\n")
	f.HttpClient = func() (*http.Client, error) {
		t.Fatal("unexpected HTTP client creation")
		return nil, nil
	}
	f.Config = func() (config.Config, error) {
		t.Fatal("unexpected config access")
		return nil, nil
	}

	cmd := NewCmdLogin(f, nil)
	cmd.SetArgs([]string{"--web", "--with-token", "--hostname", "enterprise.example.com"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want mutually exclusive flags error")
	}
	if !strings.Contains(err.Error(), "--web and --with-token cannot be used together") {
		t.Fatalf("Execute() error = %q, want mutually exclusive flags error", err.Error())
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitUsage)
	}
}

func TestNewCmdLoginRejectsWebWithCustomHost(t *testing.T) {
	f := cmdutil.TestFactory()
	f.HttpClient = func() (*http.Client, error) {
		t.Fatal("unexpected HTTP client creation")
		return nil, nil
	}
	f.Config = func() (config.Config, error) {
		t.Fatal("unexpected config access")
		return nil, nil
	}

	cmd := NewCmdLogin(f, nil)
	cmd.SetArgs([]string{"--web", "--hostname", "enterprise.example.com"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want unsupported host")
	}
	if !strings.Contains(err.Error(), "--web only supports gitcode.com") {
		t.Fatalf("Execute() error = %q, want unsupported host", err.Error())
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitUsage)
	}
}

func TestLoginWithWebOpensBrowser(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	io, _, out, _ := iostreams.Test()
	io.In = bytes.NewBufferString("test-token\n")

	var openedURL string
	opts := &LoginOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"login":"tester"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
		OpenBrowser: func(url string) error {
			openedURL = url
			return nil
		},
		Web: true,
	}

	if err := loginWithWeb(opts); err != nil {
		t.Fatalf("loginWithWeb() error = %v", err)
	}

	if openedURL != "https://gitcode.com/setting/token-classic/create" {
		t.Fatalf("opened URL = %q", openedURL)
	}
	if !strings.Contains(out.String(), "Opening https://gitcode.com/setting/token-classic/create in your browser.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLoginWithWebRejectsCustomHost(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &LoginOptions{
		IO:       io,
		Hostname: "enterprise.example.com",
		OpenBrowser: func(url string) error {
			t.Fatalf("unexpected browser open: %s", url)
			return nil
		},
	}

	err := loginWithWeb(opts)
	if err == nil {
		t.Fatal("loginWithWeb() error = nil, want unsupported host")
	}
	if !strings.Contains(err.Error(), "--web only supports gitcode.com") {
		t.Fatalf("loginWithWeb() error = %q, want unsupported host", err.Error())
	}
}

func TestLoginWithTokenRejectsMalformedHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	io, _, _, _ := iostreams.Test()

	opts := &LoginOptions{
		IO:       io,
		Hostname: "https://gitcode.com",
		Token:    "test-token",
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request to %s", req.URL.String())
					return nil, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
	}

	err := loginWithTokenFlag(opts)
	if err == nil {
		t.Fatal("loginWithTokenFlag() error = nil, want invalid host")
	}
	if !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("loginWithTokenFlag() error = %q, want invalid host", err.Error())
	}
}

func TestLoginWithWebRejectsMalformedHost(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := &LoginOptions{
		IO:       io,
		Hostname: "bad/host",
		OpenBrowser: func(url string) error {
			t.Fatalf("unexpected browser open: %s", url)
			return nil
		},
	}

	err := loginWithWeb(opts)
	if err == nil {
		t.Fatal("loginWithWeb() error = nil, want invalid host")
	}
	if !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("loginWithWeb() error = %q, want invalid host", err.Error())
	}
}

func TestNewCmdLoginRejectsInteractiveLoginWithoutTTY(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdLogin(f, nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "use --with-token") {
		t.Fatalf("Execute() error = %q", err.Error())
	}
}

func TestNewCmdLoginRejectsRemovedTokenFlag(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdLogin(f, nil)
	cmd.SetArgs([]string{"--token", "test-token"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want unknown flag error")
	}
	if !strings.Contains(err.Error(), "unknown flag: --token") {
		t.Fatalf("Execute() error = %q", err.Error())
	}
}

func ioNopCloser(body string) *readCloser {
	return &readCloser{Reader: strings.NewReader(body)}
}

type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error { return nil }

// --- issue 613: environment variable override guidance on login success ---

// newLoginTestOpts builds LoginOptions whose token verifies as "tester"
// against a mocked API and whose config is isolated to a temp dir.
func newLoginTestOpts(t *testing.T, io *iostreams.IOStreams, hostname string) *LoginOptions {
	t.Helper()
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	return &LoginOptions{
		IO:       io,
		Hostname: hostname,
		Token:    "test-token",
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"login":"tester"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
	}
}

// storedTokenFor reads the persisted token from the isolated config dir.
func storedTokenFor(t *testing.T, hostname string) string {
	t.Helper()
	cfg := config.New()
	token, _ := cfg.Authentication().StoredToken(hostname)
	return token
}

func TestLoginSuccessWithEnvOverrideWarnsAndSaves(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantVar string
	}{
		{
			name:    "GC_TOKEN set",
			envVars: map[string]string{"GC_TOKEN": "env-override"},
			wantVar: "GC_TOKEN",
		},
		{
			name:    "GITCODE_TOKEN set",
			envVars: map[string]string{"GITCODE_TOKEN": "env-override"},
			wantVar: "GITCODE_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_TOKEN", "")
			t.Setenv("GITCODE_TOKEN", "")
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			t.Setenv("SHELL", "/bin/bash")

			io, _, out, errOut := iostreams.Test()
			opts := newLoginTestOpts(t, io, "gitcode.com")

			if err := loginWithTokenFlag(opts); err != nil {
				t.Fatalf("loginWithTokenFlag() error = %v", err)
			}
			if token := storedTokenFor(t, "gitcode.com"); token != "test-token" {
				t.Fatalf("stored token = %q, want test-token (login must still save)", token)
			}
			if !strings.Contains(out.String(), "Logged in as tester") {
				t.Fatalf("stdout = %q", out.String())
			}
			if strings.Contains(out.String(), "Environment variables still take precedence") {
				t.Fatalf("stdout still contains the old static env hint: %q", out.String())
			}
			// The exact platform rendering is pinned by the envtoken unit
			// tests; here we assert the wiring by comparing against the block
			// rendered for this very runtime environment (shell + GOOS).
			// Stable-string floor: if the wiring ever regresses to an empty
			// EnvVars list, guidance becomes "" and Contains("") is vacuously
			// true, so pin the warning header independently of the block.
			if !strings.Contains(errOut.String(), "环境变量 "+tt.wantVar+" 已设置") {
				t.Errorf("stderr missing warning header for %s, got:\n%s", tt.wantVar, errOut.String())
			}
			guidance := envtoken.Warning(envtoken.Options{
				EnvVars:    []string{tt.wantVar},
				Shell:      os.Getenv("SHELL"),
				GOOS:       runtime.GOOS,
				LoginSaved: true,
			})
			if !strings.Contains(errOut.String(), guidance) {
				t.Errorf("stderr missing platform guidance block:\nwant:\n%s\ngot:\n%s", guidance, errOut.String())
			}
		})
	}
}

func TestLoginSuccessWithoutEnvStaysQuiet(t *testing.T) {
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, out, errOut := iostreams.Test()
	opts := newLoginTestOpts(t, io, "gitcode.com")

	if err := loginWithTokenFlag(opts); err != nil {
		t.Fatalf("loginWithTokenFlag() error = %v", err)
	}
	if !strings.Contains(out.String(), "Logged in as tester") {
		t.Fatalf("stdout = %q", out.String())
	}
	if strings.Contains(out.String(), "Environment variables") {
		t.Fatalf("stdout contains env-related output: %q", out.String())
	}
	if strings.Contains(errOut.String(), "环境变量") || strings.Contains(errOut.String(), "方案") {
		t.Fatalf("stderr contains env guidance: %q", errOut.String())
	}
}

func TestLoginSuccessWithEnvOnCustomHostDoesNotWarn(t *testing.T) {
	t.Setenv("GC_TOKEN", "env-override")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, out, errOut := iostreams.Test()
	opts := newLoginTestOpts(t, io, "enterprise.example.com")

	if err := loginWithTokenFlag(opts); err != nil {
		t.Fatalf("loginWithTokenFlag() error = %v", err)
	}
	if token := storedTokenFor(t, "enterprise.example.com"); token != "test-token" {
		t.Fatalf("stored token = %q, want test-token", token)
	}
	if !strings.Contains(out.String(), "Logged in as tester") {
		t.Fatalf("stdout = %q", out.String())
	}
	if strings.Contains(errOut.String(), "环境变量") || strings.Contains(out.String(), "环境变量") {
		t.Fatalf("custom host login must not warn about env override, stderr:\n%s\nstdout:\n%s", errOut.String(), out.String())
	}
	if strings.Contains(out.String(), "Environment variables still take precedence") {
		t.Fatalf("misleading static env hint must be gone for custom hosts: %q", out.String())
	}
}

func TestLoginInteractiveEnvOverrideGate(t *testing.T) {
	tests := []struct {
		name       string
		answer     string
		wantSaved  bool
		wantLogged bool
	}{
		{name: "decline with n", answer: "n\n", wantSaved: false, wantLogged: false},
		{name: "decline with empty line (default N)", answer: "\n", wantSaved: false, wantLogged: false},
		{name: "decline on EOF", answer: "", wantSaved: false, wantLogged: false},
		{name: "accept with y", answer: "y\n", wantSaved: true, wantLogged: true},
		{name: "accept with yes", answer: "YES\n", wantSaved: true, wantLogged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_TOKEN", "env-override")
			t.Setenv("GITCODE_TOKEN", "")

			io, _, out, errOut := iostreams.TestTTY()
			io.In = bytes.NewBufferString(tt.answer)
			opts := newLoginTestOpts(t, io, "gitcode.com")

			if err := loginWithTokenFlag(opts); err != nil {
				t.Fatalf("loginWithTokenFlag() error = %v", err)
			}
			if !strings.Contains(errOut.String(), "仍要将本次登录保存到本地配置吗") {
				t.Fatalf("stderr missing confirmation gate:\n%s", errOut.String())
			}
			token := storedTokenFor(t, "gitcode.com")
			if tt.wantSaved && token != "test-token" {
				t.Fatalf("stored token = %q, want test-token", token)
			}
			if !tt.wantSaved && token != "" {
				t.Fatalf("stored token = %q, want empty (declined save)", token)
			}
			if tt.wantLogged && !strings.Contains(out.String(), "Logged in as tester") {
				t.Fatalf("stdout = %q", out.String())
			}
			if !tt.wantLogged {
				if strings.Contains(out.String(), "Logged in as tester") {
					t.Fatalf("declined login must not report success, stdout = %q", out.String())
				}
				if !strings.Contains(errOut.String(), "已跳过保存登录配置") {
					t.Fatalf("stderr missing skip notice:\n%s", errOut.String())
				}
			}
			if tt.wantLogged && !strings.Contains(errOut.String(), "方案 A") {
				t.Fatalf("accepted login must print the guidance plans:\n%s", errOut.String())
			}
		})
	}
}

// TestLoginInteractivePasteTokenAndConfirmation guards the shared input
// reader: pasting "token\ny\n" in one shot must reach the confirmation gate
// instead of losing the buffered answer to a second, separate reader.
func TestLoginInteractivePasteTokenAndConfirmation(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantSaved bool
	}{
		{name: "token and y pasted together", input: "test-token\ny\n", wantSaved: true},
		{name: "token and n pasted together", input: "test-token\nn\n", wantSaved: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_TOKEN", "env-override")
			t.Setenv("GITCODE_TOKEN", "")

			io, _, out, errOut := iostreams.TestTTY()
			io.In = bytes.NewBufferString(tt.input)
			opts := newLoginTestOpts(t, io, "")

			if err := loginInteractive(opts); err != nil {
				t.Fatalf("loginInteractive() error = %v", err)
			}
			if !strings.Contains(errOut.String(), "仍要将本次登录保存到本地配置吗") {
				t.Fatalf("stderr missing confirmation gate:\n%s", errOut.String())
			}
			token := storedTokenFor(t, "gitcode.com")
			if tt.wantSaved && token != "test-token" {
				t.Fatalf("stored token = %q, want test-token", token)
			}
			if !tt.wantSaved && token != "" {
				t.Fatalf("stored token = %q, want empty (declined save)", token)
			}
			if !tt.wantSaved && strings.Contains(out.String(), "Logged in as tester") {
				t.Fatalf("declined login must not report success, stdout = %q", out.String())
			}
		})
	}
}

// TestLoginWithTokenSharesConfirmationReader guards the same buffered-reader
// contract when the token itself is read from stdin with --with-token.
func TestLoginWithTokenSharesConfirmationReader(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantSaved bool
	}{
		{name: "token and y pasted together", input: "test-token\ny\n", wantSaved: true},
		{name: "token and n pasted together", input: "test-token\nn\n", wantSaved: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_TOKEN", "env-override")
			t.Setenv("GITCODE_TOKEN", "")

			io, _, _, errOut := iostreams.TestTTY()
			io.In = bytes.NewBufferString(tt.input)
			opts := newLoginTestOpts(t, io, "gitcode.com")

			if err := loginWithToken(opts); err != nil {
				t.Fatalf("loginWithToken() error = %v, want nil", err)
			}
			if !strings.Contains(errOut.String(), "仍要将本次登录保存到本地配置吗") {
				t.Fatalf("stderr missing confirmation gate:\n%s", errOut.String())
			}

			token := storedTokenFor(t, "gitcode.com")
			if tt.wantSaved && token != "test-token" {
				t.Fatalf("stored token = %q, want test-token", token)
			}
			if !tt.wantSaved && token != "" {
				t.Fatalf("stored token = %q, want empty", token)
			}
		})
	}
}

// TestLoginEnvOverrideTokenVerifyFailsBeforeGate covers the "env set + invalid
// token" matrix cell: verification fails first, so the gate never triggers,
// nothing is saved, and no guidance is printed.
func TestLoginEnvOverrideTokenVerifyFailsBeforeGate(t *testing.T) {
	t.Setenv("GC_TOKEN", "env-override")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, _, errOut := iostreams.TestTTY()
	io.In = bytes.NewBufferString("y\n")
	opts := newLoginTestOpts(t, io, "gitcode.com")
	opts.HttpClient = func() (*http.Client, error) {
		return &http.Client{
			Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     make(http.Header),
					Body:       ioNopCloser(`{"message":"unauthorized"}`),
				}, nil
			}),
		}, nil
	}

	err := loginWithTokenFlag(opts)
	if err == nil {
		t.Fatal("loginWithTokenFlag() error = nil, want verify failure")
	}
	if !strings.Contains(err.Error(), "failed to verify token") {
		t.Fatalf("error = %q, want verify failure", err.Error())
	}
	if strings.Contains(errOut.String(), "仍要将本次登录保存到本地配置吗") {
		t.Fatalf("confirmation gate must not trigger on verify failure, stderr:\n%s", errOut.String())
	}
	if strings.Contains(errOut.String(), "方案") {
		t.Fatalf("no guidance may be printed on verify failure, stderr:\n%s", errOut.String())
	}
	if token := storedTokenFor(t, "gitcode.com"); token != "" {
		t.Fatalf("stored token = %q, want empty (nothing saved)", token)
	}
}

// erroringReader simulates a broken stdin for the confirmation gate.
type erroringReader struct{}

func (erroringReader) Read(p []byte) (int, error) { return 0, errors.New("boom") }

// TestLoginEnvOverrideGateReadErrorPropagates covers the gate's non-EOF read
// error path: it must surface as a usage CLIError without saving anything.
func TestLoginEnvOverrideGateReadErrorPropagates(t *testing.T) {
	t.Setenv("GC_TOKEN", "env-override")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, _, _ := iostreams.TestTTY()
	io.In = erroringReader{}
	opts := newLoginTestOpts(t, io, "gitcode.com")

	err := loginWithTokenFlag(opts)
	if err == nil {
		t.Fatal("loginWithTokenFlag() error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "failed to read confirmation") {
		t.Fatalf("error = %q, want read confirmation failure", err.Error())
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitUsage)
	}
	if token := storedTokenFor(t, "gitcode.com"); token != "" {
		t.Fatalf("stored token = %q, want empty (nothing saved)", token)
	}
}
