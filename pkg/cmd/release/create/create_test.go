package create

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "create with tag",
			args:    []string{"v1.0.0"},
			wantErr: false,
		},
		{
			name:    "create with tag and title",
			args:    []string{"v1.0.0", "--title", "Version 1.0"},
			wantErr: false,
		},
		{
			name:    "create with draft flag",
			args:    []string{"v1.0.0", "--draft"},
			wantErr: false,
		},
		{
			name:    "create with prerelease flag",
			args:    []string{"v1.0.0-beta", "--prerelease"},
			wantErr: false,
		},
		{
			name:    "create with json output",
			args:    []string{"v1.0.0", "--json"},
			wantErr: false,
		},
		{
			name:    "create with notes-file flag",
			args:    []string{"v1.0.0", "--notes-file", "/tmp/test.md"},
			wantErr: false,
		},
		{
			name:    "no tag specified",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
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

func TestNewCmdCreateNotesAndNotesFileMutualExclusion(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotOpts *CreateOptions
	cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
		gotOpts = opts
		return createRun(opts)
	})
	cmd.SetArgs([]string{"v1.0.0", "--notes", "text", "--notes-file", "/tmp/test.md"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when using both --notes and --notes-file")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Errorf("expected exit code %d, got %d", cmdutil.ExitUsage, cmdutil.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "cannot use both") {
		t.Errorf("error message should contain 'cannot use both', got: %v", err)
	}
	if gotOpts == nil {
		t.Fatal("options were not set")
	}
	if gotOpts.Notes != "text" {
		t.Errorf("Notes = %q, want 'text'", gotOpts.Notes)
	}
	if gotOpts.NotesFile != "/tmp/test.md" {
		t.Errorf("NotesFile = %q, want '/tmp/test.md'", gotOpts.NotesFile)
	}
}

func TestCreateRunRejectsDraftBeforeRemoteWrite(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	called := false
	opts := &CreateOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Draft:      true,
		HttpClient: func() (*http.Client, error) {
			called = true
			return &http.Client{}, nil
		},
	}

	err := createRun(opts)
	if err == nil {
		t.Fatal("expected draft unsupported error")
	}
	if called {
		t.Fatal("HttpClient should not be called when --draft is rejected")
	}
	if !strings.Contains(err.Error(), "--draft is not supported") {
		t.Fatalf("error = %v, want unsupported draft message", err)
	}
}

func TestCreateRunPrereleaseSendsReleaseStatusAndVerifies(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotPostBody string
	var sawGet bool
	opts := &CreateOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0-rc1",
		Title:      "v1.0.0 RC1",
		Prerelease: true,
		JSON:       true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodPost && req.URL.Path == "/api/v5/repos/owner/repo/releases":
						body, err := io.ReadAll(req.Body)
						if err != nil {
							return nil, err
						}
						gotPostBody = string(body)
						return releaseResponse(http.StatusOK, `{"id":1,"tag_name":"v1.0.0-rc1","name":"v1.0.0 RC1","prerelease":false}`), nil
					case req.Method == http.MethodGet && req.URL.Path == "/api/v5/repos/owner/repo/releases/tags/v1.0.0-rc1":
						sawGet = true
						return releaseResponse(http.StatusOK, `{"id":1,"tag_name":"v1.0.0-rc1","name":"v1.0.0 RC1","prerelease":true}`), nil
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			}, nil
		},
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if !strings.Contains(gotPostBody, `"release_status":"pre"`) {
		t.Fatalf("POST body = %s, want release_status pre", gotPostBody)
	}
	if !sawGet {
		t.Fatal("expected createRun to verify prerelease state with GetRelease")
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, string(out))
	}
	if got["prerelease"] != true {
		t.Fatalf("JSON output prerelease = %#v, want true", got["prerelease"])
	}
}

func TestCreateRunNotesFileReadsContent(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	// Create temp file with notes
	tmpFile, err := os.CreateTemp("", "notes*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString("Notes from file"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Title:      "Version 1.0.0",
		NotesFile:  tmpFile.Name(),
		JSON:       true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost || req.URL.Path != "/api/v5/repos/owner/repo/releases" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					return releaseResponse(http.StatusOK, `{"id":1,"tag_name":"v1.0.0","name":"Version 1.0.0","html_url":"https://gitcode.com/owner/repo/releases/v1.0.0"}`), nil
				}),
			}, nil
		},
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, string(out))
	}
}

func TestCreateRunNotesFileNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		NotesFile:  "/nonexistent/file.md",
		HttpClient: func() (*http.Client, error) {
			return &http.Client{}, nil
		},
	}

	err := createRun(opts)
	if err == nil {
		t.Fatal("expected error when notes file not found")
	}
	if !strings.Contains(err.Error(), "failed to read notes file") {
		t.Errorf("error should contain 'failed to read notes file', got: %v", err)
	}
}

func TestCreateRunJSONWritesCreatedRelease(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Title:      "Version 1.0.0",
		Notes:      "Release notes",
		JSON:       true,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost || req.URL.Path != "/api/v5/repos/owner/repo/releases" {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					}
					return releaseResponse(http.StatusOK, `{"id":1,"tag_name":"v1.0.0","name":"Version 1.0.0","html_url":"https://gitcode.com/owner/repo/releases/v1.0.0"}`), nil
				}),
			}, nil
		},
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, string(out))
	}
	if got["tag_name"] != "v1.0.0" || got["html_url"] != "https://gitcode.com/owner/repo/releases/v1.0.0" {
		t.Fatalf("JSON output = %#v", got)
	}
	if strings.Contains(string(out), "Created release") {
		t.Fatalf("JSON output contains text banner: %q", string(out))
	}
}

type releaseRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn releaseRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func releaseResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCreateRunScansInlineNotesForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Notes:      "leaked: secret-token-abc123",
	}
	err := createRun(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("createRun() error = %v, want secret detection error", err)
	}
}
