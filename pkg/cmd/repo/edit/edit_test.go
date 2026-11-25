package edit

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

func TestNewCmdEdit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "with description", args: []string{"--description", "test"}, wantErr: false},
		{name: "with private", args: []string{"--private"}, wantErr: false},
		{name: "with name", args: []string{"--name", "newname"}, wantErr: false},
		{name: "with json", args: []string{"--description", "x", "--json"}, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdEdit(cmdutil.TestFactory(), func(opts *EditOptions) error {
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

func TestNewCmdEditFlagsExist(t *testing.T) {
	t.Parallel()
	cmd := NewCmdEdit(cmdutil.TestFactory(), nil)
	for _, flag := range []string{"repo", "description", "homepage", "default-branch", "name", "private", "public", "json"} {
		flag := flag
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("flag %q missing", flag)
		}
	}
}

func TestEditRunSuccess(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	var gotMethod string
	callCount := 0
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount == 1 {
						gotMethod = req.Method
						return editTestResponse(http.StatusOK, `{}`), nil
					}
					return editTestResponse(http.StatusOK, `{"name":"repo","full_name":"owner/repo","description":"New description"}`), nil
				}),
			}, nil
		},
		Repository:  "owner/repo",
		Description: "New description",
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if gotMethod != "PATCH" {
		t.Fatalf("method = %q, want PATCH", gotMethod)
	}
	if !strings.Contains(errOut.String(), "Updated repository owner/repo") {
		t.Fatalf("stderr should contain confirmation; got: %s", errOut.String())
	}
}

func TestEditRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	callCount := 0
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount == 1 {
						return editTestResponse(http.StatusOK, `{}`), nil
					}
					return editTestResponse(http.StatusOK, `{"name":"repo","full_name":"owner/repo","description":"updated","default_branch":"main"}`), nil
				}),
			}, nil
		},
		Repository:  "owner/repo",
		Description: "updated",
		JSON:        true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if result["name"] != "repo" {
		t.Fatalf("name = %v, want repo", result["name"])
	}
	if result["default_branch"] != "main" {
		t.Fatalf("default_branch = %v, want main", result["default_branch"])
	}
}

func TestEditRunPrivatePublicConflict(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO:         io,
		Repository: "owner/repo",
		Private:    true,
		Public:     true,
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error for --private + --public")
	}
}

func TestEditRunNoFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO:         io,
		Repository: "owner/repo",
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error for no fields")
	}
}

func TestEditRunPrivate(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	stream, _, _, _ := iostreams.Test()
	var gotBody string
	callCount := 0
	opts := &EditOptions{
		IO: stream,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount == 1 {
						body, _ := io.ReadAll(req.Body)
						gotBody = string(body)
						return editTestResponse(http.StatusOK, `{}`), nil
					}
					return editTestResponse(http.StatusOK, `{"name":"repo","private":true,"full_name":"owner/repo"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Private:    true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if !strings.Contains(gotBody, `"private":true`) {
		t.Fatalf("request body should contain private:true; got: %s", gotBody)
	}
}

func TestEditRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return editTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}),
			}, nil
		},
		Repository:  "owner/repo",
		Description: "test",
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to update repository") {
		t.Fatalf("error = %q, want to wrap update failure", err.Error())
	}
}

func TestEditRunSecretScan(t *testing.T) {
	t.Setenv("GC_TOKEN", "leaked-secret-xyz")
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatal("HTTP request should not be made when secret is detected")
				return nil, nil
			})}, nil
		},
		Repository:  "owner/repo",
		Description: "token: leaked-secret-xyz",
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error for secret in description")
	}
}

func editTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestEditRunPublic(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	stream, _, _, _ := iostreams.Test()
	var gotBody string
	callCount := 0
	opts := &EditOptions{
		IO: stream,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount == 1 {
						body, _ := io.ReadAll(req.Body)
						gotBody = string(body)
						return editTestResponse(http.StatusOK, `{}`), nil
					}
					return editTestResponse(http.StatusOK, `{"name":"repo","private":false,"full_name":"owner/repo"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Public:     true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if !strings.Contains(gotBody, `"private":false`) {
		t.Fatalf("request body should contain private:false; got: %s", gotBody)
	}
}

func TestEditRunExitCodes(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	tests := []struct {
		name     string
		opts     *EditOptions
		wantCode int
	}{
		{
			name: "private public conflict",
			opts: &EditOptions{
				IO:         testIO(),
				Repository: "owner/repo",
				Private:    true,
				Public:     true,
			},
			wantCode: cmdutil.ExitUsage,
		},
		{
			name: "no fields",
			opts: &EditOptions{
				IO:         testIO(),
				Repository: "owner/repo",
			},
			wantCode: cmdutil.ExitUsage,
		},
		{
			name: "unauthorized",
			opts: &EditOptions{
				IO:         testIO(),
				Repository: "owner/repo",
				Private:    true,
				HttpClient: func() (*http.Client, error) {
					return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
						return editTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
					})}, nil
				},
			},
			wantCode: cmdutil.ExitAuth,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := editRun(tt.opts)
			if err == nil {
				t.Fatal("editRun() error = nil, want error")
			}
			if got := cmdutil.ExitCode(err); got != tt.wantCode {
				t.Fatalf("ExitCode = %d, want %d", got, tt.wantCode)
			}
		})
	}
}

func testIO() *iostreams.IOStreams {
	io, _, _, _ := iostreams.Test()
	return io
}
