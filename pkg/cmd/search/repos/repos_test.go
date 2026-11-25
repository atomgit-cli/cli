package repos

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

func TestNewCmdRepos(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "with query", args: []string{"gitcode"}, wantErr: false},
		{name: "with json", args: []string{"--json", "gitcode"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdRepos(cmdutil.TestFactory(), func(opts *ReposOptions) error {
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

func TestReposRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ReposOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return reposTestResponse(http.StatusOK, `[{"full_name":"owner/repo","description":"test repo"}]`), nil
				}),
			}, nil
		},
		Query: "gitcode",
		Limit: 30,
		JSON:  true,
	}

	if err := reposRun(opts); err != nil {
		t.Fatalf("reposRun() error = %v", err)
	}
	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &results); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0]["full_name"] != "owner/repo" {
		t.Fatalf("full_name = %v, want owner/repo", results[0]["full_name"])
	}
}

func TestReposRunHumanReadable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ReposOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return reposTestResponse(http.StatusOK, `[{"full_name":"owner/repo","description":"test"}]`), nil
				}),
			}, nil
		},
		Query: "gitcode",
		Limit: 30,
	}

	if err := reposRun(opts); err != nil {
		t.Fatalf("reposRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "owner/repo") {
		t.Fatalf("output should contain repo name; got: %s", out.String())
	}
}

func TestReposRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ReposOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return reposTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Query: "nothing",
		Limit: 30,
	}

	if err := reposRun(opts); err != nil {
		t.Fatalf("reposRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No repositories found") {
		t.Fatalf("output should say no repos; got: %s", out.String())
	}
}

func TestReposRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ReposOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return reposTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}),
			}, nil
		},
		Query: "test",
		Limit: 30,
	}

	err := reposRun(opts)
	if err == nil {
		t.Fatal("reposRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to search repositories") {
		t.Fatalf("error = %q, want to wrap search failure", err.Error())
	}
}

func reposTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
