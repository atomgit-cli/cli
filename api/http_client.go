// Package api provides HTTP client configuration
package api

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// maxRedirects limits HTTP redirect hops to mitigate redirect-based attacks.
const maxRedirects = 10

// SafeCheckRedirect enforces a safe redirect policy:
//   - limits total redirects to maxRedirects
//   - strips the Authorization header when redirecting to a different host
//
// Go's net/http already removes Authorization on cross-host redirects, but we
// enforce it explicitly as defense-in-depth so token leakage cannot occur even
// if the standard library behavior changes or a transport wraps the client.
func SafeCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if len(via) > 0 && req.URL.Host != via[0].URL.Host {
		req.Header.Del("Authorization")
	}
	return nil
}

// Default timeout values
const (
	DefaultTimeout  = 30 * time.Second
	UploadTimeout   = 10 * time.Minute
	DownloadTimeout = 10 * time.Minute
	LongPollTimeout = 5 * time.Minute
)

// DefaultHTTPClient returns an HTTP client with default configuration
func DefaultHTTPClient() *http.Client {
	return NewHTTPClient(DefaultTimeout)
}

// NewHTTPClient creates an HTTP client with specified timeout
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     newDefaultTransport(),
		CheckRedirect: SafeCheckRedirect,
	}
}

// NewHTTPClientWithRetry creates an HTTP client with retry middleware
func NewHTTPClientWithRetry(timeout time.Duration, cfg RetryConfig) *http.Client {
	transport := newDefaultTransport()
	return &http.Client{
		Timeout:       timeout,
		Transport:     RetryMiddleware(transport, cfg),
		CheckRedirect: SafeCheckRedirect,
	}
}

// NewHTTPClientWithRetryAndLogger creates an HTTP client with retry middleware and debug logging
func NewHTTPClientWithRetryAndLogger(timeout time.Duration, cfg RetryConfig, logger func(string)) *http.Client {
	transport := newDefaultTransport()
	return &http.Client{
		Timeout:       timeout,
		Transport:     RetryMiddlewareWithLogger(transport, cfg, logger),
		CheckRedirect: SafeCheckRedirect,
	}
}

// NewDownloadHTTPClientWithEnvTimeout returns an HTTP client for file downloads.
// If GC_TIMEOUT environment variable is set, uses that value as timeout.
// Otherwise uses the default DownloadTimeout (10 minutes).
func NewDownloadHTTPClientWithEnvTimeout() *http.Client {
	timeout := parseTimeoutFromEnvWithDefault(DownloadTimeout)
	return NewHTTPClientWithRetry(timeout, DefaultRetryConfig())
}

// parseTimeoutFromEnvWithDefault parses timeout from GC_TIMEOUT environment variable.
// Returns the provided default timeout if GC_TIMEOUT is not set or has invalid value.
func parseTimeoutFromEnvWithDefault(defaultTimeout time.Duration) time.Duration {
	if v := os.Getenv("GC_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil && d > 0 {
			return d
		}
		seconds, err := strconv.Atoi(v)
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultTimeout
}

// newDefaultTransport returns the default HTTP transport
func newDefaultTransport() http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// ParseTimeoutFromEnv parses timeout from GC_TIMEOUT environment variable
func ParseTimeoutFromEnv() time.Duration {
	return parseTimeoutFromEnvWithDefault(DefaultTimeout)
}

// IsDebugEnabled returns true if GC_DEBUG is set
func IsDebugEnabled() bool {
	return os.Getenv("GC_DEBUG") != "" || os.Getenv("GC_API_DEBUG") != ""
}
