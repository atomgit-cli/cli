package comment

import (
	"bytes"
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
			name:    "no issue number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid issue number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "no body provided",
			args:    []string{"123"},
			wantErr: false, // Command runs, error in run function
		},
		{
			name:    "both body and body-file",
			args:    []string{"123", "--body", "test", "--body-file", "-"},
			wantErr: false, // Command runs, error in run function
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
				if tt.errContain != "" && !contains(err.Error(), tt.errContain) {
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

func TestParseRepo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		repo      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"", "", "", true},
		{"invalid", "", "", true},
		{"too/many/parts", "", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.repo, func(t *testing.T) {
			t.Parallel()
			owner, repo, err := parseRepo(tt.repo)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseRepo() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseRepo() unexpected error: %v", err)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("parseRepo() owner = %v, want %v", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseRepo() repo = %v, want %v", repo, tt.wantRepo)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
