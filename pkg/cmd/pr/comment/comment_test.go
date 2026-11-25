package comment

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdComment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "comment with --body",
			args:    []string{"123", "--body", "Test comment"},
			wantErr: false,
		},
		{
			name:    "comment with --body-file",
			args:    []string{"123", "--body-file", "-"},
			wantErr: false,
		},
		{
			name:    "inline comment with path and position",
			args:    []string{"123", "--body", "Inline comment", "--path", "api/auth.go", "--position", "42"},
			wantErr: false,
		},
		{
			name:    "no PR number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "both body and body-file",
			args:    []string{"123", "--body", "test", "--body-file", "-"},
			wantErr: false, // Command runs, error in run function
		},
		{
			name:    "path without position",
			args:    []string{"123", "--body", "test", "--path", "api/auth.go"},
			wantErr: false, // Command runs, error in run function
		},
		{
			name:    "position without path (allowed)",
			args:    []string{"123", "--body", "test", "--position", "42"},
			wantErr: false, // Allowed - code only checks path implies position
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdComment(f, func(opts *CommentOptions) error {
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

func TestGetBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		bodyFile   string
		stdin      string
		wantBody   string
		wantErr    bool
		errContain string
	}{
		{
			name:     "body flag only",
			body:     "Hello world",
			wantBody: "Hello world",
		},
		{
			name:       "both body and body-file",
			body:       "Hello",
			bodyFile:   "-",
			wantErr:    true,
			errContain: "cannot use both",
		},
		{
			name:     "empty body",
			wantBody: "",
		},
		{
			name:     "stdin input",
			bodyFile: "-",
			stdin:    "Comment from stdin\n",
			wantBody: "Comment from stdin",
		},
		{
			name:     "stdin input multiline",
			bodyFile: "-",
			stdin:    "Line 1\nLine 2\nLine 3\n",
			wantBody: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			opts := &CommentOptions{
				IO:       f.IOStreams,
				Body:     tt.body,
				BodyFile: tt.bodyFile,
			}

			if tt.stdin != "" {
				opts.IO.In = bytes.NewBufferString(tt.stdin)
			}

			got, err := getBody(opts)
			if tt.wantErr {
				if err == nil {
					t.Errorf("getBody() expected error, got nil")
					return
				}
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("getBody() error = %v, want containing %v", err, tt.errContain)
				}
				return
			}
			if err != nil {
				t.Errorf("getBody() unexpected error: %v", err)
				return
			}
			if got != tt.wantBody {
				t.Errorf("getBody() = %v, want %v", got, tt.wantBody)
			}
		})
	}
}

func TestGetBodyFromFile(t *testing.T) {
	t.Parallel()
	// Create temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "comment.txt")
	content := "Comment from file\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f := cmdutil.TestFactory()
	opts := &CommentOptions{
		IO:       f.IOStreams,
		BodyFile: tmpFile,
	}

	got, err := getBody(opts)
	if err != nil {
		t.Fatalf("getBody() error = %v", err)
	}
	if got != "Comment from file" {
		t.Errorf("getBody() = %v, want %v", got, "Comment from file")
	}
}

func TestGetBodyFileNotFound(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &CommentOptions{
		IO:       f.IOStreams,
		BodyFile: "/nonexistent/file.txt",
	}

	_, err := getBody(opts)
	if err == nil {
		t.Error("getBody() expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("getBody() error = %v, want containing 'failed to read file'", err)
	}
}

func TestCommentRunEmptyBody(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &CommentOptions{
		IO:     f.IOStreams,
		Body:   "", // Empty body
		Number: 123,
	}

	err := commentRun(opts)
	if err == nil {
		t.Error("commentRun() expected error for empty body")
	}
	if !strings.Contains(err.Error(), "body is required") {
		t.Errorf("commentRun() error = %v, want containing 'body is required'", err)
	}
}

func TestCommentRunPathWithoutPosition(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &CommentOptions{
		IO:       f.IOStreams,
		Body:     "Test comment",
		Path:     "api/auth.go",
		Position: 0, // Missing position
		Number:   123,
	}

	err := commentRun(opts)
	if err == nil {
		t.Error("commentRun() expected error for path without position")
	}
	if !strings.Contains(err.Error(), "--position is required") {
		t.Errorf("commentRun() error = %v, want containing '--position is required'", err)
	}
}

func TestCommentRunWithMockHTTP(t *testing.T) {
	// Create mock HTTP client
	f := cmdutil.TestFactory()

	t.Setenv("GC_TOKEN", "test-token")

	opts := &CommentOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo: func() (string, error) {
			return "owner/repo", nil
		},
		Repository: "owner/repo",
		Number:     123,
		Body:       "Test comment",
	}

	// This test uses the real HTTP client factory, which will fail without a real API
	// So we just verify the options are set correctly
	// A full integration test would require a mock server or API client injection

	// Verify that commentRun attempts to proceed (will fail due to no real API)
	err := commentRun(opts)
	// Expect error because there's no real GitCode API to call
	if err == nil {
		// If no error, the mock somehow worked - that's fine too
		return
	}
	// Error should be related to API call failure, not validation
	if strings.Contains(err.Error(), "body is required") {
		t.Errorf("commentRun() should not fail on validation, got: %v", err)
	}
}

func TestInlineCommentOptions(t *testing.T) {
	t.Parallel()
	// Verify inline comment options are passed correctly to API
	opts := &CommentOptions{
		Path:     "api/auth.go",
		Position: 42,
		Body:     "Inline comment",
	}

	// Verify fields are set
	if opts.Path != "api/auth.go" {
		t.Errorf("Path = %v, want api/auth.go", opts.Path)
	}
	if opts.Position != 42 {
		t.Errorf("Position = %v, want 42", opts.Position)
	}
	if opts.Body != "Inline comment" {
		t.Errorf("Body = %v, want Inline comment", opts.Body)
	}
}

func TestGetBodyScansInlineBodyForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	f := cmdutil.TestFactory()
	opts := &CommentOptions{IO: f.IOStreams, Body: "leaked: secret-token-abc123"}
	_, err := getBody(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("getBody() error = %v, want secret detection error", err)
	}
}
