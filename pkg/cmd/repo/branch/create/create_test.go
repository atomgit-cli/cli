package create

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

func TestNewCmdCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "create branch", args: []string{"feature"}, wantErr: false},
		{name: "with ref", args: []string{"--ref", "main", "feature"}, wantErr: false},
		{name: "with description", args: []string{"--description", "test", "feature"}, wantErr: false},
		{name: "with json", args: []string{"--json", "feature"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdCreate(cmdutil.TestFactory(), func(opts *CreateOptions) error {
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

func TestNewCmdCreateFlagsExist(t *testing.T) {
	t.Parallel()
	cmd := NewCmdCreate(cmdutil.TestFactory(), nil)
	for _, flag := range []string{"repo", "ref", "description", "json"} {
		flag := flag
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("flag %q missing", flag)
		}
	}
}

func TestCreateRunBuildsPath(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	var gotPath string
	var gotMethod string
	opts := &CreateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					gotMethod = req.Method
					return createTestResponse(http.StatusOK, `{"name":"feature","protected":false}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		BranchName: "feature",
		Ref:        "main",
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	want := "/api/v5/repos/owner/repo/branches"
	if gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if gotMethod != "POST" {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
}

func TestCreateRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &CreateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return createTestResponse(http.StatusOK, `{"name":"feature","protected":false}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		BranchName: "feature",
		JSON:       true,
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	var branch map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &branch); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if branch["name"] != "feature" {
		t.Fatalf("name = %v, want feature", branch["name"])
	}
}

func TestCreateRunHumanReadable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	opts := &CreateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return createTestResponse(http.StatusOK, `{"name":"feature","protected":false}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		BranchName: "feature",
		Ref:        "main",
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "Created branch feature") {
		t.Fatalf("stderr should contain confirmation; got: %s", errOut.String())
	}
}

func TestCreateRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &CreateOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return createTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		BranchName: "feature",
	}

	err := createRun(opts)
	if err == nil {
		t.Fatal("createRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to create branch") {
		t.Fatalf("error = %q, want to wrap create failure", err.Error())
	}
}

func TestOrDash(t *testing.T) {
	t.Parallel()
	if got := orDash(""); got != "default" {
		t.Errorf("orDash(\"\") = %q, want %q", got, "default")
	}
	if got := orDash("main"); got != "main" {
		t.Errorf("orDash(\"main\") = %q, want %q", got, "main")
	}
}

func createTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
