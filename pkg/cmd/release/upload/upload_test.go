package upload

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestUploadRun(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	filePath := filepath.Join(t.TempDir(), "asset.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/tags/v1.0.0"):
			// Same-name precheck: the release has no assets yet.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/v1.0.0/upload_url"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://uploads.gitcode.test/upload/asset.txt","headers":{"X-Test":"1"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/upload/asset.txt":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	err := uploadRun(&UploadOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Files:      []string{filePath},
	})
	if err != nil {
		t.Fatalf("uploadRun() error = %v", err)
	}

	if !strings.Contains(out.String(), "Uploaded asset.txt") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestUploadRunJSONWritesUploadedFiles(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	filePath := filepath.Join(t.TempDir(), "asset.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/tags/v1.0.0"):
			// Same-name precheck: the release has no assets yet.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/v1.0.0/upload_url"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://uploads.gitcode.test/upload/asset.txt","headers":{"X-Test":"1"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/upload/asset.txt":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	err := uploadRun(&UploadOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Files:      []string{filePath},
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("uploadRun() error = %v", err)
	}

	var got []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out.String())
	}
	if len(got) != 1 || got[0]["name"] != "asset.txt" || got[0]["size"] != float64(5) || got[0]["content_type"] != "text/plain" {
		t.Fatalf("JSON output = %#v", got)
	}
	if strings.Contains(out.String(), "Uploaded asset.txt") {
		t.Fatalf("JSON output contains text banner: %q", out.String())
	}
}

func TestUploadRunRejectsUnsupportedLabel(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := testutil.NewTestIOStreams()
	err := uploadRun(&UploadOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return nil, errors.New("should not be called") },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Files:      []string{"asset.txt"},
		Label:      "linux-amd64",
	})
	if err == nil {
		t.Fatal("uploadRun() error = nil, want usage error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
}

func TestUploadRunRefusesExistingAssetNames(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	filePath := filepath.Join(t.TempDir(), "asset.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	putCount := 0
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/tags/v1.0.0"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[{"name":"asset.txt"}]}`))
		case r.Method == http.MethodPut:
			putCount++
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	err := uploadRun(&UploadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Files:      []string{filePath},
	})
	if err == nil || !strings.Contains(err.Error(), "already has asset(s): asset.txt") || !strings.Contains(err.Error(), "delete-asset") {
		t.Fatalf("error = %v, want same-name refusal with delete-asset guidance", err)
	}
	if putCount != 0 {
		t.Errorf("PUT count = %d, want 0 (refuse before any upload)", putCount)
	}
}

func TestUploadRunNamesPartialSuccessOnFailure(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	second := filepath.Join(dir, "b.txt")
	for _, f := range []string{first, second} {
		if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/tags/v1.0.0"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/v1.0.0/upload_url"):
			if r.URL.Query().Get("file_name") == "b.txt" {
				// Second file's upload URL lookup fails.
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://uploads.gitcode.test/upload/a.txt","headers":{"X-Test":"1"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/upload/a.txt":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	err := uploadRun(&UploadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Files:      []string{first, second},
	})
	if err == nil || !strings.Contains(err.Error(), "already uploaded in this run: a.txt") {
		t.Fatalf("error = %v, want partial-success list in the error", err)
	}
}

func TestUploadRunStreamsFileContentIntact(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "payload.bin")
	payload := strings.Repeat("x", 4096)
	if err := os.WriteFile(filePath, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	var putBody string
	var putLength int64
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/tags/v1.0.0"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/v1.0.0/upload_url"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://uploads.gitcode.test/upload/payload.bin","headers":{}}`))
		case r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)
			putLength = r.ContentLength
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	err := uploadRun(&UploadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v1.0.0",
		Files:      []string{filePath},
	})
	if err != nil {
		t.Fatalf("uploadRun() error = %v", err)
	}
	if putBody != payload {
		t.Errorf("uploaded body length = %d, want %d (streaming must not truncate)", len(putBody), len(payload))
	}
	if putLength != int64(len(payload)) {
		t.Errorf("Content-Length = %d, want %d", putLength, int64(len(payload)))
	}
}

func TestUploadRunReleaseNotFoundIsWrapped(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	filePath := filepath.Join(t.TempDir(), "asset.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	err := uploadRun(&UploadOptions{
		IO:         ioStreams,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		TagName:    "v9.9.9",
		Files:      []string{filePath},
	})
	if err == nil || !strings.Contains(err.Error(), "release v9.9.9 not found in owner/repo") {
		t.Fatalf("error = %v, want wrapped not-found", err)
	}
	if code := cmdutil.ExitCode(err); code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func TestUploadRunRefusesDirectoryAndBatchDuplicates(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, _, _ := iostreams.Test()
	dir := t.TempDir()
	inner := filepath.Join(dir, "sub")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	twinA := filepath.Join(dir, "a", "tool.txt")
	twinB := filepath.Join(dir, "b", "tool.txt")
	for _, f := range []string{twinA, twinB} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v5/repos/owner/repo/releases/tags/v1.0.0") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	options := func(files ...string) *UploadOptions {
		return &UploadOptions{
			IO:         ioStreams,
			HttpClient: func() (*http.Client, error) { return client, nil },
			Repository: "owner/repo",
			TagName:    "v1.0.0",
			Files:      files,
		}
	}

	// A directory argument is rejected by the stat check.
	if err := uploadRun(options(inner)); err == nil || !strings.Contains(err.Error(), "cannot upload a directory") {
		t.Errorf("directory error = %v, want cannot-upload-a-directory", err)
	}
	// Two files with the same basename conflict within the batch.
	if err := uploadRun(options(twinA, twinB)); err == nil ||
		!strings.Contains(err.Error(), "tool.txt (twice in this batch)") {
		t.Errorf("batch duplicate error = %v, want batch-duplicate refusal", err)
	}
}
