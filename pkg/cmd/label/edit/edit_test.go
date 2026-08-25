package edit

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdEdit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "rename", args: []string{"bug", "--new-name", "bug-fix", "-R", "o/r"}, wantErr: false},
		{name: "color", args: []string{"bug", "--color", "#ff0000", "-R", "o/r"}, wantErr: false},
		{name: "json", args: []string{"bug", "--new-name", "x", "--json", "-R", "o/r"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdEdit(f, func(opts *EditOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEditRunUpdatesNameAndColor(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	var capturedMethod, capturedPath, capturedQuery string
	opts := &EditOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Name:       "bug",
		NewName:    "bug-fix",
		Color:      "#ff0000",
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					capturedMethod = req.Method
					capturedPath = req.URL.Path
					capturedQuery = req.URL.RawQuery
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"id":12738100,"name":"bug-fix","color":"#ff0000"}`),
					}, nil
				}),
			}, nil
		},
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if capturedMethod != "PATCH" {
		t.Errorf("method = %q, want PATCH", capturedMethod)
	}
	if !strings.Contains(capturedPath, "/labels/bug") {
		t.Errorf("path = %q, want contains /labels/bug", capturedPath)
	}
	q, _ := url.ParseQuery(capturedQuery)
	if q.Get("name") != "bug-fix" {
		t.Errorf("query name = %q, want bug-fix", q.Get("name"))
	}
	if q.Get("color") != "#ff0000" {
		t.Errorf("query color = %q, want #ff0000", q.Get("color"))
	}
	if !strings.Contains(out.String(), "Updated label bug-fix") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestEditRunColorOnly(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	var capturedQuery string
	opts := &EditOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Name:       "bug",
		Color:      "#00ff00",
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					capturedQuery = req.URL.RawQuery
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"id":12738100,"name":"bug","color":"#00ff00"}`),
					}, nil
				}),
			}, nil
		},
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	q, _ := url.ParseQuery(capturedQuery)
	if _, ok := q["name"]; ok {
		t.Errorf("query should not contain name when only --color is set, got %q", capturedQuery)
	}
	if q.Get("color") != "#00ff00" {
		t.Errorf("query color = %q, want #00ff00", q.Get("color"))
	}
	// output uses API-returned label.Name when --new-name is not set
	if !strings.Contains(out.String(), "Updated label bug") {
		t.Fatalf("output = %q, want label.Name 'bug'", out.String())
	}
}

func TestEditRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	opts := &EditOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Name:       "bug",
		NewName:    "bug-fix",
		JSON:       true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"id":12738100,"name":"bug-fix","color":"#ff0000"}`),
					}, nil
				}),
			}, nil
		},
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	if !strings.Contains(out.String(), `"name": "bug-fix"`) {
		t.Fatalf("json output = %q", out.String())
	}
}

func TestEditRunRequiresNewNameOrColor(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Name:       "bug",
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
	}
	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want usage error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d (ExitUsage)", got, cmdutil.ExitUsage)
	}
}

func TestEditRunInvalidColorFormat(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Name:       "bug",
		Color:      "red",
		HttpClient: func() (*http.Client, error) { return &http.Client{}, nil },
	}
	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want usage error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d (ExitUsage)", got, cmdutil.ExitUsage)
	}
	if !strings.Contains(err.Error(), "hex format") {
		t.Fatalf("error = %q, want hex format hint", err.Error())
	}
}

func TestEditRunAPIErrorReturnsError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	f.IOStreams.Out = &strings.Builder{}
	opts := &EditOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		Name:       "missing-label",
		NewName:    "x",
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       ioNopCloser(`{"message":"label not found"}`),
					}, nil
				}),
			}, nil
		},
	}
	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to update label") {
		t.Fatalf("error = %q, want failed to update label", err.Error())
	}
}

func ioNopCloser(body string) *readCloser {
	return &readCloser{Reader: strings.NewReader(body)}
}

type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error { return nil }
