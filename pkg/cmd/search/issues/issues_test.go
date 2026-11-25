package issues

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "with query", args: []string{"bug"}, wantErr: false},
		{name: "with repo", args: []string{"-R", "owner/repo", "bug"}, wantErr: false},
		{name: "with state", args: []string{"--state", "open", "bug"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdIssues(cmdutil.TestFactory(), func(opts *IssuesOptions) error {
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

func TestIssuesRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &IssuesOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return issuesTestResponse(http.StatusOK, `[{"number":"42","state":"open","title":"Bug report"}]`), nil
				}),
			}, nil
		},
		Query: "bug",
		Limit: 30,
		JSON:  true,
	}

	if err := issuesRun(opts); err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &results); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0]["title"] != "Bug report" {
		t.Fatalf("title = %v, want Bug report", results[0]["title"])
	}
}

func TestIssuesRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &IssuesOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return issuesTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Query: "nothing",
		Limit: 30,
	}

	if err := issuesRun(opts); err != nil {
		t.Fatalf("issuesRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No issues found") {
		t.Fatalf("output should say no issues; got: %s", out.String())
	}
}

func TestIssuesRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &IssuesOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return issuesTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}),
			}, nil
		},
		Query: "test",
		Limit: 30,
	}

	err := issuesRun(opts)
	if err == nil {
		t.Fatal("issuesRun() error = nil, want error")
	}
}

func issuesTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
