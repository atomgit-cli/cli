package view

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdView(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view with tag",
			args:    []string{"v1.0.0"},
			wantErr: false,
		},
		{
			name:    "view with web flag",
			args:    []string{"v1.0.0", "--web"},
			wantErr: false,
		},
		{
			name:    "no tag specified",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			cmd := NewCmdView(f, func(opts *ViewOptions) error {
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

func TestAssetSizeLabel(t *testing.T) {
	if got := assetSizeLabel(api.ReleaseAsset{Size: 0}); got != "unknown size" {
		t.Fatalf("assetSizeLabel() = %q", got)
	}
	if got := assetSizeLabel(api.ReleaseAsset{Size: 42}); got != "42 bytes" {
		t.Fatalf("assetSizeLabel() = %q", got)
	}
}

func viewReleaseJSON(name string) string {
	return `{"tag_name":"v1.0.0","name":"` + name + `","body":"release notes","html_url":"https://gitcode.com/owner/repo/-/releases/v1.0.0","draft":false,"prerelease":false,"assets":[{"name":"a.zip","size":10,"download_count":2}]}`
}

func viewTestOptions(t *testing.T, streams *iostreams.IOStreams, web bool, openBrowser func(string) error, status int, body string) *ViewOptions {
	t.Helper()
	return &ViewOptions{
		IO: streams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: status,
					Status:     http.StatusText(status),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			})}, nil
		},
		Repository:  "owner/repo",
		TagName:     "v1.0.0",
		Web:         web,
		OpenBrowser: openBrowser,
	}
}

func TestViewRunWebOpensTheReleaseURL(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	var opened string
	opts := viewTestOptions(t, io, true, func(url string) error { opened = url; return nil }, http.StatusOK, viewReleaseJSON("rel"))
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if opened != "https://gitcode.com/owner/repo/-/releases/v1.0.0" {
		t.Errorf("opened = %q", opened)
	}
	if !strings.Contains(out.String(), "Opening") {
		t.Errorf("output = %q, want Opening line", out.String())
	}
	if strings.Contains(out.String(), "Tag:") {
		t.Errorf("output = %q, want no human render after --web", out.String())
	}
}

func TestViewRunWebBrowserErrorNonTTYWarnsOnly(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, errOut := iostreams.Test()
	opts := viewTestOptions(t, io, true, func(string) error { return errors.New("no browser") }, http.StatusOK, viewReleaseJSON("rel"))
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v, want nil (non-TTY degrades to a warning)", err)
	}
	if !strings.Contains(errOut.String(), "Failed to open browser") {
		t.Errorf("stderr = %q, want browser failure warning", errOut.String())
	}
}

func TestViewRunWebBrowserErrorOnTTYReturnsError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.TestTTY()
	opts := viewTestOptions(t, io, true, func(string) error { return errors.New("no browser") }, http.StatusOK, viewReleaseJSON("rel"))
	if err := viewRun(opts); err == nil {
		t.Fatal("viewRun() error = nil, want the browser error on a TTY")
	}
}

func TestViewRunJSONEmitsTheRelease(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := viewTestOptions(t, io, false, nil, http.StatusOK, viewReleaseJSON("rel"))
	opts.JSON = true
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if parsed["tag_name"] != "v1.0.0" || parsed["html_url"] != "https://gitcode.com/owner/repo/-/releases/v1.0.0" {
		t.Errorf("JSON = %v", parsed)
	}
}

func TestViewRunHumanRenderFallsBackToTagWhenNameEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, out, _ := iostreams.Test()
	opts := viewTestOptions(t, io, false, nil, http.StatusOK, viewReleaseJSON(""))
	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"v1.0.0\n", "Tag: v1.0.0", "Status: ", "URL: https://gitcode.com/owner/repo/-/releases/v1.0.0", "release notes", "Assets:", "a.zip"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestViewRunNotFoundIsWrapped(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	opts := viewTestOptions(t, io, false, nil, http.StatusNotFound, ``)
	err := viewRun(opts)
	if err == nil || !strings.Contains(err.Error(), "release v1.0.0 not found in owner/repo") {
		t.Fatalf("error = %v, want wrapped not-found", err)
	}
	if code := cmdutil.ExitCode(err); code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}
