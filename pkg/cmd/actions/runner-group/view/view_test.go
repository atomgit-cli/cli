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
		{name: "view with org and id", args: []string{"rg-1", "--org", "my-org"}, wantErr: false},
		{name: "view with json", args: []string{"rg-1", "--org", "my-org", "--json"}, wantErr: false},
		{name: "missing org", args: []string{"rg-1"}, wantErr: true},
		{name: "missing id", args: []string{}, wantErr: true},
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

func TestViewRunJSONOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, `{"runner_group_id":"1","runner_group_name":"prod","share_all":true,"share_all_public_repos":false,"explicit_shared_repo_count":3,"created_at":1700000000000,"updated_at":1700000100000}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "1",
		JSON:          true,
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"runner_group_id":"1"`) {
		t.Fatalf("JSON output = %q, missing runner_group_id", got)
	}
	if !strings.Contains(got, `"runner_group_name":"prod"`) {
		t.Fatalf("JSON output = %q, missing runner_group_name", got)
	}
	if !strings.Contains(got, `"share_all":true`) {
		t.Fatalf("JSON output = %q, missing share_all", got)
	}
}

func TestViewRunTextOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, `{"runner_group_id":"1","runner_group_name":"prod","share_all":true,"share_all_public_repos":false,"explicit_shared_repo_count":3,"created_at":1700000000000,"updated_at":1700000100000}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "1",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "prod") {
		t.Fatalf("text output = %q, missing group name", got)
	}
	if !strings.Contains(got, "Share All:") && !strings.Contains(got, "yes") {
		t.Fatalf("text output = %q, missing share status", got)
	}
	if !strings.Contains(got, "Shared Repos:") {
		t.Fatalf("text output = %q, missing shared repo count", got)
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
					return viewTestResponse(http.StatusOK, `{"runner_group_id":"1","runner_group_name":"prod"}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "rg-1",
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	want := "/api/v8/orgs/my-org/actions/runner-groups/rg-1"
	if gotPath != want {
		t.Fatalf("request path = %q, want %q", gotPath, want)
	}
}

func TestViewRunMissingOrg(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ViewOptions{
		IO:            io,
		HttpClient:    func() (*http.Client, error) { return &http.Client{}, nil },
		Org:           "",
		RunnerGroupID: "1",
	}

	err := viewRun(opts)
	if err == nil {
		t.Fatal("viewRun() error = nil, want error for missing --org")
	}
}

func TestViewRunMissingID(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, _, _ := iostreams.Test()
	opts := &ViewOptions{
		IO:            io,
		HttpClient:    func() (*http.Client, error) { return &http.Client{}, nil },
		Org:           "my-org",
		RunnerGroupID: "",
	}

	err := viewRun(opts)
	if err == nil {
		t.Fatal("viewRun() error = nil, want error for missing runner group id")
	}
}

func TestViewRunAPIError(t *testing.T) {
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
		Org:           "my-org",
		RunnerGroupID: "nonexistent",
	}

	err := viewRun(opts)
	if err == nil {
		t.Fatal("viewRun() error = nil, want error for 404")
	}
}

func TestViewRunJSONFaithfulOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := iostreams.Test()
	opts := &ViewOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return viewTestResponse(http.StatusOK, `{"runner_group_id":"1","runner_group_name":"prod","share_all":true,"extra_field":"preserved"}`), nil
				}),
			}, nil
		},
		Org:           "my-org",
		RunnerGroupID: "1",
		JSON:          true,
	}

	if err := viewRun(opts); err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "extra_field") {
		t.Fatalf("JSON output = %q, missing extra_field (not faithful)", got)
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
