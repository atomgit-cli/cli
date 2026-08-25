package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.InitialWait != 1*time.Second {
		t.Errorf("InitialWait = %v, want 1s", cfg.InitialWait)
	}
	if cfg.MaxWait != 30*time.Second {
		t.Errorf("MaxWait = %v, want 30s", cfg.MaxWait)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", cfg.Multiplier)
	}
}

func TestRetryMiddleware_Success(t *testing.T) {
	t.Parallel()
	callCount := 0
	mockTransport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	cfg := DefaultRetryConfig()
	client := &http.Client{Transport: RetryMiddleware(mockTransport, cfg)}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (no retry on success)", callCount)
	}
}

func TestRetryMiddleware_ServerError(t *testing.T) {
	t.Parallel()
	callCount := 0
	mockTransport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount < 3 {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	cfg := RetryConfig{MaxRetries: 3, InitialWait: 10 * time.Millisecond, MaxWait: 1 * time.Second, Multiplier: 2.0}
	client := &http.Client{Transport: RetryMiddleware(mockTransport, cfg)}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3 (2 retries)", callCount)
	}
}

func TestRetryMiddleware_RateLimit(t *testing.T) {
	t.Parallel()
	callCount := 0
	mockTransport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			resp := &http.Response{
				StatusCode: 429,
				Header:     http.Header{"Retry-After": []string{"1"}},
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}
			return resp, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	cfg := RetryConfig{MaxRetries: 3, InitialWait: 10 * time.Millisecond, MaxWait: 1 * time.Second, Multiplier: 2.0}
	client := &http.Client{Transport: RetryMiddleware(mockTransport, cfg)}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (1 retry after Rate Limit)", callCount)
	}
}

func TestRetryMiddleware_NoRetryOn401(t *testing.T) {
	t.Parallel()
	callCount := 0
	mockTransport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: 401, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	cfg := DefaultRetryConfig()
	client := &http.Client{Transport: RetryMiddleware(mockTransport, cfg)}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (no retry on 401)", callCount)
	}
}

func TestRetryMiddleware_Exhausted(t *testing.T) {
	t.Parallel()
	callCount := 0
	mockTransport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	cfg := RetryConfig{MaxRetries: 2, InitialWait: 10 * time.Millisecond, MaxWait: 1 * time.Second, Multiplier: 2.0}
	client := &http.Client{Transport: RetryMiddleware(mockTransport, cfg)}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500 (last response)", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3 (initial + 2 retries)", callCount)
	}
}

func TestShouldRetryOnStatus(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{cfg: DefaultRetryConfig()}

	tests := []struct {
		code     int
		expected bool
	}{
		{200, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tt := range tests {
		tt := tt
		got := rt.shouldRetryOnStatus(tt.code)
		if got != tt.expected {
			t.Errorf("shouldRetryOnStatus(%d) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestShouldRetryOnErrorWrappedContext(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{cfg: DefaultRetryConfig()}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil", err: nil, expected: false},
		{name: "bare context.Canceled", err: context.Canceled, expected: false},
		{name: "bare context.DeadlineExceeded", err: context.DeadlineExceeded, expected: false},
		{name: "wrapped context.Canceled", err: fmt.Errorf("request failed: %w", context.Canceled), expected: false},
		{name: "wrapped context.DeadlineExceeded", err: fmt.Errorf("request failed: %w", context.DeadlineExceeded), expected: false},
		{name: "other network error", err: errors.New("connection refused"), expected: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rt.shouldRetryOnError(tt.err)
			if got != tt.expected {
				t.Errorf("shouldRetryOnError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestCalculateWait(t *testing.T) {
	t.Parallel()
	cfg := RetryConfig{
		MaxRetries:  3,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
	rt := &retryTransport{cfg: cfg}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{10, 30 * time.Second}, // capped at MaxWait
	}

	for _, tt := range tests {
		tt := tt
		got := rt.calculateWait(tt.attempt)
		if got != tt.expected {
			t.Errorf("calculateWait(%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestSanitizeError_UrlErrorStripsHost(t *testing.T) {
	t.Parallel()
	inner := errors.New("dial tcp: connection refused")
	ue := &url.Error{Op: "Get", URL: "https://api.gitcode.com/api/v5/repos", Err: inner}

	got := sanitizeError(ue)
	if strings.Contains(got, "api.gitcode.com") {
		t.Errorf("sanitizeError leaked host: %q", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("sanitizeError lost inner error: %q", got)
	}
}

func TestSanitizeError_PlainError(t *testing.T) {
	t.Parallel()
	err := errors.New("something failed")
	if got := sanitizeError(err); got != "something failed" {
		t.Errorf("sanitizeError = %q, want %q", got, "something failed")
	}
}

func TestSanitizeError_NilInnerUsesOp(t *testing.T) {
	t.Parallel()
	ue := &url.Error{Op: "Get", URL: "https://api.gitcode.com/path", Err: nil}
	if got := sanitizeError(ue); got != "Get" {
		t.Errorf("sanitizeError = %q, want %q", got, "Get")
	}
}

func TestRetrySkipsNonRewindableBody(t *testing.T) {
	t.Parallel()
	callCount := 0
	mockTransport := testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte("server error"))),
		}, nil
	})

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 3
	cfg.InitialWait = 1 * time.Millisecond
	client := &http.Client{Transport: RetryMiddleware(mockTransport, cfg)}

	req, _ := http.NewRequest("POST", "http://example.com", io.NopCloser(strings.NewReader("body data")))
	// io.NopCloser doesn't match *bytes.Reader/*strings.Reader/*bytes.Buffer,
	// so GetBody stays nil — body is non-rewindable

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close()

	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1 (non-rewindable body should skip retries)", callCount)
	}
}
