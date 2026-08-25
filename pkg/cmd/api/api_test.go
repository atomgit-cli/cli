package api

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdAPIParsesFlags(t *testing.T) {
	t.Parallel()
	cmd := NewCmdAPI(cmdutil.TestFactory(), func(opts *Options) error {
		if opts.Endpoint != "repos/owner/repo" {
			t.Fatalf("Endpoint = %q", opts.Endpoint)
		}
		if opts.Method != "PATCH" {
			t.Fatalf("Method = %q", opts.Method)
		}
		if len(opts.Headers) != 1 || opts.Headers[0] != "X-Test: yes" {
			t.Fatalf("Headers = %#v", opts.Headers)
		}
		if opts.Input != "body.json" {
			t.Fatalf("Input = %q", opts.Input)
		}
		return nil
	})
	cmd.SetArgs([]string{"repos/owner/repo", "--method", "PATCH", "--header", "X-Test: yes", "--input", "body.json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestRunCallsRawAPI(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, in, out, _ := testutil.NewTestIOStreams()
	in.WriteString(`{"title":"updated"}`)
	var gotPath string
	var gotMethod string
	var gotAuth string
	var gotHeader string
	var gotBody string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotHeader = r.Header.Get("X-Test")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/pulls/1",
		Method:     "patch",
		Headers:    []string{"X-Test: yes"},
		Input:      "-",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if gotPath != "/api/v5/repos/owner/repo/pulls/1" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q", gotMethod)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotHeader != "yes" {
		t.Fatalf("X-Test = %q", gotHeader)
	}
	if gotBody != `{"title":"updated"}` {
		t.Fatalf("body = %q", gotBody)
	}
	if strings.TrimSpace(out.String()) != `{"ok":true}` {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunDefaultsToPostWhenInputProvided(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, in, out, _ := testutil.NewTestIOStreams()
	in.WriteString(`{"title":"created"}`)
	var gotMethod string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     http.MethodGet,
		Input:      "-",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if strings.TrimSpace(out.String()) != `{"ok":true}` {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunRejectsForeignURLHost(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.String())
	}))

	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "https://example.com/api/v5/repos/owner/repo",
		Method:     http.MethodGet,
	})
	if err == nil || !strings.Contains(err.Error(), "api endpoint host") {
		t.Fatalf("run() error = %v, want host rejection", err)
	}
}

func TestParseHeadersRejectsInvalidHeader(t *testing.T) {
	t.Parallel()
	_, err := parseHeaders([]string{"missing-colon"})
	if err == nil || !strings.Contains(err.Error(), "--header") {
		t.Fatalf("parseHeaders() error = %v, want host rejection", err)
	}
}

func TestReadInputEmptyReturnsNil(t *testing.T) {
	t.Parallel()
	r, err := readInput(&Options{Input: ""})
	if err != nil {
		t.Fatalf("readInput() error = %v", err)
	}
	if r != nil {
		t.Errorf("readInput() = %v, want nil", r)
	}
}

func TestReadInputStreamsFile(t *testing.T) {
	t.Parallel()
	tmpFile, err := os.CreateTemp("", "input*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer os.Remove(tmpFile.Name())
	want := `{"k":"v"}`
	if _, err := tmpFile.WriteString(want); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	tmpFile.Close()

	r, err := readInput(&Options{Input: tmpFile.Name()})
	if err != nil {
		t.Fatalf("readInput() error = %v", err)
	}
	if r == nil {
		t.Fatal("readInput() returned nil reader")
	}
	defer func() {
		if rc, ok := r.(io.Closer); ok {
			_ = rc.Close()
		}
	}()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != want {
		t.Errorf("readInput() data = %q, want %q", string(data), want)
	}
}
