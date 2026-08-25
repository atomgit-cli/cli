package edit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdEdit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "edit title",
			args:    []string{"123", "--title", "New title", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit body",
			args:    []string{"123", "--body", "New body", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit state close",
			args:    []string{"123", "--state", "close", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit state reopen",
			args:    []string{"123", "--state", "reopen", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit with labels",
			args:    []string{"123", "--label", "bug,enhancement", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit with assignees",
			args:    []string{"123", "--assignee", "user1", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit with milestone",
			args:    []string{"123", "--milestone", "5", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit with security-hole",
			args:    []string{"123", "--security-hole", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit multiple fields",
			args:    []string{"123", "--title", "Title", "--body", "Body", "--label", "bug", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "edit with json output",
			args:    []string{"123", "--title", "Title", "--json", "-R", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "missing issue number",
			args:    []string{"-R", "owner/repo"},
			wantErr: true,
		},
		{
			name:    "invalid issue number",
			args:    []string{"abc", "-R", "owner/repo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdEdit(f, func(opts *EditOptions) error {
				// Mock run function - just validate
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

func TestEditRunJSONWritesUpdatedIssue(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/api/v5/repos/owner/issues/12" {
						t.Fatalf("unexpected request: %s", req.URL.Path)
					}
					return issueResponse(http.StatusOK, `{"number":"12","title":"updated","html_url":"https://gitcode.com/owner/repo/issues/12"}`), nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Number:     12,
		Title:      "updated",
		JSON:       true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, string(out))
	}
	if got["number"] != "12" || got["html_url"] != "https://gitcode.com/owner/repo/issues/12" {
		t.Fatalf("JSON output = %#v", got)
	}
	if strings.Contains(string(out), "Updated issue") {
		t.Fatalf("JSON output contains text banner: %q", string(out))
	}
}

func TestEditRun_NoEditOptions(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Number:     123,
		Repository: "owner/repo",
	}

	err := editRun(opts)
	if err == nil {
		t.Error("Expected error when no edit options provided")
	}
	if err.Error() != "at least one edit option is required (e.g., --title, --body, --body-file, --state, --assignee, --label, --milestone, --security-hole)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestParseRepo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		repo      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"gitcode-cli/cli", "gitcode-cli", "cli", false},
		{"", "", "", true},
		{"invalid", "", "", true},
		{"too/many/parts", "", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.repo, func(t *testing.T) {
			t.Parallel()
			owner, repo, err := parseRepo(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if owner != tt.wantOwner {
				t.Errorf("parseRepo() owner = %v, want %v", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseRepo() repo = %v, want %v", repo, tt.wantRepo)
			}
		})
	}
}

func TestEditRunFailsWhenAssigneesAreNotApplied(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/api/v5/repos/owner/issues/12":
						return issueResponse(http.StatusOK, `{"number":"12","html_url":"https://gitcode.com/owner/repo/issues/12"}`), nil
					case "/api/v5/repos/owner/repo/issues/12":
						return issueResponse(http.StatusOK, `{"number":"12","assignees":[]}`), nil
					default:
						t.Fatalf("unexpected request: %s", req.URL.Path)
						return nil, nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		Number:     12,
		Title:      "same title",
		Assignees:  []string{"alice"},
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want assignee verification error")
	}
	if !strings.Contains(f.IOStreams.Out.(*bytes.Buffer).String(), "Updated issue #12") {
		t.Fatalf("stdout = %q, want updated issue output", f.IOStreams.Out.(*bytes.Buffer).String())
	}
	if !strings.Contains(err.Error(), "did not apply the requested assignees") {
		t.Fatalf("editRun() error = %v", err)
	}
}

func TestEditRunJSONSuppressesOutputWhenAssigneesAreNotApplied(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/api/v5/repos/owner/issues/12":
						return issueResponse(http.StatusOK, `{"number":"12","html_url":"https://gitcode.com/owner/repo/issues/12"}`), nil
					case "/api/v5/repos/owner/repo/issues/12":
						return issueResponse(http.StatusOK, `{"number":"12","assignees":[]}`), nil
					default:
						t.Fatalf("unexpected request: %s", req.URL.Path)
						return nil, nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		Number:     12,
		Title:      "same title",
		Assignees:  []string{"alice"},
		JSON:       true,
	}

	err := editRun(opts)
	if err == nil {
		t.Fatal("editRun() error = nil, want assignee verification error")
	}
	if out := f.IOStreams.Out.(*bytes.Buffer).String(); out != "" {
		t.Fatalf("stdout = %q, want empty JSON output on failed verification", out)
	}
}

func TestEditRunUsesAssigneeUsernameWithoutResolution(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/api/v5/repos/owner/issues/12":
						body, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("ReadAll() error = %v", err)
						}
						values, err := url.ParseQuery(string(body))
						if err != nil {
							t.Fatalf("ParseQuery() error = %v", err)
						}
						if got := values.Get("assignee"); got != "alice" {
							t.Fatalf("assignee = %q, want alice", got)
						}
						return issueResponse(
							http.StatusOK,
							`{"number":"12","html_url":"https://gitcode.com/owner/repo/issues/12"}`,
						), nil
					case "/api/v5/repos/owner/repo/issues/12":
						return issueResponse(
							http.StatusOK,
							`{"number":"12","assignees":[{"login":"alice"}]}`,
						), nil
					default:
						t.Fatalf("unexpected request: %s", req.URL.Path)
						return nil, nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		Number:     12,
		Assignees:  []string{"alice"},
		JSON:       true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(f.IOStreams.Out.(*bytes.Buffer).Bytes(), &got); err != nil {
		t.Fatalf("JSON output did not parse: %v", err)
	}
	assignees, ok := got["assignees"].([]interface{})
	if !ok || len(assignees) != 1 || assignees[0].(map[string]interface{})["login"] != "alice" {
		t.Fatalf("JSON assignees = %#v, want verified alice", got["assignees"])
	}
}

func TestEditRunFailsWhenAssigneeVerificationReadFails(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/api/v5/repos/owner/issues/12":
						return issueResponse(
							http.StatusOK,
							`{"number":"12","html_url":"https://gitcode.com/owner/repo/issues/12"}`,
						), nil
					case "/api/v5/repos/owner/repo/issues/12":
						return issueResponse(http.StatusInternalServerError, `{"message":"temporary failure"}`), nil
					default:
						t.Fatalf("unexpected request: %s", req.URL.Path)
						return nil, nil
					}
				}),
			}, nil
		},
		Repository: "owner/repo",
		Number:     12,
		Assignees:  []string{"alice"},
		JSON:       true,
	}

	err := editRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to verify requested assignees") {
		t.Fatalf("editRun() error = %v, want verification failure", err)
	}
	if !strings.Contains(err.Error(), "https://gitcode.com/owner/repo/issues/12") {
		t.Fatalf("editRun() error = %v, want updated issue URL", err)
	}
	if out := f.IOStreams.Out.(*bytes.Buffer).String(); out != "" {
		t.Fatalf("stdout = %q, want empty JSON output on failed verification", out)
	}
}

func issueResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetBodyScansInlineBodyForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	f := cmdutil.TestFactory()
	opts := &EditOptions{IO: f.IOStreams, Body: "leaked: secret-token-abc123"}
	_, err := getBody(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("getBody() error = %v, want secret detection error", err)
	}
}
