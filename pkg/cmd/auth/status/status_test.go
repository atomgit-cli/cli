package status

import (
	"errors"
	"fmt"
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

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("write failed") }

func TestNewCmdStatus(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdStatus(f, nil)
	if cmd == nil {
		t.Fatal("NewCmdStatus returned nil")
	}
	if cmd.Use != "status" {
		t.Errorf("Expected Use 'status', got %q", cmd.Use)
	}
	if strings.Contains(cmd.Long, "keyring") {
		t.Fatalf("status help unexpectedly references keyring: %q", cmd.Long)
	}
	if !strings.Contains(cmd.Long, "auth.json") {
		t.Fatalf("status help should reference local config storage: %q", cmd.Long)
	}
}

func TestStatusRunUsesStoredToken(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("gitcode.com", "tester", "stored-token", "ssh", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &StatusOptions{
		IO: f.IOStreams,
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

	if err := statusRun(opts); err != nil {
		t.Fatalf("statusRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Logged in as tester (config)") {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "Git operations protocol: ssh") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestStatusRunWithHostnameUsesStoredTokenInsteadOfEnvOverride(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("other.example.com", "stored-user", "stored-token", "ssh", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &StatusOptions{
		IO:          f.IOStreams,
		Hostname:    "other.example.com",
		HostnameSet: true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					auth := req.Header.Get("Authorization")
					if auth != "Bearer stored-token" {
						t.Fatalf("Authorization = %q, want %q", auth, "Bearer stored-token")
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"login":"stored-user"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
	}

	if err := statusRun(opts); err != nil {
		t.Fatalf("statusRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Logged in as stored-user (config)") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestStatusRunDefaultCustomHostUsesStoredTokenInsteadOfEnvOverride(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "other.example.com")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("other.example.com", "stored-user", "stored-token", "ssh", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &StatusOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Host != "api.other.example.com" {
						t.Fatalf("request host = %q, want api.other.example.com", req.URL.Host)
					}
					auth := req.Header.Get("Authorization")
					if auth != "Bearer stored-token" {
						t.Fatalf("Authorization = %q, want %q", auth, "Bearer stored-token")
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"login":"stored-user"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
	}

	if err := statusRun(opts); err != nil {
		t.Fatalf("statusRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Logged in as stored-user (config)") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestStatusRunRejectsMalformedDefaultHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "https://gitcode.com")
	t.Setenv("GC_TOKEN", "env-token")

	f := cmdutil.TestFactory()
	opts := &StatusOptions{
		IO: f.IOStreams,
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

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want invalid host error")
	}
	if !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("statusRun() error = %q, want invalid host", err.Error())
	}
}

func TestStatusRunShowTokenDisplaysFullToken(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "secret-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	io, in, _, _ := iostreams.TestTTY()
	f.IOStreams = io
	in.WriteString("gitcode.com\n")
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &StatusOptions{
		IO:        f.IOStreams,
		ShowToken: true,
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

	if err := statusRun(opts); err != nil {
		t.Fatalf("statusRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Token: secret-token") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestStatusRunShowTokenRequiresConfirmationInNonInteractiveMode(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "secret-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	f.IOStreams.Out = out
	f.IOStreams.ErrOut = errOut

	opts := &StatusOptions{
		IO:        f.IOStreams,
		ShowToken: true,
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

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want confirmation error")
	}
	if !strings.Contains(err.Error(), "interactive confirmation") {
		t.Fatalf("statusRun() error = %q, want interactive confirmation hint", err.Error())
	}
	if strings.Contains(out.String(), "secret-token") || strings.Contains(errOut.String(), "secret-token") {
		t.Fatalf("token leaked without confirmation; stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func ioNopCloser(body string) *readCloser {
	return &readCloser{Reader: strings.NewReader(body)}
}

type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error { return nil }

func TestStatusRunNotLoggedInReturnsAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	f.IOStreams.Out = out
	f.IOStreams.ErrOut = errOut

	opts := &StatusOptions{
		IO:         f.IOStreams,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Config:     func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d (ExitAuth)", got, cmdutil.ExitAuth)
	}
	if !strings.Contains(errOut.String(), "Not logged in") {
		t.Fatalf("stderr = %q, want 'Not logged in'", errOut.String())
	}
	if strings.Contains(out.String(), "Not logged in") {
		t.Fatalf("stdout leaked diagnostic: %q", out.String())
	}
}

func TestStatusRunInvalidTokenReturnsAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "bad-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	f.IOStreams.Out = out
	f.IOStreams.ErrOut = errOut

	opts := &StatusOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"invalid token"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d (ExitAuth)", got, cmdutil.ExitAuth)
	}
	if !strings.Contains(errOut.String(), "invalid or expired") {
		t.Fatalf("stderr = %q, want 'invalid or expired'", errOut.String())
	}
	if strings.Contains(out.String(), "invalid or expired") {
		t.Fatalf("stdout leaked diagnostic: %q", out.String())
	}
}

func TestStatusRunServerErrorReturnsGenericErrorNotAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	f.IOStreams.Out = out
	f.IOStreams.ErrOut = errOut

	opts := &StatusOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"internal server error"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d (ExitError) for 500, not ExitAuth", got, cmdutil.ExitError)
	}
	if !strings.Contains(errOut.String(), "Token verification failed") {
		t.Fatalf("stderr = %q, want 'Token verification failed'", errOut.String())
	}
}

func TestStatusRunNetworkErrorReturnsGenericErrorNotAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	f.IOStreams.Out = out
	f.IOStreams.ErrOut = errOut

	opts := &StatusOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("dial tcp: connection refused")
				}),
			}, nil
		},
		Config: func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d (ExitError) for network error, not ExitAuth", got, cmdutil.ExitError)
	}
	if strings.Contains(errOut.String(), "invalid or expired") {
		t.Fatalf("stderr = %q, should not say 'invalid or expired' for network error", errOut.String())
	}
}

func TestStatusRunNotLoggedInJSONReturnsAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &StatusOptions{
		IO:         f.IOStreams,
		JSON:       true,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Config:     func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d (ExitAuth)", got, cmdutil.ExitAuth)
	}
	if !strings.Contains(out.String(), `"logged_in": false`) {
		t.Fatalf("stdout = %q, want logged_in: false JSON", out.String())
	}
	if !strings.Contains(out.String(), `"token_valid": false`) {
		t.Fatalf("stdout = %q, want token_valid: false JSON (state fields must serialize false)", out.String())
	}
}

func TestStatusRunInvalidTokenJSONReturnsAuthError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "bad-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &StatusOptions{
		IO:   f.IOStreams,
		JSON: true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"invalid token"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d (ExitAuth)", got, cmdutil.ExitAuth)
	}
	// token exists but is invalid: LoggedIn stays true and TokenValid must
	// serialize as false - state booleans carry information in both values.
	if !strings.Contains(out.String(), `"logged_in": true`) {
		t.Fatalf("stdout = %q, want logged_in: true JSON", out.String())
	}
	if !strings.Contains(out.String(), `"token_valid": false`) {
		t.Fatalf("stdout = %q, want token_valid: false JSON", out.String())
	}
}

func TestStatusRunNotLoggedInJSONWriteFailureReturnsError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	f.IOStreams.Out = errWriter{}

	opts := &StatusOptions{
		IO:         f.IOStreams,
		JSON:       true,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Config:     func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want write error")
	}
	if !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("error = %q, want 'write failed'", err.Error())
	}
}

func TestStatusRunInvalidTokenJSONWriteFailureReturnsError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "bad-token")
	t.Setenv("GITCODE_TOKEN", "")

	f := cmdutil.TestFactory()
	f.IOStreams.Out = errWriter{}

	opts := &StatusOptions{
		IO:   f.IOStreams,
		JSON: true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusUnauthorized,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"invalid token"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) { return config.New(), nil },
	}

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want write error")
	}
	if !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("error = %q, want 'write failed'", err.Error())
	}
}

// --- issue 613: environment variable override guidance on auth failure ---

// newStatusTestOpts builds StatusOptions with a mocked verification endpoint.
func newStatusTestOpts(t *testing.T, io *iostreams.IOStreams, statusCode int) *StatusOptions {
	t.Helper()
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	return &StatusOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: statusCode,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"unauthorized"}`),
					}, nil
				}),
			}, nil
		},
		Config: func() (config.Config, error) {
			return config.New(), nil
		},
	}
}

func TestStatusAuthFailureWithEnvSourceShowsOverrideGuidance(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantVar string
	}{
		{
			name:    "GC_TOKEN source",
			envVars: map[string]string{"GC_TOKEN": "env-token"},
			wantVar: "GC_TOKEN",
		},
		{
			name:    "GITCODE_TOKEN source",
			envVars: map[string]string{"GITCODE_TOKEN": "env-token"},
			wantVar: "GITCODE_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_HOST", "")
			t.Setenv("GC_TOKEN", "")
			t.Setenv("GITCODE_TOKEN", "")
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			t.Setenv("SHELL", "/bin/bash")

			io, _, out, errOut := iostreams.Test()
			opts := newStatusTestOpts(t, io, 401)

			err := statusRun(opts)
			if err == nil {
				t.Fatal("statusRun() error = nil, want auth error")
			}
			if code := cmdutil.ExitCode(err); code != cmdutil.ExitAuth {
				t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitAuth)
			}
			for _, want := range []string{
				"Token is invalid or expired",
				"仅重新登录不会生效",
				"gc auth login",
				"gc auth status",
			} {
				if !strings.Contains(errOut.String(), want) {
					t.Errorf("stderr missing %q, got:\n%s", want, errOut.String())
				}
			}
			// The exact platform rendering is pinned by the envtoken unit
			// tests; here we assert the wiring by comparing against the block
			// rendered for this very runtime environment (shell + GOOS).
			guidance := envtoken.Warning(envtoken.Options{
				EnvVars:    []string{tt.wantVar},
				Shell:      os.Getenv("SHELL"),
				GOOS:       runtime.GOOS,
				LoginSaved: false,
			})
			if !strings.Contains(errOut.String(), guidance) {
				t.Errorf("stderr missing platform guidance block:\nwant:\n%s\ngot:\n%s", guidance, errOut.String())
			}
			// The guidance must never embed the token value itself.
			if strings.Contains(errOut.String(), "env-token") {
				t.Errorf("stderr must not contain the environment token value")
			}
			if strings.Contains(errOut.String(), "To re-authenticate, run: gc auth login") {
				t.Errorf("stderr must not suggest plain re-login for env source:\n%s", errOut.String())
			}
			if out.String() != "" {
				t.Errorf("failure diagnostics must stay off stdout, got %q", out.String())
			}
		})
	}
}

func TestStatusAuthFailureWithConfigSourceKeepsReLoginHint(t *testing.T) {
	t.Setenv("GC_HOST", "")
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	// Isolate the config dir before any config.New() so the stored login the
	// test creates never leaks into the real user configuration.
	io, _, _, errOut := iostreams.Test()
	opts := newStatusTestOpts(t, io, 401)

	cfg := config.New()
	if _, err := cfg.Authentication().Login("gitcode.com", "tester", "stored-token", "ssh", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := statusRun(opts); err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if !strings.Contains(errOut.String(), "To re-authenticate, run: gc auth login") {
		t.Fatalf("config source must keep the re-login hint, stderr:\n%s", errOut.String())
	}
	if strings.Contains(errOut.String(), "方案") {
		t.Fatalf("config source must not print env guidance plans, stderr:\n%s", errOut.String())
	}
}

func TestStatusNetworkFailureWithEnvSourceHasNoGuidance(t *testing.T) {
	t.Setenv("GC_HOST", "")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, _, errOut := iostreams.Test()
	opts := newStatusTestOpts(t, io, 500)

	if err := statusRun(opts); err == nil {
		t.Fatal("statusRun() error = nil, want verification error")
	}
	if !strings.Contains(errOut.String(), "Token verification failed") {
		t.Fatalf("stderr = %q", errOut.String())
	}
	if strings.Contains(errOut.String(), "方案") || strings.Contains(errOut.String(), "环境变量") {
		t.Fatalf("non-auth failures must not print env guidance, stderr:\n%s", errOut.String())
	}
}

func TestStatusAuthFailureWithHostnameUsesStoredTokenGuidanceUnchanged(t *testing.T) {
	t.Setenv("GC_HOST", "")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	// Isolate the config dir before any config.New() so the stored login the
	// test creates never leaks into the real user configuration.
	io, _, _, errOut := iostreams.Test()
	opts := newStatusTestOpts(t, io, 401)
	opts.Hostname = "enterprise.example.com"
	opts.HostnameSet = true

	cfg := config.New()
	if _, err := cfg.Authentication().Login("enterprise.example.com", "tester", "stored-token", "ssh", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := statusRun(opts); err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if !strings.Contains(errOut.String(), "To re-authenticate, run: gc auth login") {
		t.Fatalf("explicit hostname reads stored token; guidance must stay unchanged, stderr:\n%s", errOut.String())
	}
}

func TestStatusAuthFailureJSONModeWithEnvSourceStaysPureJSON(t *testing.T) {
	t.Setenv("GC_HOST", "")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	io, _, out, errOut := iostreams.Test()
	opts := newStatusTestOpts(t, io, 401)
	opts.JSON = true

	err := statusRun(opts)
	if err == nil {
		t.Fatal("statusRun() error = nil, want auth error")
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d", code, cmdutil.ExitAuth)
	}
	// The exact payload after the token_valid contract fix (issue 624): the
	// state boolean serializes false instead of being dropped by omitempty,
	// matching what docs/COMMANDS.md has always promised.
	want := `{
  "hostname": "gitcode.com",
  "logged_in": true,
  "token_source": "GC_TOKEN",
  "token_valid": false
}`
	if got := strings.TrimSpace(out.String()); got != want {
		t.Fatalf("stdout JSON = %q, want %q", got, want)
	}
	if errOut.String() != "" {
		t.Fatalf("JSON mode must not print text guidance to stderr, got %q", errOut.String())
	}
}
