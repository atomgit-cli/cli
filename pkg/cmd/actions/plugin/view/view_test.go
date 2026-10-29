package view

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func viewTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewCmdView(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "view with name", args: []string{"checkout", "-R", "owner/repo"}, wantErr: false},
		{name: "view with json", args: []string{"checkout", "-R", "owner/repo", "--json"}, wantErr: false},
		{name: "view missing name", args: []string{"-R", "owner/repo"}, wantErr: false},
		{name: "view no args", args: []string{}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdView(f, func(opts *ViewOptions) error { return nil })
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestViewRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	detailJSON := `{"name":"checkout","display_name":"Checkout","description":"checkout repo","vision_content":[{"version":"v1","readme":"# Checkout"}],"custom":"preserved"}`
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, detailJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "checkout",
		JSON:       true,
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if result["custom"] != "preserved" {
		t.Fatalf("custom field lost: %v", result["custom"])
	}
	if result["name"] != "checkout" {
		t.Fatalf("name = %v, want checkout", result["name"])
	}
}

func TestViewRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	detailJSON := `{"name":"checkout","display_name":"Checkout","description":"checkout repo","vision_content":[{"version":"v1","readme":"# Checkout\n\nUse this plugin"},{"version":"v2","readme":"# Checkout v2"}]}`
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, detailJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "checkout",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "checkout") {
		t.Fatalf("stdout = %q, want it to contain 'checkout'", out)
	}
	if !strings.Contains(out, "Checkout") {
		t.Fatalf("stdout = %q, want it to contain 'Checkout'", out)
	}
	if !strings.Contains(out, "Version v1") {
		t.Fatalf("stdout = %q, want it to contain 'Version v1'", out)
	}
	if !strings.Contains(out, "Version v2") {
		t.Fatalf("stdout = %q, want it to contain 'Version v2'", out)
	}
	if !strings.Contains(out, "# Checkout") {
		t.Fatalf("stdout = %q, want it to contain README content", out)
	}
}

func TestViewRunV2HostAndBearer(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotHost string
	var gotAuth string
	var gotPath string
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotHost = req.URL.Host
					gotAuth = req.Header.Get("Authorization")
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return viewTestResponse(http.StatusOK, `{}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "checkout",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	if gotHost != "web-api.gitcode.com" {
		t.Fatalf("host = %q, want web-api.gitcode.com", gotHost)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	if !strings.Contains(gotPath, "name=checkout") {
		t.Fatalf("path = %q, want it to contain name=checkout", gotPath)
	}
	if !strings.Contains(gotPath, "actions/plugins/detail") {
		t.Fatalf("path = %q, want it to contain actions/plugins/detail", gotPath)
	}
}

func TestViewRunWritesToFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	detailJSON := `{"name":"checkout","display_name":"Checkout","vision_content":[{"version":"v1","readme":"# Checkout"}]}`
	tmpFile := filepath.Join(t.TempDir(), "test-plugin-detail.json")
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, detailJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "checkout",
		JSON:       true,
		Files:      tmpFile,
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("file content is not valid JSON: %v", err)
	}
	if result["name"] != "checkout" {
		t.Fatalf("file content name = %v, want checkout", result["name"])
	}
	os.Remove(tmpFile)
}

func TestViewRunEmptyVisionContent(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	detailJSON := `{"name":"checkout","display_name":"Checkout","description":"checkout repo","vision_content":[]}`
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, detailJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "checkout",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Versions:\t0") {
		t.Fatalf("stdout = %q, want 'Versions:\\t0'", stdout.String())
	}
}

func TestViewRunNoReadme(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, stdout, _ := iostreams.Test()
	detailJSON := `{"name":"checkout","vision_content":[{"version":"v1","readme":""}]}`
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, detailJSON), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "checkout",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "no README available") {
		t.Fatalf("stdout = %q, want 'no README available'", stdout.String())
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
					return viewTestResponse(http.StatusNotFound, `{"message":"plugin not found"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		PluginName: "nonexistent",
	}

	err := viewRun(opts)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "failed to view actions plugin") {
		t.Fatalf("error = %v, want it to contain 'failed to view actions plugin'", err)
	}
}
