package review

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestReviewRun_Approve(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	reviewCalled := false
	commentCalled := false

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			reviewCalled = true
			if owner != "owner" || repo != "repo" || number != 123 {
				t.Fatalf("unexpected review args: %s/%s #%d", owner, repo, number)
			}
			if opts == nil || opts.Force {
				t.Fatalf("unexpected review opts: %+v", opts)
			}
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			commentCalled = true
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !reviewCalled {
		t.Fatal("expected approve to call ReviewPR")
	}
	if commentCalled {
		t.Fatal("did not expect approve without comment to call CreatePRComment")
	}
	if !strings.Contains(out.String(), "approved PR #123") {
		t.Fatalf("expected approval output, got %q", out.String())
	}
}

func TestReviewRun_ApproveWithComment(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	reviewCalled := false
	commentCalled := false

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
		Comment:    "LGTM",
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			reviewCalled = true
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			commentCalled = true
			if opts.Body != "LGTM" {
				t.Fatalf("unexpected comment body %q", opts.Body)
			}
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !reviewCalled || !commentCalled {
		t.Fatalf("expected approve with comment to call both comment and review, got review=%v comment=%v", reviewCalled, commentCalled)
	}
	if !strings.Contains(out.String(), "LGTM") {
		t.Fatalf("expected comment echoed in output, got %q", out.String())
	}
}

func TestReviewRun_RequestChanges(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	commentCalled := false

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Request:    true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			commentCalled = true
			expected := "[REQUEST CHANGES] Changes requested without detailed feedback."
			if opts.Body != expected {
				t.Fatalf("expected comment body %q, got %q", expected, opts.Body)
			}
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !commentCalled {
		t.Fatal("expected request changes to call CreatePRComment")
	}
	if !strings.Contains(out.String(), "requested changes on PR #123") {
		t.Fatalf("expected request changes output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "does not support REQUEST_CHANGES natively") {
		t.Fatalf("expected degradation note in output, got %q", out.String())
	}
}

func TestReviewRun_RequestChangesWithComment(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	commentCalled := false

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Request:    true,
		Comment:    "Please fix the error handling",
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			commentCalled = true
			expected := "[REQUEST CHANGES] Please fix the error handling"
			if opts.Body != expected {
				t.Fatalf("expected comment body %q, got %q", expected, opts.Body)
			}
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !commentCalled {
		t.Fatal("expected request changes with comment to call CreatePRComment")
	}
	if !strings.Contains(out.String(), "Please fix the error handling") {
		t.Fatalf("expected comment echoed in output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "requested changes on PR #123") {
		t.Fatalf("expected request changes output, got %q", out.String())
	}
}

func TestReviewRun_RequestChangesJSON(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Request:    true,
		Comment:    "Needs work",
		JSON:       true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "requested_changes"`) {
		t.Fatalf("expected JSON action requested_changes, got %q", out.String())
	}
	if !strings.Contains(out.String(), `"comment": "Needs work"`) {
		t.Fatalf("expected JSON comment to contain original user input, got %q", out.String())
	}
	if strings.Contains(out.String(), `[REQUEST CHANGES] Needs work`) {
		t.Fatalf("JSON comment field should NOT contain [REQUEST CHANGES] prefix, got %q", out.String())
	}
}

func TestReviewRun_NoActionReturnsUsageExitCode(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
	})
	if err == nil {
		t.Fatal("expected error for missing review action")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestReviewRun_MissingTokenReturnsAuthExitCode(t *testing.T) {
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	io, _, _, _ := testutil.NewTestIOStreams()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitAuth {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitAuth)
	}
}

func TestNewCmdReview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "approve PR",
			args:    []string{"123", "--approve"},
			wantErr: false,
		},
		{
			name:    "request changes",
			args:    []string{"123", "--request"},
			wantErr: false,
		},
		{
			name:    "review with comment",
			args:    []string{"123", "--comment", "Looks good"},
			wantErr: false,
		},
		{
			name:    "approve with body",
			args:    []string{"123", "--approve", "--comment", "LGTM"},
			wantErr: false,
		},
		{
			name:    "no PR number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc", "--approve"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdReview(cmdutil.TestFactory(), func(opts *ReviewOptions) error {
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

func TestReviewRun_RequestChangesNoCommentJSON(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Request:    true,
		JSON:       true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "requested_changes"`) {
		t.Fatalf("expected JSON action requested_changes, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Changes requested without detailed feedback.") {
		t.Fatalf("expected default comment in JSON output, got %q", out.String())
	}
}

func TestReviewRun_ApproveAndRequestMutuallyExclusive(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
		Request:    true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect approve+request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			t.Fatal("did not expect approve+request to call CreatePRComment")
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for --approve with --request")
	}
	if !strings.Contains(err.Error(), "--approve and --request are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestReviewRun_RequestChangesAPIError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Request:    true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return nil, fmt.Errorf("API error: 500 Internal Server Error")
		},
	})
	if err == nil {
		t.Fatal("expected error when CreatePRComment fails")
	}
	if !strings.Contains(err.Error(), "failed to request changes on PR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewRun_RequestChangesWithCommentFile(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	tmpFile, err := os.CreateTemp("", "comment-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("Fix the bug"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	commentCalled := false

	err = reviewRun(&ReviewOptions{
		IO:          io,
		HttpClient:  func() (*http.Client, error) { return &http.Client{}, nil },
		Repository:  "owner/repo",
		Number:      123,
		Request:     true,
		CommentFile: tmpFile.Name(),
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			commentCalled = true
			expected := "[REQUEST CHANGES] Fix the bug"
			if opts.Body != expected {
				t.Fatalf("expected comment body %q, got %q", expected, opts.Body)
			}
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !commentCalled {
		t.Fatal("expected request changes with comment-file to call CreatePRComment")
	}
	if !strings.Contains(out.String(), "Fix the bug") {
		t.Fatalf("expected comment echoed in output, got %q", out.String())
	}
}

func TestReviewRun_ForceWithRequestReturnsError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Force:      true,
		Request:    true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect force+request to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			t.Fatal("did not expect force+request to call CreatePRComment")
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for --force with --request")
	}
	if !strings.Contains(err.Error(), "--force can only be used with --approve") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewRun_ApproveWithCommentFile(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	tmpFile, err := os.CreateTemp("", "comment-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("Self-check passed"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	reviewCalled := false
	commentCalled := false

	err = reviewRun(&ReviewOptions{
		IO:          io,
		HttpClient:  func() (*http.Client, error) { return &http.Client{}, nil },
		Repository:  "owner/repo",
		Number:      123,
		Approve:     true,
		CommentFile: tmpFile.Name(),
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			reviewCalled = true
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			commentCalled = true
			if opts.Body != "Self-check passed" {
				t.Fatalf("unexpected comment body %q", opts.Body)
			}
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !reviewCalled || !commentCalled {
		t.Fatalf("expected approve with comment-file to call both, got review=%v comment=%v", reviewCalled, commentCalled)
	}
	if !strings.Contains(out.String(), "Self-check passed") {
		t.Fatalf("expected comment echoed in output, got %q", out.String())
	}
}

func TestReviewRun_ForceApproveJSON(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
		Yes:        true,
		Force:      true,
		JSON:       true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			if !opts.Force {
				t.Fatal("expected Force=true")
			}
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			t.Fatal("did not expect force approve to call CreatePRComment")
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "force_approved"`) {
		t.Fatalf("expected JSON action force_approved, got %q", out.String())
	}
}

func TestReviewRun_ApproveJSON(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
		JSON:       true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "approved"`) {
		t.Fatalf("expected JSON action approved, got %q", out.String())
	}
}

func TestReviewRun_CommentOnlyJSON(t *testing.T) {
	t.Parallel()
	io, _, out, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Comment:    "Just a note",
		JSON:       true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			t.Fatal("did not expect comment to call ReviewPR")
			return nil
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return &api.PRComment{Body: opts.Body}, nil
		},
	})
	if err != nil {
		t.Fatalf("reviewRun() unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"action": "commented"`) {
		t.Fatalf("expected JSON action commented, got %q", out.String())
	}
}

func TestReviewRun_ApproveAPIError(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		Repository: "owner/repo",
		Number:     123,
		Approve:    true,
		ReviewPR: func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error {
			return fmt.Errorf("API error: 403 Forbidden")
		},
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return &api.PRComment{}, nil
		},
	})
	if err == nil {
		t.Fatal("expected error when ReviewPR fails")
	}
	if !strings.Contains(err.Error(), "failed to approve PR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewRun_CommentAndCommentFileMutuallyExclusive(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	restoreToken := testutil.SetTestToken()
	defer restoreToken()

	err := reviewRun(&ReviewOptions{
		IO:          io,
		HttpClient:  func() (*http.Client, error) { return &http.Client{}, nil },
		Repository:  "owner/repo",
		Number:      123,
		Comment:     "inline comment",
		CommentFile: "some-file.txt",
		ReviewPR:    func(client *api.Client, owner, repo string, number int, opts *api.ReviewPROptions) error { return nil },
		CreatePRComment: func(client *api.Client, owner, repo string, number int, opts *api.CreatePRCommentOptions) (*api.PRComment, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for --comment with --comment-file")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCmdReview_InvalidNumberReturnsUsageExitCode(t *testing.T) {
	t.Parallel()
	cmd := NewCmdReview(cmdutil.TestFactory(), func(opts *ReviewOptions) error {
		return nil
	})
	cmd.SetArgs([]string{"abc", "--approve"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid number error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestReviewRunScansInlineCommentForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	io, _, _, _ := testutil.NewTestIOStreams()
	opts := &ReviewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		Comment:    "leaked: secret-token-abc123",
	}
	err := reviewRun(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("reviewRun() error = %v, want secret detection error", err)
	}
}
