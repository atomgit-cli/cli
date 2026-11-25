package delete

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

func TestNewCmdDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "delete with id", args: []string{"art-1"}, wantErr: false},
		{name: "delete with yes", args: []string{"--yes", "art-1"}, wantErr: false},
		{name: "delete with dry-run", args: []string{"--dry-run", "art-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdDelete(f, func(opts *DeleteOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteRunDryRun(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatal("API should not be called in dry-run")
					return nil, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		DryRun:     true,
	}

	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "Dry run") {
		t.Fatalf("output = %q, want 'Dry run'", out.String())
	}
}

func TestDeleteRunDryRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatal("API should not be called in dry-run")
					return nil, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		DryRun:     true,
		JSON:       true,
	}

	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if result["action"] != "dry_run" {
		t.Fatalf("action = %v, want dry_run", result["action"])
	}
}

func TestDeleteRunWithYes(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	var gotMethod string
	var gotPath string
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotMethod = req.Method
					gotPath = req.URL.Path
					return deleteTestResponse(http.StatusNoContent, ""), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		Yes:        true,
	}

	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	if gotMethod != "DELETE" {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	wantPath := "/api/v8/repos/owner/repo/actions/artifacts/art-1"
	if gotPath != wantPath {
		t.Fatalf("path = %q, want %q", gotPath, wantPath)
	}
	if !strings.Contains(out.String(), "Deleted") {
		t.Fatalf("output = %q, want 'Deleted'", out.String())
	}
}

func TestDeleteRunWithYesJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return deleteTestResponse(http.StatusNoContent, ""), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		Yes:        true,
		JSON:       true,
	}

	if err := deleteRun(opts); err != nil {
		t.Fatalf("deleteRun() error = %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if result["action"] != "deleted" {
		t.Fatalf("action = %v, want deleted", result["action"])
	}
}

func TestDeleteRunRefusesWithoutYesNonTTY(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test() // non-TTY by default
	apiCalled := false
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					apiCalled = true
					return deleteTestResponse(http.StatusNoContent, ""), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "art-1",
		// no --yes
	}

	err := deleteRun(opts)
	if err == nil {
		t.Fatal("deleteRun() error = nil, want error in non-interactive without --yes")
	}
	if apiCalled {
		t.Fatal("API was called; confirmation gate should refuse before deleting")
	}
}

func TestDeleteRunError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &DeleteOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return deleteTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		ArtifactID: "missing",
		Yes:        true,
	}

	err := deleteRun(opts)
	if err == nil {
		t.Fatal("deleteRun() error = nil, want error")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d", got, cmdutil.ExitNotFound)
	}
}

func deleteTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
