package reopen

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdReopen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "reopen issue",
			args:    []string{"123"},
			wantErr: false,
		},
		{
			name:    "reopen with comment",
			args:    []string{"123", "--comment", "Reopening"},
			wantErr: false,
		},
		{
			name:    "reopen with yes",
			args:    []string{"123", "--yes"},
			wantErr: false,
		},
		{
			name:    "no issue number",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdReopen(f, func(opts *ReopenOptions) error {
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

func TestReopenRunRequiresConfirmationBeforeWrite(t *testing.T) {
	t.Setenv("GC_TOKEN", "token")

	requests := 0
	f := cmdutil.TestFactory()
	err := reopenRun(&ReopenOptions{
		IO:         f.IOStreams,
		HttpClient: testHTTPClient(&requests),
		BaseRepo:   f.BaseRepo,
		Repository: "owner/repo",
		Number:     123,
		Comment:    "Still needed",
	})
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("reopenRun() error = %v, want confirmation required", err)
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0 before confirmation", requests)
	}
}

func testHTTPClient(requests *int) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		return &http.Client{Transport: issueReopenRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			*requests = *requests + 1
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		})}, nil
	}
}

type issueReopenRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn issueReopenRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
