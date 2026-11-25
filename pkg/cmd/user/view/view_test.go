package view

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

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: false},
		{name: "with username", args: []string{"dev"}, wantErr: false},
		{name: "with json", args: []string{"--json"}, wantErr: false},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCmdView(cmdutil.TestFactory(), func(opts *ViewOptions) error {
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

func TestNewCmdViewFlagsExist(t *testing.T) {
	t.Parallel()
	cmd := NewCmdView(cmdutil.TestFactory(), nil)
	if cmd.Flags().Lookup("json") == nil {
		t.Fatal("json flag missing")
	}
}

func TestViewRunCurrentUser(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, `{"login":"dev","name":"Developer","email":"dev@test.com","bio":"Test bio","company":"Acme","type":"User","html_url":"https://gitcode.com/dev","followers":10,"following":5}`), nil
				}),
			}, nil
		},
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"dev", "Developer", "Test bio", "Acme"} {
		want := want
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; output=%s", want, got)
		}
	}
}

func TestViewRunByUsername(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	var gotPath string
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					return viewTestResponse(http.StatusOK, `{"login":"other","name":"Other User"}`), nil
				}),
			}, nil
		},
		Username: "other",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if gotPath != "/api/v5/users/other" {
		t.Fatalf("path = %q, want %q", gotPath, "/api/v5/users/other")
	}
	if !strings.Contains(out.String(), "other") {
		t.Fatalf("output should contain username; got: %s", out.String())
	}
}

func TestViewRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, `{"login":"dev","name":"Dev","type":"User"}`), nil
				}),
			}, nil
		},
		JSON: true,
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	var user map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &user); err != nil {
		t.Fatalf("output is not valid JSON: %v; raw: %s", err, out.String())
	}
	if user["login"] != "dev" {
		t.Fatalf("login = %v, want dev", user["login"])
	}
}

func TestViewRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Username: "missing",
	}

	err := viewRun(opts)
	if err == nil {
		t.Fatal("viewRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to get user") {
		t.Fatalf("error = %q, want to wrap get user failure", err.Error())
	}
}

func viewTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
