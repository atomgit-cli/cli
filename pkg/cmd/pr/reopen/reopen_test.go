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
			name:    "reopen PR",
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
			name:    "no PR number",
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
		Repository: "owner/repo",
		Number:     123,
		Comment:    "Reopening",
	})
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("reopenRun() error = %v, want confirmation required", err)
	}
	if requests != 0 {
		t.Fatalf("HTTP requests = %d, want 0 before confirmation", requests)
	}
}

func TestReopenRunAddsCommentAfterReopen(t *testing.T) {
	t.Setenv("GC_TOKEN", "token")

	var requests []string
	f := cmdutil.TestFactory()
	err := reopenRun(&ReopenOptions{
		IO:         f.IOStreams,
		HttpClient: orderedReopenHTTPClient(&requests),
		Repository: "owner/repo",
		Number:     123,
		Comment:    "Reopening",
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("reopenRun() error = %v", err)
	}

	want := []string{
		"GET /repos/owner/repo/pulls/123",
		"PATCH /repos/owner/repo/pulls/123",
		"POST /repos/owner/repo/pulls/123/comments",
	}
	if len(requests) != len(want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("request[%d] = %q, want %q; all requests: %v", i, requests[i], want[i], requests)
		}
	}
}

func testHTTPClient(requests *int) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		return &http.Client{Transport: prReopenRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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

func orderedReopenHTTPClient(requests *[]string) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		return &http.Client{Transport: prReopenRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			path = strings.TrimPrefix(path, "/api/v5")
			*requests = append(*requests, req.Method+" "+path)

			body := `{"number":123,"title":"Test PR","state":"open","html_url":"https://gitcode.com/owner/repo/pulls/123"}`
			if strings.HasSuffix(path, "/comments") {
				body = `{"id":1,"body":"Reopening"}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     http.StatusText(http.StatusOK),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})}, nil
	}
}

type prReopenRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn prReopenRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
