package cmdutil

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestAuthenticatedClientRejectsEnvTokenForCustomHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "enterprise.example.com")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	_, err := AuthenticatedClient(&http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected request to %s", req.URL.String())
			return nil, nil
		}),
	})
	if err == nil {
		t.Fatal("AuthenticatedClient() error = nil, want auth error")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("AuthenticatedClient() error = %q, want not authenticated", err.Error())
	}
}

func TestAuthenticatedClientUsesStoredTokenForCustomHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "enterprise.example.com")
	t.Setenv("GC_TOKEN", "env-token")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("enterprise.example.com", "tester", "stored-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	client, err := AuthenticatedClient(&http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "api.enterprise.example.com" {
				t.Fatalf("request host = %q, want api.enterprise.example.com", req.URL.Host)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer stored-token" {
				t.Fatalf("Authorization = %q, want stored token", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("AuthenticatedClient() error = %v", err)
	}
	if err := client.Get("/user", &struct{}{}); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestAuthenticatedClientRejectsMalformedHost(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "https://gitcode.com")
	t.Setenv("GC_TOKEN", "env-token")

	_, err := AuthenticatedClient(&http.Client{})
	if err == nil {
		t.Fatal("AuthenticatedClient() error = nil, want invalid host error")
	}
	if !strings.Contains(err.Error(), "invalid host") {
		t.Fatalf("AuthenticatedClient() error = %q, want invalid host", err.Error())
	}
}

// TestDefaultTokenPrefersGCToken verifies GC_TOKEN wins over GITCODE_TOKEN and
// the stored config file.
func TestDefaultTokenPrefersGCToken(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "env-token-gc")
	t.Setenv("GITCODE_TOKEN", "env-token-gitcode")

	if got := DefaultToken(); got != "env-token-gc" {
		t.Fatalf("DefaultToken() = %q, want GC_TOKEN value", got)
	}
}

// TestDefaultTokenFallsBackToGitcodeToken verifies GITCODE_TOKEN is used when
// GC_TOKEN is unset.
func TestDefaultTokenFallsBackToGitcodeToken(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "env-token-gitcode")

	if got := DefaultToken(); got != "env-token-gitcode" {
		t.Fatalf("DefaultToken() = %q, want GITCODE_TOKEN value", got)
	}
}

// TestDefaultTokenFallsBackToStoredToken verifies the stored config token is
// used when both environment variables are unset.
func TestDefaultTokenFallsBackToStoredToken(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", configDir)
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	cfg := config.New()
	if _, err := cfg.Authentication().Login("gitcode.com", "tester", "stored-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if got := DefaultToken(); got != "stored-token" {
		t.Fatalf("DefaultToken() = %q, want stored token", got)
	}
}

// TestDefaultTokenEmptyWhenUnauthenticated verifies DefaultToken returns an
// empty string when no env token and no stored token exist.
func TestDefaultTokenEmptyWhenUnauthenticated(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	if got := DefaultToken(); got != "" {
		t.Fatalf("DefaultToken() = %q, want empty when unauthenticated", got)
	}
}

// TestAuthenticatedClientFromFactoryRejectsNilFactory verifies the nil HTTP
// client factory guard.
func TestAuthenticatedClientFromFactoryRejectsNilFactory(t *testing.T) {
	_, err := AuthenticatedClientFromFactory(nil)
	if err == nil {
		t.Fatal("AuthenticatedClientFromFactory(nil) = nil, want error")
	}
	if !strings.Contains(err.Error(), "missing HTTP client factory") {
		t.Fatalf("error = %q, want 'missing HTTP client factory'", err.Error())
	}
}

// TestAuthenticatedClientFromFactoryWrapsClientError verifies an HTTP client
// creation failure is wrapped with context.
func TestAuthenticatedClientFromFactoryWrapsClientError(t *testing.T) {
	clientErr := errors.New("dial tcp: connection refused")
	_, err := AuthenticatedClientFromFactory(func() (*http.Client, error) {
		return nil, clientErr
	})
	if err == nil {
		t.Fatal("AuthenticatedClientFromFactory(client error) = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to create HTTP client") {
		t.Fatalf("error = %q, want 'failed to create HTTP client'", err.Error())
	}
	if !errors.Is(err, clientErr) {
		t.Fatalf("error chain does not preserve the client error cause")
	}
}

// TestAuthenticatedClientFromFactoryRequiresAuthentication verifies the happy
// wiring path reaches the auth check and surfaces its error when no token is
// configured.
func TestAuthenticatedClientFromFactoryRequiresAuthentication(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_HOST", "enterprise.example.com")
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")

	_, err := AuthenticatedClientFromFactory(func() (*http.Client, error) {
		return &http.Client{}, nil
	})
	if err == nil {
		t.Fatal("AuthenticatedClientFromFactory(unauthenticated) = nil, want auth error")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("error = %q, want 'not authenticated'", err.Error())
	}
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("error type = %T, want *CLIError", err)
	}
	if cliErr.Code != ExitAuth {
		t.Fatalf("error code = %d, want %d", cliErr.Code, ExitAuth)
	}
}
