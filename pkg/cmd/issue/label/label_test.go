package label

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "add labels",
			args:    []string{"123", "--add", "bug,enhancement"},
			wantErr: false,
		},
		{
			name:    "remove label",
			args:    []string{"123", "--remove", "bug"},
			wantErr: false,
		},
		{
			name:    "list labels",
			args:    []string{"123", "--list"},
			wantErr: false,
		},
		{
			name:    "no issue number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid issue number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "no action specified",
			args:    []string{"123"},
			wantErr: false, // Command runs, error in run function
		},
		{
			name:    "add with repo",
			args:    []string{"123", "--add", "bug", "-R", "owner/repo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdLabel(f, func(opts *LabelOptions) error {
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

// issueLabelClient captures request paths/methods so tests can assert the
// command hits the issue update form (PATCH /issues/{n}) and not a stale
// dedicated-labels endpoint. GET returns getStatus+getBody; PATCH/PUT return
// patchStatus+patchBody.
func issueLabelClient(t *testing.T, gotPath, gotMethod *string, getStatus, patchStatus int, getBody, patchBody string) *http.Client {
	t.Helper()
	return &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			*gotPath = req.URL.Path
			*gotMethod = req.Method
			status, body := getStatus, getBody
			if req.Method == http.MethodPatch || req.Method == http.MethodPut {
				status, body = patchStatus, patchBody
			}
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}

func httpFactory(client *http.Client) func() (*http.Client, error) {
	return func() (*http.Client, error) { return client, nil }
}

func TestLabelRunCombinedRemoveAndAddIsAtomic(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath, gotMethod string
	client := issueLabelClient(t, &gotPath, &gotMethod, http.StatusOK, http.StatusOK,
		`{"number":"123","labels":[{"name":"triage"},{"name":"keep"}]}`,
		`{"number":"123","labels":[{"name":"keep"},{"name":"verified"},{"name":"in-progress"}]}`)
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "triage",
		Add:        []string{"verified,in-progress"},
		AddSet:     true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	// Combined path must read the issue then update it once (PATCH /issues/123).
	if !strings.Contains(gotPath, "/issues/123") {
		t.Fatalf("request path = %q, want to contain /issues/123", gotPath)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH (single atomic update)", gotMethod)
	}
	if !strings.Contains(out.String(), "Removed label 'triage'") {
		t.Fatalf("output = %q, want removal confirmation", out.String())
	}
	if !strings.Contains(out.String(), "Added labels to issue #123: verified, in-progress") {
		t.Fatalf("output = %q, want add confirmation", out.String())
	}
}

func TestLabelRunCombinedJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotPath, gotMethod string
	client := issueLabelClient(t, &gotPath, &gotMethod, http.StatusOK, http.StatusOK,
		`{"number":"123","labels":[{"name":"triage"}]}`,
		`{"number":"123","labels":[{"name":"verified"}]}`)
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "triage",
		Add:        []string{"verified"},
		AddSet:     true,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", gotMethod)
	}
	if !strings.Contains(out.String(), `"action": "update"`) {
		t.Fatalf("output = %q, want action update", out.String())
	}
	if !strings.Contains(out.String(), `"label": "triage"`) {
		t.Fatalf("output = %q, want removed label", out.String())
	}
}

// P1: an invalid --add value combined with --remove must NOT trigger any
// remote write (no partial-success: label removed but nothing added).
func TestLabelRunCombinedInvalidAddDoesNotMutate(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	for _, add := range []string{"", ",", " , , "} {
		add := add
		client := &http.Client{
			Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("no remote request expected for invalid add %q; got %s %s", add, req.Method, req.URL.Path)
				return nil, nil
			}),
		}
		ios, _, _, _ := iostreams.Test()
		err := labelRun(&LabelOptions{
			IO:         ios,
			HttpClient: httpFactory(client),
			Repository: "owner/repo",
			Number:     123,
			Remove:     "triage",
			Add:        []string{add},
			AddSet:     true,
		})
		if err == nil {
			t.Fatalf("add=%q: expected usage error, got nil", add)
		}
		if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
			t.Fatalf("add=%q: ExitCode = %d, want ExitUsage(%d)", add, got, cmdutil.ExitUsage)
		}
	}
}

// P1 (cobra parity): `--add ""` is parsed by cobra's StringSlice to an empty
// slice, so opts.Add has length 0 even though the flag was passed. AddSet must
// still trigger validation and reject without any remote mutation.
func TestLabelRunCombinedEmptyAddSliceDoesNotMutate(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	client := &http.Client{
		Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("no remote request expected; got %s %s", req.Method, req.URL.Path)
			return nil, nil
		}),
	}
	ios, _, _, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "triage",
		Add:        []string{}, // mimics cobra parsing `--add ""`
		AddSet:     true,
	})
	if err == nil {
		t.Fatal("expected usage error, got nil")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode = %d, want ExitUsage(%d)", got, cmdutil.ExitUsage)
	}
}

// Edge: when the removed label is absent from the current set, the combined
// path still adds the new labels (delete on an absent map key is a no-op).
func TestLabelRunCombinedRemoveAbsentLabel(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotMethod string
	var gotPath string
	// GET returns [keep] (no "triage"); PATCH returns the final set.
	client := issueLabelClient(t, &gotPath, &gotMethod, http.StatusOK, http.StatusOK,
		`{"number":"123","labels":[{"name":"keep"}]}`,
		`{"number":"123","labels":[{"name":"keep"},{"name":"verified"}]}`)
	ios, _, out, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "triage",
		Add:        []string{"verified"},
		AddSet:     true,
	})
	if err != nil {
		t.Fatalf("labelRun() error = %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", gotMethod)
	}
	if !strings.Contains(out.String(), "Added labels to issue #123: verified") {
		t.Fatalf("output = %q, want add confirmation", out.String())
	}
}

func TestLabelRunCombinedGetIssueFails(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotMethod string
	var gotPath string
	client := issueLabelClient(t, &gotPath, &gotMethod, http.StatusNotFound, http.StatusOK, `{}`, `{}`)
	ios, _, _, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "triage",
		Add:        []string{"verified"},
		AddSet:     true,
	})
	if err == nil {
		t.Fatal("labelRun() error = nil, want not-found")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want ExitNotFound(%d)", got, cmdutil.ExitNotFound)
	}
}

func TestLabelRunCombinedUpdateFails(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var gotMethod string
	var gotPath string
	// GET succeeds with current labels; PATCH fails with 500.
	client := issueLabelClient(t, &gotPath, &gotMethod, http.StatusOK, http.StatusInternalServerError,
		`{"number":"123","labels":[{"name":"triage"},{"name":"keep"}]}`, `{}`)
	ios, _, _, _ := iostreams.Test()
	err := labelRun(&LabelOptions{
		IO:         ios,
		HttpClient: httpFactory(client),
		Repository: "owner/repo",
		Number:     123,
		Remove:     "triage",
		Add:        []string{"verified"},
		AddSet:     true,
	})
	if err == nil {
		t.Fatal("labelRun() error = nil, want update failure")
	}
	if gotMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH (update attempted)", gotMethod)
	}
	if !strings.Contains(err.Error(), "failed to update issue labels") {
		t.Fatalf("error = %q, want 'failed to update issue labels'", err.Error())
	}
}
