package api

import (
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestParseTimeoutFromEnv_Default(t *testing.T) {
	os.Unsetenv("GC_TIMEOUT")
	got := ParseTimeoutFromEnv()
	if got != DefaultTimeout {
		t.Errorf("ParseTimeoutFromEnv() = %v, want %v", got, DefaultTimeout)
	}
}

func TestParseTimeoutFromEnv_WithDuration(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "60s")
	defer os.Unsetenv("GC_TIMEOUT")

	got := ParseTimeoutFromEnv()
	if got != 60*time.Second {
		t.Errorf("ParseTimeoutFromEnv() = %v, want 60s", got)
	}
}

func TestParseTimeoutFromEnv_WithMinutes(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "2m")
	defer os.Unsetenv("GC_TIMEOUT")

	got := ParseTimeoutFromEnv()
	if got != 2*time.Minute {
		t.Errorf("ParseTimeoutFromEnv() = %v, want 2m", got)
	}
}

func TestParseTimeoutFromEnv_WithSecondsOnly(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "120")
	defer os.Unsetenv("GC_TIMEOUT")

	got := ParseTimeoutFromEnv()
	// Should parse as seconds when no unit specified
	if got != 120*time.Second {
		t.Errorf("ParseTimeoutFromEnv() = %v, want 120s", got)
	}
}

func TestParseTimeoutFromEnv_InvalidValue(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "invalid")
	defer os.Unsetenv("GC_TIMEOUT")

	got := ParseTimeoutFromEnv()
	// Should return default on invalid value
	if got != DefaultTimeout {
		t.Errorf("ParseTimeoutFromEnv() = %v, want %v (default)", got, DefaultTimeout)
	}
}

func TestParseTimeoutFromEnv_NegativeValue(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "-10s")
	defer os.Unsetenv("GC_TIMEOUT")

	got := ParseTimeoutFromEnv()
	// Should return default on negative value
	if got != DefaultTimeout {
		t.Errorf("ParseTimeoutFromEnv() = %v, want %v (default)", got, DefaultTimeout)
	}
}

func TestIsDebugEnabled_NotSet(t *testing.T) {
	os.Unsetenv("GC_DEBUG")
	os.Unsetenv("GC_API_DEBUG")

	if IsDebugEnabled() {
		t.Error("IsDebugEnabled() = true, want false")
	}
}

func TestIsDebugEnabled_GCDebugSet(t *testing.T) {
	os.Setenv("GC_DEBUG", "1")
	defer os.Unsetenv("GC_DEBUG")

	if !IsDebugEnabled() {
		t.Error("IsDebugEnabled() = false, want true")
	}
}

func TestIsDebugEnabled_GCAPIDebugSet(t *testing.T) {
	os.Setenv("GC_API_DEBUG", "1")
	defer os.Unsetenv("GC_API_DEBUG")

	if !IsDebugEnabled() {
		t.Error("IsDebugEnabled() = false, want true")
	}
}

func TestNewHTTPClient(t *testing.T) {
	timeout := 45 * time.Second
	client := NewHTTPClient(timeout)

	if client.Timeout != timeout {
		t.Errorf("client.Timeout = %v, want %v", client.Timeout, timeout)
	}
	if client.CheckRedirect == nil {
		t.Error("client.CheckRedirect is nil, want SafeCheckRedirect")
	}
}

func TestNewHTTPClientWithRetry(t *testing.T) {
	timeout := 60 * time.Second
	cfg := DefaultRetryConfig()
	client := NewHTTPClientWithRetry(timeout, cfg)

	if client.Timeout != timeout {
		t.Errorf("client.Timeout = %v, want %v", client.Timeout, timeout)
	}

	// Check that transport is wrapped with retry
	if client.Transport == nil {
		t.Error("client.Transport is nil")
	}

	// Check that base transport is configured properly
	if _, ok := client.Transport.(*retryTransport); !ok {
		t.Errorf("client.Transport is not retryTransport")
	}
}

func TestNewDownloadHTTPClientWithEnvTimeout_Default(t *testing.T) {
	os.Unsetenv("GC_TIMEOUT")
	defer os.Unsetenv("GC_TIMEOUT")

	client := NewDownloadHTTPClientWithEnvTimeout()

	if client.Timeout != DownloadTimeout {
		t.Errorf("client.Timeout = %v, want %v (DownloadTimeout)", client.Timeout, DownloadTimeout)
	}
}

func TestNewDownloadHTTPClientWithEnvTimeout_WithEnv(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "5m")
	defer os.Unsetenv("GC_TIMEOUT")

	client := NewDownloadHTTPClientWithEnvTimeout()

	if client.Timeout != 5*time.Minute {
		t.Errorf("client.Timeout = %v, want 5m", client.Timeout)
	}
}

func TestNewDownloadHTTPClientWithEnvTimeout_WithSeconds(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "300")
	defer os.Unsetenv("GC_TIMEOUT")

	client := NewDownloadHTTPClientWithEnvTimeout()

	// Should parse as seconds when no unit specified
	if client.Timeout != 300*time.Second {
		t.Errorf("client.Timeout = %v, want 300s", client.Timeout)
	}
}

func TestNewDownloadHTTPClientWithEnvTimeout_InvalidValue(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "invalid")
	defer os.Unsetenv("GC_TIMEOUT")

	client := NewDownloadHTTPClientWithEnvTimeout()

	// Should use DownloadTimeout on invalid value
	if client.Timeout != DownloadTimeout {
		t.Errorf("client.Timeout = %v, want %v (DownloadTimeout)", client.Timeout, DownloadTimeout)
	}
}

func TestNewDownloadHTTPClientWithEnvTimeout_NegativeValue(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "-10s")
	defer os.Unsetenv("GC_TIMEOUT")

	client := NewDownloadHTTPClientWithEnvTimeout()

	// Should use DownloadTimeout on negative value
	if client.Timeout != DownloadTimeout {
		t.Errorf("client.Timeout = %v, want %v (DownloadTimeout)", client.Timeout, DownloadTimeout)
	}
}

func TestNewDownloadHTTPClientWithEnvTimeout_ZeroValue(t *testing.T) {
	os.Setenv("GC_TIMEOUT", "0")
	defer os.Unsetenv("GC_TIMEOUT")

	client := NewDownloadHTTPClientWithEnvTimeout()

	// Should use DownloadTimeout on zero value
	if client.Timeout != DownloadTimeout {
		t.Errorf("client.Timeout = %v, want %v (DownloadTimeout)", client.Timeout, DownloadTimeout)
	}
}

func TestSafeCheckRedirect_SameHostKeepsAuth(t *testing.T) {
	origReq, _ := http.NewRequest("GET", "https://api.gitcode.com/api/v5/repos", nil)
	origReq.Header.Set("Authorization", "Bearer secret")

	newReq, _ := http.NewRequest("GET", "https://api.gitcode.com/api/v5/repos/redirected", nil)
	newReq.Header.Set("Authorization", "Bearer secret")

	via := []*http.Request{origReq}
	if err := SafeCheckRedirect(newReq, via); err != nil {
		t.Fatalf("SafeCheckRedirect returned error: %v", err)
	}
	if got := newReq.Header.Get("Authorization"); got != "Bearer secret" {
		t.Errorf("Authorization = %q, want %q (same host must keep header)", got, "Bearer secret")
	}
}

func TestSafeCheckRedirect_CrossHostStripsAuth(t *testing.T) {
	origReq, _ := http.NewRequest("GET", "https://api.gitcode.com/api/v5/repos", nil)
	origReq.Header.Set("Authorization", "Bearer secret")

	newReq, _ := http.NewRequest("GET", "https://evil.example.com/steal", nil)
	newReq.Header.Set("Authorization", "Bearer secret")

	via := []*http.Request{origReq}
	if err := SafeCheckRedirect(newReq, via); err != nil {
		t.Fatalf("SafeCheckRedirect returned error: %v", err)
	}
	if got := newReq.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want empty (cross host must strip header)", got)
	}
}

func TestSafeCheckRedirect_MaxRedirects(t *testing.T) {
	via := make([]*http.Request, maxRedirects)
	for i := range via {
		via[i] = &http.Request{URL: &url.URL{Host: "api.gitcode.com"}}
	}
	newReq := &http.Request{URL: &url.URL{Host: "api.gitcode.com"}}
	err := SafeCheckRedirect(newReq, via)
	if err == nil {
		t.Fatal("SafeCheckRedirect returned nil, want error after max redirects")
	}
}
