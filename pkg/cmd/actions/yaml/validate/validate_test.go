package validate

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "file", args: []string{"--file", "ci.yml", "-R", "owner/repo"}, wantErr: false},
		{name: "stdin", args: []string{"--file", "-"}, wantErr: false},
		{name: "json", args: []string{"--file", "ci.yml", "--json"}, wantErr: false},
		{name: "no file", args: []string{}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdValidate(f, func(opts *ValidateOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRunValid(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, out, _ := iostreams.Test()
	var gotMethod string
	var gotPath string
	var gotBody string
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotMethod = req.Method
					gotPath = req.URL.Path
					body, _ := io.ReadAll(req.Body)
					gotBody = string(body)
					return validateTestResponse(http.StatusOK, `{"valid":true,"diagnostics":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "ci.yml"),
	}
	if err := os.WriteFile(opts.File, []byte("name: ci\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	if err := validateRun(opts); err != nil {
		t.Fatalf("validateRun() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	wantPath := "/api/v8/repos/owner/repo/actions/workflows/validate"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	assertBase64Body(t, gotBody, "name: ci\n")
	if !strings.Contains(out.String(), "Workflow YAML is valid") {
		t.Fatalf("output = %q, want valid message", out.String())
	}
}

func TestValidateRunStdin(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, in, out, _ := iostreams.Test()
	if _, err := in.WriteString("name: ci\n"); err != nil {
		t.Fatalf("in.WriteString() error = %v", err)
	}
	var gotBody string
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gotBody = string(body)
					return validateTestResponse(http.StatusOK, `{"valid":true,"diagnostics":[]}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       "-",
	}

	if err := validateRun(opts); err != nil {
		t.Fatalf("validateRun() error = %v", err)
	}
	assertBase64Body(t, gotBody, "name: ci\n")
	if !strings.Contains(out.String(), "Workflow YAML is valid") {
		t.Fatalf("output = %q, want valid message", out.String())
	}
}

func TestValidateRunInvalid(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, out, _ := iostreams.Test()
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return validateTestResponse(http.StatusOK, `{
						"valid": false,
						"diagnostics": [
							{
								"range": {"start": {"line": 2, "column": 4}, "end": {"line": 2, "column": 5}},
								"severity": "error",
								"message": "bad yaml"
							}
						]
					}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "ci.yml"),
	}
	if err := os.WriteFile(opts.File, []byte("name: ci\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	err := validateRun(opts)
	if err == nil {
		t.Fatal("validateRun() error = nil, want error for invalid YAML")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitError)
	}
	if !strings.Contains(out.String(), "Workflow YAML is invalid") {
		t.Fatalf("output = %q, want invalid message", out.String())
	}
	if !strings.Contains(out.String(), "line 2, column 4") || !strings.Contains(out.String(), "bad yaml") {
		t.Fatalf("output = %q, want diagnostic location and message", out.String())
	}
}

func TestValidateRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	body := `{"valid":true,"diagnostics":[]}`
	ios, _, out, _ := iostreams.Test()
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return validateTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "ci.yml"),
		JSON:       true,
	}
	if err := os.WriteFile(opts.File, []byte("name: ci\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	if err := validateRun(opts); err != nil {
		t.Fatalf("validateRun() error = %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if result["valid"] != true {
		t.Fatalf("valid = %v, want true", result["valid"])
	}
}

func TestValidateRunJSONInvalidStillEmitsJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	body := `{"valid":false,"diagnostics":[{"range":{"start":{"line":0,"column":0},"end":{"line":0,"column":0}},"severity":"error","message":"bad"}]}`
	ios, _, out, _ := iostreams.Test()
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return validateTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "ci.yml"),
		JSON:       true,
	}
	if err := os.WriteFile(opts.File, []byte("name: ci\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	err := validateRun(opts)
	if err == nil {
		t.Fatal("validateRun() error = nil, want error for invalid YAML")
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if result["valid"] != false {
		t.Fatalf("valid = %v, want false", result["valid"])
	}
}

func TestValidateRunMissingFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, _, _ := iostreams.Test()
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatal("API should not be called for a missing file")
					return nil, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "missing.yml"),
	}

	err := validateRun(opts)
	if err == nil {
		t.Fatal("validateRun() error = nil, want error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to read workflow YAML file") {
		t.Fatalf("error = %q, want file read error", err.Error())
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitError)
	}
}

func TestValidateRunEmptyFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, _, _ := iostreams.Test()
	var gotBody string
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gotBody = string(body)
					return validateTestResponse(http.StatusBadRequest, `{"message":"base64content is required"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "empty.yml"),
	}
	if err := os.WriteFile(opts.File, nil, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	err := validateRun(opts)
	if err == nil {
		t.Fatal("validateRun() error = nil, want API error for empty content")
	}
	assertBase64Body(t, gotBody, "")
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestValidateRunRejectsContentWithSecret(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-secret-token")

	ios, _, _, _ := iostreams.Test()
	opts := &ValidateOptions{
		IO: ios,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatal("API should not be called when content contains the current token")
					return nil, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		File:       filepath.Join(t.TempDir(), "ci.yml"),
	}
	secretContent := "name: ci\ntoken: test-secret-token\n"
	if err := os.WriteFile(opts.File, []byte(secretContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	err := validateRun(opts)
	if err == nil {
		t.Fatal("validateRun() error = nil, want secret scan error")
	}
	if !strings.Contains(err.Error(), "content contains secret") {
		t.Fatalf("error = %q, want secret scan error", err.Error())
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitError)
	}
}

func TestValidateRunAPIError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	tests := []struct {
		name     string
		status   int
		wantExit int
	}{
		{name: "401 unauthorized", status: http.StatusUnauthorized, wantExit: cmdutil.ExitAuth},
		{name: "403 forbidden", status: http.StatusForbidden, wantExit: cmdutil.ExitAuth},
		{name: "404 not found", status: http.StatusNotFound, wantExit: cmdutil.ExitNotFound},
		{name: "409 conflict", status: http.StatusConflict, wantExit: cmdutil.ExitConflict},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ios, _, _, _ := iostreams.Test()
			opts := &ValidateOptions{
				IO: ios,
				HttpClient: func() (*http.Client, error) {
					return &http.Client{
						Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
							return validateTestResponse(tt.status, `{"message":"api error"}`), nil
						}),
					}, nil
				},
				Repository: "owner/repo",
				File:       filepath.Join(t.TempDir(), "ci.yml"),
			}
			if err := os.WriteFile(opts.File, []byte("name: ci\n"), 0o644); err != nil {
				t.Fatalf("os.WriteFile() error = %v", err)
			}

			err := validateRun(opts)
			if err == nil {
				t.Fatal("validateRun() error = nil, want API error")
			}
			if got := cmdutil.ExitCode(err); got != tt.wantExit {
				t.Fatalf("ExitCode = %d, want %d", got, tt.wantExit)
			}
		})
	}
}

func assertBase64Body(t *testing.T, gotBody, wantContent string) {
	t.Helper()
	var req map[string]string
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request body is not JSON: %v; body=%q", err, gotBody)
	}
	decoded, err := base64.StdEncoding.DecodeString(req["base64_content"])
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}
	if string(decoded) != wantContent {
		t.Fatalf("decoded content = %q, want %q", string(decoded), wantContent)
	}
}

func validateTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
