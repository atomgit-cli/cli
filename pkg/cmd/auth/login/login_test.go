package login

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

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
			for _, want := range []string{
				"环境变量 " + tt.wantVar + " 已设置",
				"sed -i 's/^\\(export \\)\\{0,1\\}" + tt.wantVar + "=/# &/' ~/.bashrc",
				"unset " + tt.wantVar,
				"gc auth status",
			} {
				if !strings.Contains(errOut.String(), want) {
					t.Errorf("stderr missing %q, got:\n%s", want, errOut.String())
				}
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
