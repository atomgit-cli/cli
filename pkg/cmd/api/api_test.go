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
	_, err := parseHeaders([]string{"missing-colon"})
	if err == nil || !strings.Contains(err.Error(), "--header") {
		t.Fatalf("parseHeaders() error = %v, want host rejection", err)
	}
}

func TestReadInputEmptyReturnsNil(t *testing.T) {
	r, err := readInput(&Options{Input: ""})
	if err != nil {
		t.Fatalf("readInput() error = %v", err)
	}
	if r != nil {
		t.Errorf("readInput() = %v, want nil", r)
	}
}

func TestReadInputStreamsFile(t *testing.T) {
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

func TestNewCmdAPIParsesNewFlags(t *testing.T) {
	cmd := NewCmdAPI(cmdutil.TestFactory(), func(opts *Options) error {
		if !opts.Include || !opts.Paginate || !opts.Slurp {
			t.Fatalf("flags = include:%v paginate:%v slurp:%v", opts.Include, opts.Paginate, opts.Slurp)
		}
		if opts.JQ != ".login" {
			t.Fatalf("JQ = %q", opts.JQ)
		}
		if len(opts.Fields) != 2 || opts.Fields[0] != "k1=v1" || opts.Fields[1] != "k2=v2" {
			t.Fatalf("Fields = %#v", opts.Fields)
		}
		return nil
	})
	cmd.SetArgs([]string{"user", "--include", "--paginate", "--slurp", "--jq", ".login", "-f", "k1=v1", "-f", "k2=v2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestRunIncludeOutputsResponseHeaders(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("X-Ratelimit-Remaining", "42")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
		Include:    true,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "HTTP/1.1 200 OK\n") {
		t.Fatalf("output does not start with status line: %q", got)
	}
	if !strings.Contains(got, "Etag: \"abc123\"\n") {
		t.Fatalf("output missing ETag header: %q", got)
	}
	if !strings.Contains(got, "X-Ratelimit-Remaining: 42\n") {
		t.Fatalf("output missing rate limit header: %q", got)
	}
	if !strings.Contains(got, "\n\n{\"ok\":true}\n") {
		t.Fatalf("output missing blank line then body: %q", got)
	}
}

func TestRun304ExitsNonZeroWithHeadersIncluded(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
		Include:    true,
	})
	if err == nil {
		t.Fatal("run() 304 error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "304") {
		t.Fatalf("error = %q, want 304 mention", err.Error())
	}
	if !strings.Contains(out.String(), "HTTP/1.1 304") || !strings.Contains(out.String(), "Etag: \"abc123\"") {
		t.Fatalf("stdout = %q, want 304 headers before the error", out.String())
	}
}

func TestRun304ExitsNonZeroWithoutInclude(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
	})
	if err == nil {
		t.Fatal("run() 304 error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "304") {
		t.Fatalf("error = %q, want 304 mention", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty without --include", out.String())
	}
}

func TestRunPaginatesUntilEmptyPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	var paths []string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			_, _ = w.Write([]byte(`[{"n":1},{"n":2}]`))
		case "2":
			_, _ = w.Write([]byte(`[{"n":3}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues?per_page=2",
		Method:     "GET",
		Paginate:   true,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	// Page 1, page 2, then the terminating empty page 3.
	if len(paths) != 3 {
		t.Fatalf("requests = %d (%v), want 3", len(paths), paths)
	}
	if !strings.Contains(paths[1], "page=2") || !strings.Contains(paths[1], "per_page=2") {
		t.Fatalf("second request query = %q, want page=2 and preserved per_page=2", paths[1])
	}
	got := out.String()
	for _, want := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q missing item %s", got, want)
		}
	}
	if strings.Contains(got, "[]") {
		t.Fatalf("output %q should not contain the empty terminating page", got)
	}
}

func TestRunPaginateInjectsDefaultPerPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	var gotQuery string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[]`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(gotQuery, "per_page=100") || !strings.Contains(gotQuery, "page=1") {
		t.Fatalf("first query = %q, want page=1 per_page=100 defaults", gotQuery)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty for empty first page", out.String())
	}
}

func TestRunPaginateSlurpMergesPages(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			_, _ = w.Write([]byte(`[{"n":1},{"n":2}]`))
		case "2":
			_, _ = w.Write([]byte(`[{"n":3},{"n":4}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues?per_page=2",
		Method:     "GET",
		Paginate:   true,
		Slurp:      true,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := "[{\"n\":1},{\"n\":2},{\"n\":3},{\"n\":4}]\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunPaginateRequiresGET(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "POST",
		Paginate:   true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunPaginateRejectsInput(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Input:      "body.json",
		Paginate:   true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunSlurpRequiresPaginate(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Slurp:      true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunPaginateNonArrayResponse(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"not a list"}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
		Paginate:   true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want error for non-array response")
	}
	if !strings.Contains(err.Error(), "array") {
		t.Fatalf("error = %q, want array mention", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestRunPaginateNullBodyErrors(t *testing.T) {
	// A null body decodes as a nil slice without error, which would silently
	// terminate pagination and mask server-side errors (issue 650).
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`null`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
		Slurp:      true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want error for null body")
	}
	if !strings.Contains(err.Error(), "array") {
		t.Fatalf("error = %q, want array mention", err.Error())
	}
	if !strings.Contains(err.Error(), "empty or null") {
		t.Fatalf("error = %q, want empty-or-null mention", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty (no silent [] output)", out.String())
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitError)
	}
}

func TestRunPaginateEmptyBodyErrors(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write nothing: a zero-byte response body.
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want error for empty body")
	}
	if !strings.Contains(err.Error(), "array") {
		t.Fatalf("error = %q, want array mention", err.Error())
	}
	if !strings.Contains(err.Error(), "empty or null") {
		t.Fatalf("error = %q, want consistent empty-or-null message (same as null body)", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitError)
	}
}

func TestRunPaginateNullOnLaterPageErrors(t *testing.T) {
	// A null body on a later page is the most dangerous variant: the old
	// implementation silently truncated pagination and emitted the partial
	// data as a complete result. The guard runs per page, so a data page
	// followed by a null page must now error (issue 650). In slurp mode the
	// merged data is discarded, matching the page-limit failure semantics.
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	var pages []string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`[{"n":1}]`))
		default:
			_, _ = w.Write([]byte(`null`))
		}
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
		Slurp:      true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want error for null body on a later page")
	}
	if !strings.Contains(err.Error(), "empty or null") {
		t.Fatalf("error = %q, want empty-or-null mention", err.Error())
	}
	if len(pages) != 2 {
		t.Fatalf("requests = %d (%v), want 2 (data page then null page)", len(pages), pages)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty (slurp discards merged data on error)", out.String())
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitError {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitError)
	}
}

func TestRunPaginateStopsAtPageLimit(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	var requests int
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// The endpoint ignores the page parameter and always returns a full page.
		_, _ = w.Write([]byte(`[{"n":1}]`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues?per_page=1",
		Method:     "GET",
		Paginate:   true,
	})
	if err == nil {
		t.Fatalf("run() error = nil, want page limit error after %d requests", requests)
	}
	if !strings.Contains(err.Error(), "page limit") {
		t.Fatalf("error = %q, want page limit mention", err.Error())
	}
	if requests != 100 {
		t.Fatalf("requests = %d, want 100", requests)
	}
}

func TestRunJQFilterStringField(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"login":"octocat","id":1}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "user",
		Method:     "GET",
		JQ:         ".login",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out.String(); got != "octocat\n" {
		t.Fatalf("stdout = %q, want %q", got, "octocat\n")
	}
}

func TestRunJQFilterObjectAndArray(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":10,"title":"a"},{"number":20,"title":"b"}]`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		JQ:         ".[] | .number",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out.String(); got != "10\n20\n" {
		t.Fatalf("stdout = %q, want %q", got, "10\n20\n")
	}
}

func TestRunJQInvalidFilter(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "user",
		Method:     "GET",
		JQ:         ".[",
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunJQWithPaginateFiltersEachPage(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			_, _ = w.Write([]byte(`[{"n":1},{"n":2}]`))
		case "2":
			_, _ = w.Write([]byte(`[{"n":3}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues?per_page=2",
		Method:     "GET",
		Paginate:   true,
		JQ:         ".[] | .n",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out.String(); got != "1\n2\n3\n" {
		t.Fatalf("stdout = %q, want %q", got, "1\n2\n3\n")
	}
}

func TestRunFieldsFormJSONBody(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	var gotMethod, gotContentType, gotBody string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues/1/comments",
		Fields:     []string{"body=hello", "k2=v2"},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST default with fields", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody != `{"body":"hello","k2":"v2"}` {
		t.Fatalf("body = %q, want JSON object of fields", gotBody)
	}
	if got := out.String(); got != "{\"id\":1}\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunFieldsConflictWithInput(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo",
		Fields:     []string{"k=v"},
		Input:      "body.json",
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunFieldsKeepExplicitMethod(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	var gotMethod string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_, _ = w.Write([]byte(`{}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "PATCH",
		MethodSet:  true,
		Fields:     []string{"k=v"},
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH preserved", gotMethod)
	}
}

func TestRunFieldsRejectsMalformedField(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo",
		Fields:     []string{"novalue"},
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunIncludeWithJQ(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"x"`)
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "user",
		Method:     "GET",
		Include:    true,
		JQ:         ".login",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "HTTP/1.1 200 OK\n") || !strings.Contains(got, "Etag: \"x\"\n") {
		t.Fatalf("output missing include block: %q", got)
	}
	if !strings.HasSuffix(got, "\n\noctocat\n") {
		t.Fatalf("output missing filtered body after header block: %q", got)
	}
}

func TestRunSlurpRejectsJQ(t *testing.T) {
	// gh rejects this combination explicitly; silently ignoring --jq
	// would hand consumers unfiltered bulk data.
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
		Slurp:      true,
		JQ:         ".[] | .n",
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunPaginateSlurpEmptyFirstPageOutputsEmptyArray(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues?per_page=2",
		Method:     "GET",
		Paginate:   true,
		Slurp:      true,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out.String(); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q (empty array, not null)", got, "[]\n")
	}
}

func TestRunFieldsRejectsMalformedFieldBeforeAuth(t *testing.T) {
	// Field format validation must happen before authentication so a
	// usage error (exit 2) is reported even without any credentials.
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	os.Unsetenv("GC_TOKEN")
	os.Unsetenv("GITCODE_TOKEN")
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo",
		Fields:     []string{"novalue"},
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d (usage before auth)", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunPaginateRejectsFields(t *testing.T) {
	ioStreams, _, _, _ := testutil.NewTestIOStreams()
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return http.DefaultClient, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
		Fields:     []string{"k=v"},
	})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
}

func TestRunPaginate304(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", `"x"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo/issues",
		Method:     "GET",
		Paginate:   true,
	})
	if err == nil {
		t.Fatal("run() error = nil, want 304 error")
	}
	if !strings.Contains(err.Error(), "304") {
		t.Fatalf("error = %q, want 304 mention", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty without --include", out.String())
	}
}

func TestRun304ConditionalRequestWithIfNoneMatch(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"abc123"` {
			w.Header().Set("Etag", `"abc123"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Etag", `"abc123"`)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	// First request: 200 with ETag. Second: conditional request hits 304.
	first := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
		Include:    true,
	})
	if first != nil {
		t.Fatalf("first run() error = %v", first)
	}
	ioStreams2, _, out2, _ := testutil.NewTestIOStreams()
	second := run(&Options{
		IO:         ioStreams2,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
		Headers:    []string{"If-None-Match: \"abc123\""},
		Include:    true,
	})
	if second == nil {
		t.Fatal("conditional run() error = nil, want 304 error")
	}
	if !strings.Contains(second.Error(), "304") {
		t.Fatalf("error = %q, want 304 mention", second.Error())
	}
	if !strings.Contains(out2.String(), "HTTP/1.1 304") || !strings.Contains(out2.String(), "Etag: \"abc123\"") {
		t.Fatalf("stdout = %q, want 304 headers", out2.String())
	}
	_ = out
}

func TestRunIncludeMultiValueHeader(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1; path=/")
		w.Header().Add("Set-Cookie", "b=2; path=/")
		_, _ = w.Write([]byte(`{}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
		Include:    true,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Set-Cookie: a=1; path=/\n") || !strings.Contains(got, "Set-Cookie: b=2; path=/\n") {
		t.Fatalf("output missing multi-value headers: %q", got)
	}
}

func TestRunJQObjectResultAsCompactJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"user":{"name":"octo","id":7}}`))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "user",
		Method:     "GET",
		JQ:         ".user",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out.String(); got != "{\"id\":7,\"name\":\"octo\"}\n" {
		t.Fatalf("stdout = %q, want compact JSON object line", got)
	}
}

func TestRunBodyWithTrailingNewlineNotDuplicated(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"ok\":true}\n"))
	}))
	err := run(&Options{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Endpoint:   "repos/owner/repo",
		Method:     "GET",
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := out.String(); got != "{\"ok\":true}\n" {
		t.Fatalf("stdout = %q, want single trailing newline", got)
	}
}
