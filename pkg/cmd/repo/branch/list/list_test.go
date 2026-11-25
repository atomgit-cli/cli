package list

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

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: false},
		{name: "with json", args: []string{"--json"}, wantErr: false},
		{name: "with limit", args: []string{"--limit", "5"}, wantErr: false},
		{name: "with format", args: []string{"--format", "table"}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdList(cmdutil.TestFactory(), func(opts *ListOptions) error {
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

func TestNewCmdListFlagsExist(t *testing.T) {
	t.Parallel()
	cmd := NewCmdList(cmdutil.TestFactory(), nil)
	for _, flag := range []string{"repo", "limit", "page", "json", "format"} {
		flag := flag
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("flag %q missing", flag)
		}
	}
}

func TestListRunBuildsPath(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					return listTestResponse(http.StatusOK, `[{"name":"main","protected":false,"default_branch":true}]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	want := "/api/v5/repos/owner/repo/branches"
	if gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func TestListRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `[{"name":"main","protected":true,"default_branch":true},{"name":"dev","protected":false}]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
		JSON:       true,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	var branches []map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &branches); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if len(branches) != 2 {
		t.Fatalf("len(branches) = %d, want 2", len(branches))
	}
}

func TestListRunHumanReadable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `[{"name":"main","protected":false,"default_branch":true},{"name":"dev","protected":false}]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "main") {
		t.Errorf("output should contain 'main'; got: %s", got)
	}
	if !strings.Contains(got, "dev") {
		t.Errorf("output should contain 'dev'; got: %s", got)
	}
}

func TestListRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	if err := listRun(opts); err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No branches found") {
		t.Fatalf("output should say no branches; got: %s", out.String())
	}
}

func TestListRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ListOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return listTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	}

	err := listRun(opts)
	if err == nil {
		t.Fatal("listRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to list branches") {
		t.Fatalf("error = %q, want to wrap list failure", err.Error())
	}
}

func listTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
