package view

import (
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
		{name: "view run", args: []string{"run-1"}, wantErr: false},
		{name: "view with json", args: []string{"--json", "run-1"}, wantErr: false},
		{name: "no args", args: []string{}, wantErr: true},
		{name: "too many args", args: []string{"a", "b"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestNewCmdViewEmptyRunID(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdView(f, nil)
	cmd.SetArgs([]string{""})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error for empty run id")
	}
}

func TestNewCmdViewJSONFlag(t *testing.T) {
	t.Parallel()
	cmd := NewCmdView(cmdutil.TestFactory(), func(opts *ViewOptions) error {
		return nil
	})
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("json flag missing")
	}
}

func TestViewRunBuildsV8Path(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	var gotPath string
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					if req.URL.RawQuery != "" {
						gotPath += "?" + req.URL.RawQuery
					}
					return viewTestResponse(http.StatusOK, detailRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	want := "/api/v8/repos/owner/repo/actions/runs/run-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
	if strings.Contains(gotPath, "access_token=") {
		t.Fatalf("request path unexpectedly contains access_token: %q", gotPath)
	}
}

func TestViewRunJSONIsFaithful(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	body := detailRunJSON()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, body), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
		JSON:       true,
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	if out.String() != body+"\n" {
		t.Fatalf("JSON output not faithful verbatim: got len %d, want len %d", len(out.String()), len(body)+1)
	}
}

func TestViewRunHumanRendering(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, detailRunJSON()), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		RunID:      "run-1",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"CI", "#7", "run id", "build", "compile", "Stages"} {
		want := want
		if !strings.Contains(got, want) {
			t.Errorf("human output missing %q; output=%s", want, got)
		}
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
		Repository: "owner/repo",
		RunID:      "missing",
	}

	err := viewRun(opts)
	if err == nil {
		t.Fatal("viewRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to get pipeline run") {
		t.Fatalf("error = %q, want to wrap pipeline run failure", err.Error())
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (404 preserved through %%w wrap)", got, cmdutil.ExitNotFound)
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

func detailRunJSON() string {
	return `{
		"workflow_run_id":"run-1","workflow_id":"wf-1","workflow_name":"CI",
		"file_path":".gitcode/workflows/ci.yml","title":"run CI","status":"COMPLETED",
		"event":"Push","run_number":7,"head_branch":"main","head_sha":"abc123",
		"actor":{"id":"1","object_id":"u1","login":"dev","name":"Dev"},
		"start_time":1700000000,"end_time":1700000100,"pause_time":0,
		"exist_in_default_branch":true,
		"stages":[{"id":"stg-1","category":"ci","name":"build","identifier":"build",
			"status":"COMPLETED","jobs":[{"id":"job-1","name":"compile","identifier":"compile",
			"status":"COMPLETED","steps":[{"id":"step-1","name":"checkout","status":"COMPLETED"}]}]}]
	}`
}

func TestFormatTime(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		tt := tt
		name string
		in   int64
		want string
	}{
		{name: "zero", in: 0, want: "-"},
		{name: "negative", in: -1, want: "-"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatTime(tt.in); got != tt.want {
				t.Errorf("formatTime(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	// The Actions v8 API returns millisecond timestamps (13 digits). A
	// millisecond value and its second equivalent must render identically,
	// and must not explode into year 58486 (the pre-fix bug).
	const sec int64 = 1783500745
	ms := sec * 1000
	gotSec := formatTime(sec)
	gotMs := formatTime(ms)
	if gotSec != gotMs {
		t.Fatalf("formatTime(sec)=%q != formatTime(ms)=%q", gotSec, gotMs)
	}
	if !strings.HasPrefix(gotSec, "2026") {
		t.Fatalf("formatTime(%d) = %q, want a 2026 date (ms/s mixup renders 58486)", sec, gotSec)
	}
}
