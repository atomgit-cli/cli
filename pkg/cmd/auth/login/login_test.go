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
	t.Parallel()
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdLogin(f, nil)
	for _, source := range []string{cmd.Example, cmd.Long} {
		source := source
		for _, bad := range []string{`echo "`, `echo '`, `printf "`, `printf '`, "token.txt", "cat token", "cat ~/.secret"} {
			bad := bad
			if strings.Contains(source, bad) {
				t.Errorf("help contains unsafe token example %q: %q", bad, source)
			}
		}
	}
}

func TestNewCmdLoginBindsWebAndHostnameFlags(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
