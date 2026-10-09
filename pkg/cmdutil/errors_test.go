package cmdutil

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
)

func TestExitCode(t *testing.T) {
	t.Run("success returns ExitSuccess", func(t *testing.T) {
		if got := ExitCode(nil); got != ExitSuccess {
			t.Fatalf("ExitCode(nil) = %d, want %d", got, ExitSuccess)
		}
	})

	t.Run("cli usage error", func(t *testing.T) {
		if got := ExitCode(NewUsageError("bad args")); got != ExitUsage {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cli auth error", func(t *testing.T) {
		if got := ExitCode(NewAuthError("not authenticated")); got != ExitAuth {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitAuth)
		}
	})

	t.Run("cli not found error", func(t *testing.T) {
		if got := ExitCode(NewNotFoundError("resource not found", nil)); got != ExitNotFound {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitNotFound)
		}
	})

	t.Run("cli conflict error", func(t *testing.T) {
		if got := ExitCode(NewConflictError("conflict")); got != ExitConflict {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitConflict)
		}
	})

	t.Run("api auth error", func(t *testing.T) {
		err := &api.APIError{StatusCode: 401, Message: "unauthorized"}
		if got := ExitCode(err); got != ExitAuth {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitAuth)
		}
	})

	t.Run("api embedded not found error code", func(t *testing.T) {
		err := &api.APIError{StatusCode: 400, ErrorCode: 404, ErrorMessage: "404 Not Found Commit"}
		if got := ExitCode(err); got != ExitNotFound {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitNotFound)
		}
	})

	t.Run("generic error", func(t *testing.T) {
		if got := ExitCode(errors.New("boom")); got != ExitError {
			t.Fatalf("ExitCode() = %d, want %d", got, ExitError)
		}
	})

	// Cobra usage errors should return ExitUsage (2)
	t.Run("cobra missing args error", func(t *testing.T) {
		err := errors.New("accepts 1 arg(s), received 0")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra missing args) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra too many args error", func(t *testing.T) {
		err := errors.New("accepts at most 1 arg(s), received 2")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra too many args) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra minimum args error", func(t *testing.T) {
		err := errors.New("requires at least 2 arg(s), only received 1")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra minimum args) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra required flag error", func(t *testing.T) {
		err := errors.New("required flag(s) \"target-repo\" not set")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra required flag) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra unknown command error", func(t *testing.T) {
		err := errors.New("unknown command \"foo\" for \"gc\"")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra unknown command) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra unknown flag error", func(t *testing.T) {
		err := errors.New("unknown flag: --unknown-flag")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra unknown flag) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra unknown shorthand flag error", func(t *testing.T) {
		err := errors.New("unknown shorthand flag: -f in `-f'")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra unknown shorthand flag) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("cobra range args error", func(t *testing.T) {
		err := errors.New("accepts between 1 and 3 arg(s), received 4")
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(cobra range args) = %d, want %d", got, ExitUsage)
		}
	})

	// APIError StatusCode path coverage (issue #352)
	t.Run("api error 403 returns ExitAuth", func(t *testing.T) {
		err := &api.APIError{StatusCode: 403, Message: "forbidden"}
		if got := ExitCode(err); got != ExitAuth {
			t.Fatalf("ExitCode(api 403) = %d, want %d", got, ExitAuth)
		}
	})

	t.Run("api error 400 returns ExitUsage", func(t *testing.T) {
		err := &api.APIError{StatusCode: 400, Message: "bad request"}
		if got := ExitCode(err); got != ExitUsage {
			t.Fatalf("ExitCode(api 400) = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("api error 404 via StatusCode returns ExitNotFound", func(t *testing.T) {
		err := &api.APIError{StatusCode: 404, ErrorMessage: "Not Found"}
		if got := ExitCode(err); got != ExitNotFound {
			t.Fatalf("ExitCode(api 404 StatusCode) = %d, want %d", got, ExitNotFound)
		}
	})

	t.Run("api error 409 via ErrorCode returns ExitConflict", func(t *testing.T) {
		err := &api.APIError{StatusCode: 400, ErrorCode: 409, Message: "conflict"}
		if got := ExitCode(err); got != ExitConflict {
			t.Fatalf("ExitCode(api 409 ErrorCode) = %d, want %d", got, ExitConflict)
		}
	})

	t.Run("api error 409 via StatusCode returns ExitConflict", func(t *testing.T) {
		err := &api.APIError{StatusCode: 409, Message: "conflict"}
		if got := ExitCode(err); got != ExitConflict {
			t.Fatalf("ExitCode(api 409 StatusCode) = %d, want %d", got, ExitConflict)
		}
	})

	// 5xx and unknown status codes fall through to ExitError
	t.Run("api error 500 returns ExitError", func(t *testing.T) {
		err := &api.APIError{StatusCode: 500, Message: "internal server error"}
		if got := ExitCode(err); got != ExitError {
			t.Fatalf("ExitCode(api 500) = %d, want %d", got, ExitError)
		}
	})

	t.Run("api error 503 returns ExitError", func(t *testing.T) {
		err := &api.APIError{StatusCode: 503, Message: "service unavailable"}
		if got := ExitCode(err); got != ExitError {
			t.Fatalf("ExitCode(api 503) = %d, want %d", got, ExitError)
		}
	})

	t.Run("api error 422 returns ExitError", func(t *testing.T) {
		err := &api.APIError{StatusCode: 422, Message: "unprocessable entity"}
		if got := ExitCode(err); got != ExitError {
			t.Fatalf("ExitCode(api 422) = %d, want %d", got, ExitError)
		}
	})

	// Wrapped error via fmt.Errorf(%w)
	t.Run("wrapped cli error via fmt.Errorf", func(t *testing.T) {
		inner := NewAuthError("not authenticated")
		wrapped := fmt.Errorf("additional context: %w", inner)
		if got := ExitCode(wrapped); got != ExitAuth {
			t.Fatalf("ExitCode(wrapped CLIError) = %d, want %d", got, ExitAuth)
		}
	})

	t.Run("wrapped api error via fmt.Errorf", func(t *testing.T) {
		inner := &api.APIError{StatusCode: 403, Message: "forbidden"}
		wrapped := fmt.Errorf("request failed: %w", inner)
		if got := ExitCode(wrapped); got != ExitAuth {
			t.Fatalf("ExitCode(wrapped APIError) = %d, want %d", got, ExitAuth)
		}
	})

	// nil CLIError boundary — a typed nil *CLIError through error interface
	// is non-nil in Go (interface has type but nil value), so it falls through
	// to ExitError. The nil guard in ExitCode prevents the dereference panic.
	t.Run("nil cli error boundary", func(t *testing.T) {
		var cliErr *CLIError = nil
		if got := ExitCode(cliErr); got != ExitError {
			t.Fatalf("ExitCode(nil CLIError) = %d, want %d", got, ExitError)
		}
	})

	t.Run("cli error with zero code falls through", func(t *testing.T) {
		err := &CLIError{Code: 0, Message: "zero code"}
		if got := ExitCode(err); got != ExitError {
			t.Fatalf("ExitCode(CLIError Code=0) = %d, want %d", got, ExitError)
		}
	})

	// net.OpError — network errors return ExitError
	t.Run("net op error returns ExitError", func(t *testing.T) {
		netErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
		if got := ExitCode(netErr); got != ExitError {
			t.Fatalf("ExitCode(net.OpError) = %d, want %d", got, ExitError)
		}
	})

	// releaseError-like error (no special type — falls through to generic ExitError)
	t.Run("generic errors.New returns ExitError", func(t *testing.T) {
		releaseErr := errors.New("invalid release ID")
		if got := ExitCode(releaseErr); got != ExitError {
			t.Fatalf("ExitCode(release-like error) = %d, want %d", got, ExitError)
		}
	})
}

func TestWrapNotFound(t *testing.T) {
	t.Run("nil error returns nil", func(t *testing.T) {
		if got := WrapNotFound(nil, "issue %s not found", "1"); got != nil {
			t.Fatalf("WrapNotFound(nil) = %v, want nil", got)
		}
	})

	t.Run("generic error returns unchanged", func(t *testing.T) {
		orig := errors.New("generic")
		if got := WrapNotFound(orig, "issue %s not found", "1"); got != orig {
			t.Fatalf("WrapNotFound(generic) returned different error, want original")
		}
	})

	t.Run("api 404 via StatusCode wraps to NotFoundError", func(t *testing.T) {
		api404 := &api.APIError{StatusCode: 404, ErrorMessage: "Not Found"}
		got := WrapNotFound(api404, "issue #%d not found in %s/%s", 42, "owner", "repo")
		var cliErr *CLIError
		if !errors.As(got, &cliErr) {
			t.Fatalf("WrapNotFound(api 404) should return CLIError, got %T", got)
		}
		if cliErr.Code != ExitNotFound {
			t.Fatalf("WrapNotFound(api 404) Code = %d, want %d", cliErr.Code, ExitNotFound)
		}
		if cliErr.Cause != api404 {
			t.Fatalf("WrapNotFound cause should be original APIError")
		}
	})

	t.Run("api 404 via ErrorCode wraps to NotFoundError", func(t *testing.T) {
		api404 := &api.APIError{StatusCode: 400, ErrorCode: 404, ErrorMessage: "404 Not Found Commit"}
		got := WrapNotFound(api404, "commit %s not found", "abc123")
		var cliErr *CLIError
		if !errors.As(got, &cliErr) {
			t.Fatalf("WrapNotFound(api ErrorCode 404) should return CLIError, got %T", got)
		}
		if cliErr.Code != ExitNotFound {
			t.Fatalf("WrapNotFound(api ErrorCode 404) Code = %d, want %d", cliErr.Code, ExitNotFound)
		}
	})

	t.Run("api 401 returns unchanged", func(t *testing.T) {
		api401 := &api.APIError{StatusCode: 401, Message: "unauthorized"}
		if got := WrapNotFound(api401, "issue #%d", 1); got != api401 {
			t.Fatalf("WrapNotFound(api 401) should return original, got %v", got)
		}
	})
}

func TestCLIError_Error(t *testing.T) {
	t.Run("nil receiver returns empty string", func(t *testing.T) {
		var cliErr *CLIError
		if got := cliErr.Error(); got != "" {
			t.Fatalf("nil CLIError.Error() = %q, want empty", got)
		}
	})

	t.Run("message only", func(t *testing.T) {
		err := &CLIError{Code: ExitUsage, Message: "bad args"}
		if got := err.Error(); got != "bad args" {
			t.Fatalf("Error() = %q, want %q", got, "bad args")
		}
	})

	t.Run("cause only", func(t *testing.T) {
		cause := errors.New("underlying")
		err := &CLIError{Code: ExitError, Cause: cause}
		if got := err.Error(); got != "underlying" {
			t.Fatalf("Error() = %q, want %q", got, "underlying")
		}
	})

	t.Run("message and cause", func(t *testing.T) {
		cause := errors.New("EOF")
		err := &CLIError{Code: ExitError, Message: "read failed", Cause: cause}
		expected := "read failed: EOF"
		if got := err.Error(); got != expected {
			t.Fatalf("Error() = %q, want %q", got, expected)
		}
	})

	t.Run("empty message and no cause", func(t *testing.T) {
		err := &CLIError{Code: ExitError}
		if got := err.Error(); got != "unknown error" {
			t.Fatalf("Error() = %q, want %q", got, "unknown error")
		}
	})
}

func TestCLIError_Unwrap(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var cliErr *CLIError
		if got := cliErr.Unwrap(); got != nil {
			t.Fatalf("nil CLIError.Unwrap() = %v, want nil", got)
		}
	})

	t.Run("no cause returns nil", func(t *testing.T) {
		err := &CLIError{Code: ExitUsage, Message: "bad args"}
		if got := err.Unwrap(); got != nil {
			t.Fatalf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("with cause returns cause", func(t *testing.T) {
		cause := errors.New("underlying")
		err := &CLIError{Code: ExitError, Cause: cause}
		if got := err.Unwrap(); got != cause {
			t.Fatalf("Unwrap() = %v, want %v", got, cause)
		}
	})
}

func TestFormatAPIID(t *testing.T) {
	cases := map[string]struct {
		input interface{}
		want  string
	}{
		"string":  {input: "123", want: "123"},
		"float64": {input: 123.0, want: "123"},
		"float32": {input: float32(456), want: "456"},
		"int":     {input: 123, want: "123"},
		"int64":   {input: int64(-42), want: "-42"},
		"int32":   {input: int32(7), want: "7"},
		"uint":    {input: uint(9), want: "9"},
		"uint64":  {input: uint64(18446744073709551615), want: "18446744073709551615"},
		"uint32":  {input: uint32(3000000000), want: "3000000000"},
		"nil":     {input: nil, want: ""},
		"other":   {input: true, want: "true"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := FormatAPIID(tc.input); got != tc.want {
				t.Fatalf("FormatAPIID(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
