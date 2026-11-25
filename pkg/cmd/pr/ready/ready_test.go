package ready

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "mark ready",
			args:    []string{"123", "--ready"},
			wantErr: false,
		},
		{
			name:    "mark wip",
			args:    []string{"123", "--wip"},
			wantErr: false,
		},
		{
			name:    "mark ready with yes",
			args:    []string{"123", "--ready", "--yes"},
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
			cmd := NewCmdReady(f, func(opts *ReadyOptions) error {
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

func TestReadyRunRequiresConfirmationBeforeUpdate(t *testing.T) {
	t.Setenv("GC_TOKEN", "token")

	var requests []string
	httpClient := &http.Client{Transport: prReadyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		if len(requests) == 1 && req.Method == http.MethodGet {
			return prReadyResponse(http.StatusOK, `{"number":123,"title":"Fix","state":"open"}`), nil
		}
		t.Fatalf("unexpected request before confirmation: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})}

	f := cmdutil.TestFactory()
	err := readyRun(&ReadyOptions{
		IO:         f.IOStreams,
		HttpClient: func() (*http.Client, error) { return httpClient, nil },
		Repository: "owner/repo",
		Number:     123,
		Ready:      true,
	})
	if err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("readyRun() error = %v, want confirmation required", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %#v, want only PR lookup before confirmation", requests)
	}
}

type prReadyRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn prReadyRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func prReadyResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
