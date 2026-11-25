package cmdutil

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitcode.com/gitcode-cli/cli/api"
)

const (
	ExitSuccess  = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitNotFound = 3
	ExitAuth     = 4
	ExitConflict = 5
)

// CLIError represents a stable CLI-facing error with an exit code.
type CLIError struct {
	Code    int
	Message string
	Cause   error
	// Silent suppresses the stderr error line at the entry point while
	// keeping the exit code. Protocol commands (credential helpers) use it
	// to signal "no result" without polluting the protocol stream.
	Silent bool
}

func (e *CLIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" && e.Cause != nil {
		return e.Cause.Error()
	}
	if e.Message == "" {
		return "unknown error"
	}
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *CLIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewCLIError creates a CLIError with a stable exit code.
func NewCLIError(code int, message string, cause error) error {
	return &CLIError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func NewUsageError(message string) error {
	return NewCLIError(ExitUsage, message, nil)
}

// NewSilentError creates a CLIError that exits with the given code without
// printing anything to stderr.
func NewSilentError(code int, message string) error {
	return &CLIError{
		Code:    code,
		Message: message,
		Silent:  true,
	}
}

// IsSilent reports whether the error chain contains a silent CLIError that
// should suppress the stderr error line at the entry point.
func IsSilent(err error) bool {
	var cliErr *CLIError
	return errors.As(err, &cliErr) && cliErr != nil && cliErr.Silent
}

func NewAuthError(message string) error {
	return NewCLIError(ExitAuth, message, nil)
}

func NewNotFoundError(message string, cause error) error {
	return NewCLIError(ExitNotFound, message, cause)
}

func NewConflictError(message string) error {
	return NewCLIError(ExitConflict, message, nil)
}

// WrapNotFound wraps an error as NotFoundError if it's a 404 API error.
// Returns the original error if it's not a 404.
// Usage: return cmdutil.WrapNotFound(err, "issue #%d not found in %s/%s", number, owner, repo)
func WrapNotFound(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode == 404 || apiErr.StatusCode == 404) {
		return NewNotFoundError(fmt.Sprintf(format, args...), err)
	}

	return err
}

// FormatAPIID normalizes API IDs that may arrive as strings or JSON numbers.
func FormatAPIID(id interface{}) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case float32:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	default:
		return fmt.Sprintf("%v", id)
	}
}

// ExitCode maps a command error to a stable process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) && cliErr != nil {
		if cliErr.Code > 0 {
			return cliErr.Code
		}
	}

	// Check for Cobra usage errors (argument/flag validation failures)
	if isCobraUsageError(err) {
		return ExitUsage
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode {
		case 404:
			return ExitNotFound
		case 409:
			return ExitConflict
		}
		switch apiErr.StatusCode {
		case 400:
			return ExitUsage
		case 401, 403:
			return ExitAuth
		case 404:
			return ExitNotFound
		case 409:
			return ExitConflict
		default:
			return ExitError
		}
	}

	return ExitError
}

// isCobraUsageError detects Cobra's argument and flag validation errors.
// These errors should return ExitUsage (2) per agent-friendly spec.
func isCobraUsageError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Cobra argument validation errors:
	// - "accepts %d arg(s), received %d" (ExactArgs, MaximumNArgs, RangeArgs)
	// - "requires at least %d arg(s), only received %d" (MinimumNArgs)
	if (strings.HasPrefix(errMsg, "accepts") || strings.HasPrefix(errMsg, "requires")) &&
		strings.Contains(errMsg, "arg(s)") {
		return true
	}

	// Cobra required flag errors:
	// - "required flag(s) \"%s\" not set"
	if strings.Contains(errMsg, "required flag") {
		return true
	}

	// Cobra unknown command/flag errors:
	// - "unknown command %q for %q"
	// - "unknown flag: %s"
	// - "unknown shorthand flag: %s"
	if strings.Contains(errMsg, "unknown command") || strings.Contains(errMsg, "unknown flag") || strings.Contains(errMsg, "unknown shorthand") {
		return true
	}

	return false
}
