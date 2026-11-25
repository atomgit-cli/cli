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
		{name: "with name", args: []string{"--name", "New"}, wantErr: false},
		{name: "with bio", args: []string{"--bio", "test"}, wantErr: false},
		{name: "with json", args: []string{"--name", "New", "--json"}, wantErr: false},
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
	for _, flag := range []string{"name", "bio", "email", "company", "location", "website", "json"} {
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
					return editTestResponse(http.StatusOK, `{"login":"dev","name":"New Name"}`), nil
				}),
			}, nil
		},
		Name: "New Name",
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if gotMethod != "PATCH" {
		t.Fatalf("method = %q, want PATCH", gotMethod)
	}
	if !strings.Contains(errOut.String(), "Updated profile for dev") {
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
					return editTestResponse(http.StatusOK, `{"login":"dev","name":"Updated"}`), nil
				}),
			}, nil
		},
		Bio:  "New bio",
		JSON: true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	var user map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &user); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if user["login"] != "dev" {
		t.Fatalf("login = %v, want dev", user["login"])
	}
}

func TestEditRunNoFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return editTestResponse(http.StatusOK, `{}`), nil
			})}, nil
		},
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want usage error for no fields")
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
		Name: "New",
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to update user") {
		t.Fatalf("error = %q, want to wrap update failure", err.Error())
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

func TestEditRunSecretScanRejection(t *testing.T) {
	t.Setenv("GC_TOKEN", "leaked-secret-token-xyz")
	io, _, _, _ := iostreams.Test()
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatal("HTTP request should not be made when secret is detected")
				return nil, nil
			})}, nil
		},
		Bio: "my token is leaked-secret-token-xyz",
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error for secret in bio")
	}
}
