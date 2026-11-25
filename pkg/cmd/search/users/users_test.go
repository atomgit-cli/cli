package users

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

func TestNewCmdUsers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "with query", args: []string{"dev"}, wantErr: false},
		{name: "with json", args: []string{"--json", "dev"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdUsers(cmdutil.TestFactory(), func(opts *UsersOptions) error {
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

func TestUsersRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &UsersOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return usersTestResponse(http.StatusOK, `[{"login":"dev","name":"Developer"}]`), nil
				}),
			}, nil
		},
		Query: "dev",
		Limit: 30,
		JSON:  true,
	}

	if err := usersRun(opts); err != nil {
		t.Fatalf("usersRun() error = %v", err)
	}
	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &results); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0]["login"] != "dev" {
		t.Fatalf("login = %v, want dev", results[0]["login"])
	}
}

func TestUsersRunHumanReadable(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &UsersOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return usersTestResponse(http.StatusOK, `[{"login":"dev","name":"Developer"}]`), nil
				}),
			}, nil
		},
		Query: "dev",
		Limit: 30,
	}

	if err := usersRun(opts); err != nil {
		t.Fatalf("usersRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "dev") {
		t.Fatalf("output should contain login; got: %s", out.String())
	}
}

func TestUsersRunEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &UsersOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return usersTestResponse(http.StatusOK, `[]`), nil
				}),
			}, nil
		},
		Query: "nobody",
		Limit: 30,
	}

	if err := usersRun(opts); err != nil {
		t.Fatalf("usersRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No users found") {
		t.Fatalf("output should say no users; got: %s", out.String())
	}
}

func TestUsersRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &UsersOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return usersTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}),
			}, nil
		},
		Query: "test",
		Limit: 30,
	}

	err := usersRun(opts)
	if err == nil {
		t.Fatal("usersRun() error = nil, want error")
	}
}

func usersTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
