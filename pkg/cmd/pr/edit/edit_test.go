package edit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdEdit(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "valid number with title",
			args:    []string{"123", "--repo", "owner/repo", "--title", "New title"},
			wantErr: false,
		},
		{
			name:    "invalid number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "no flags specified",
			args:    []string{"123", "--repo", "owner/repo"},
			wantErr: true, // No changes specified
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: io,
				HttpClient: func() (*http.Client, error) {
					return &http.Client{}, nil
				},
			}

			var runCalled bool
			cmd := NewCmdEdit(f, func(opts *EditOptions) error {
				runCalled = true
				// Check if no changes specified
				if opts.Title == "" && opts.Body == "" && opts.BodyFile == "" &&
					opts.Base == "" && opts.Draft == "" &&
					len(opts.Labels) == 0 && opts.Milestone == 0 &&
					opts.CloseRelatedIssue == "" {
					return fmt.Errorf("no changes specified. Use flags to specify what to edit")
				}
				return nil
			})
			cmd.SetArgs(tt.args)

			_, err := cmd.ExecuteC()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteC() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !runCalled {
				t.Error("run function was not called")
			}
		})
	}
}

func TestEditRun(t *testing.T) {
	tests := []struct {
		name    string
		opts    *EditOptions
		wantErr bool
	}{
		{
			name: "no repository",
			opts: &EditOptions{
				Repository: "",
				Number:     123,
				Title:      "New title",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, _ := iostreams.Test()
			tt.opts.IO = io
			tt.opts.HttpClient = func() (*http.Client, error) {
				return &http.Client{}, nil
			}

			err := editRun(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("editRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCmdEditTracksLabelFlags(t *testing.T) {
	ioStreams, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ioStreams}

	var got *EditOptions
	cmd := NewCmdEdit(f, func(opts *EditOptions) error {
		got = opts
		return nil
	})
	cmd.SetArgs([]string{
		"123", "-R", "owner/repo",
		"--labels", "legacy",
		"--add-label", "added",
		"--remove-label", "removed",
	})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("ExecuteC() error = %v", err)
	}
	if got == nil || !got.LabelsSet || !got.AddLabelsSet || !got.RemoveLabelsSet {
		t.Fatalf("label flag presence was not recorded: %#v", got)
	}
}

func TestEditRunUpdatesLabels(t *testing.T) {
	tests := []struct {
		name          string
		configure     func(*EditOptions)
		wantLabels    string
		wantRequests  int
		wantFirstVerb string
	}{
		{
			name: "legacy labels add and deduplicate",
			configure: func(opts *EditOptions) {
				opts.Labels = []string{"risk/high", "status/approved"}
			},
			wantLabels:    "type/bug,risk/high,status/approved",
			wantRequests:  3,
			wantFirstVerb: http.MethodGet,
		},
		{
			name: "add and remove preserve other labels",
			configure: func(opts *EditOptions) {
				opts.AddLabels = []string{"status/approved"}
				opts.RemoveLabels = []string{"risk/high"}
			},
			wantLabels:    "type/bug,status/approved",
			wantRequests:  3,
			wantFirstVerb: http.MethodGet,
		},
		{
			name: "replace labels skips current labels lookup",
			configure: func(opts *EditOptions) {
				opts.ReplaceLabels = []string{"status/approved", "status/approved"}
				opts.ReplaceLabelsSet = true
				opts.Yes = true
			},
			wantLabels:    "status/approved",
			wantRequests:  2,
			wantFirstVerb: http.MethodPatch,
		},
		{
			name: "replace with empty value clears labels",
			configure: func(opts *EditOptions) {
				opts.ReplaceLabelsSet = true
				opts.Yes = true
			},
			wantLabels:    ",",
			wantRequests:  2,
			wantFirstVerb: http.MethodPatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GC_TOKEN", "test-token")
			var methods []string
			var gotLabels string

			ioStreams, _, _, _ := iostreams.Test()
			opts := &EditOptions{
				IO:         ioStreams,
				Repository: "owner/repo",
				Number:     123,
			}
			tt.configure(opts)
			opts.HttpClient = func() (*http.Client, error) {
				return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					methods = append(methods, req.Method)
					switch req.Method {
					case http.MethodGet:
						return editTestResponse(`{"number":123,"labels":[{"name":"type/bug"},{"name":"risk/high"}]}`), nil
					case http.MethodPatch:
						body, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("read request body: %v", err)
						}
						values, err := url.ParseQuery(string(body))
						if err != nil {
							t.Fatalf("parse request body: %v", err)
						}
						gotLabels = values.Get("labels")
						if _, ok := values["labels"]; !ok {
							t.Fatal("PATCH request omitted labels field")
						}
						return editTestResponse(`{"number":123,"title":"updated"}`), nil
					default:
						t.Fatalf("unexpected method %s", req.Method)
						return nil, nil
					}
				})}, nil
			}

			if err := editRun(opts); err != nil {
				t.Fatalf("editRun() error = %v", err)
			}
			if len(methods) != tt.wantRequests {
				t.Fatalf("request count = %d, want %d (%v)", len(methods), tt.wantRequests, methods)
			}
			if methods[0] != tt.wantFirstVerb {
				t.Fatalf("first method = %q, want %q", methods[0], tt.wantFirstVerb)
			}
			if gotLabels != tt.wantLabels {
				t.Fatalf("labels = %q, want %q", gotLabels, tt.wantLabels)
			}
		})
	}
}

func TestEditRunRejectsUnsafeLabelChangesBeforeHTTP(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*EditOptions)
		want      string
	}{
		{
			name: "replace requires confirmation",
			configure: func(opts *EditOptions) {
				opts.ReplaceLabels = []string{"status/approved"}
				opts.ReplaceLabelsSet = true
			},
			want: "confirmation required",
		},
		{
			name: "replace conflicts with add",
			configure: func(opts *EditOptions) {
				opts.ReplaceLabelsSet = true
				opts.AddLabels = []string{"status/approved"}
			},
			want: "cannot be combined",
		},
		{
			name: "same label cannot be added and removed",
			configure: func(opts *EditOptions) {
				opts.AddLabels = []string{"status/approved"}
				opts.RemoveLabels = []string{"status/approved"}
			},
			want: "both added and removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ioStreams, _, _, _ := iostreams.Test()
			httpCalled := false
			opts := &EditOptions{
				IO:         ioStreams,
				Repository: "owner/repo",
				Number:     123,
				HttpClient: func() (*http.Client, error) {
					httpCalled = true
					return &http.Client{}, nil
				},
			}
			tt.configure(opts)

			err := editRun(opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("editRun() error = %v, want containing %q", err, tt.want)
			}
			if httpCalled {
				t.Fatal("HTTP client was created before label validation or confirmation")
			}
		})
	}
}

func editTestResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestEditRunJSONFetchesCompleteUpdatedPR(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var requests []string
	var getCount int
	ioStreams, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.Path)

				var body string
				switch req.Method {
				case http.MethodPatch:
					body = `{"title":"updated","labels":null}`
					formBody, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read PATCH body: %v", err)
					}
					values, err := url.ParseQuery(string(formBody))
					if err != nil {
						t.Fatalf("parse PATCH body: %v", err)
					}
					if got := values.Get("labels"); got != "type/bug,type/feature" {
						t.Fatalf("PATCH labels = %q, want preserved and added labels", got)
					}
				case http.MethodGet:
					getCount++
					if getCount == 1 {
						body = `{"number":123,"labels":[{"id":1,"name":"type/bug","color":"#428BCA"}]}`
					} else {
						body = `{
							"id":456,
							"number":123,
							"title":"updated",
							"labels":[
								{"id":1,"name":"type/bug","color":"#428BCA"},
								{"id":2,"name":"type/feature","color":"#428BCA"}
							]
						}`
					}
				default:
					t.Fatalf("unexpected request method: %s", req.Method)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			})}, nil
		},
		Repository: "owner/repo",
		Number:     123,
		Title:      "updated",
		AddLabels:  []string{"type/feature"},
		JSON:       true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}

	wantRequests := []string{
		"GET /api/v5/repos/owner/repo/pulls/123",
		"PATCH /api/v5/repos/owner/repo/pulls/123",
		"GET /api/v5/repos/owner/repo/pulls/123",
	}
	if strings.Join(requests, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}

	var result struct {
		ID     interface{} `json:"id"`
		Number int         `json:"number"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	if result.ID == nil || result.Number != 123 {
		t.Fatalf("incomplete PR identity: %#v", result)
	}
	if len(result.Labels) != 2 ||
		result.Labels[0].Name != "type/bug" ||
		result.Labels[1].Name != "type/feature" {
		t.Fatalf("labels = %#v, want updated labels", result.Labels)
	}
}

func TestEditRunJSONReturnsErrorWhenFinalStateCannotBeFetched(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ioStreams, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				status := http.StatusOK
				body := `{"title":"updated"}`
				if req.Method == http.MethodGet {
					status = http.StatusInternalServerError
					body = `{"message":"temporary failure"}`
				}
				return &http.Response{
					StatusCode: status,
					Status:     http.StatusText(status),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			})}, nil
		},
		Repository: "owner/repo",
		Number:     123,
		Title:      "updated",
		JSON:       true,
	}

	err := editRun(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to fetch updated PR") {
		t.Fatalf("editRun() error = %v, want final state fetch error", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want no partial JSON", out.String())
	}
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid repo",
			repo:      "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:    "empty repo requires explicit repo",
			repo:    "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			repo:    "invalid",
			wantErr: true,
		},
		{
			name:    "too many parts",
			repo:    "owner/repo/extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRepo(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("parseRepo() owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseRepo() repo = %v, want %v", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestEditRunScansInlineBodyForSecrets(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	f := cmdutil.TestFactory()
	opts := &EditOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     1,
		Body:       "leaked: secret-token-abc123",
	}
	err := editRun(opts)
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("editRun() error = %v, want secret detection error", err)
	}
}

func TestEditRunAddAssignee(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	var methods []string
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				return editTestResponse(`{"number":123,"title":"test"}`), nil
			})}, nil
		},
		Repository:   "owner/repo",
		Number:       123,
		AddAssignees: []string{"user1", "user2"},
	}
	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	foundPost := false
	for _, m := range methods {
		if m == "POST" {
			foundPost = true
		}
	}
	if !foundPost {
		t.Fatalf("expected POST request for add-assignee; methods: %v", methods)
	}
}

func TestEditRunAddReviewerAndTester(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	var methods []string
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				return editTestResponse(`{"number":123,"title":"test"}`), nil
			})}, nil
		},
		Repository:   "owner/repo",
		Number:       123,
		AddReviewers: []string{"rev1"},
		AddTesters:   []string{"test1"},
	}
	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	postCount := 0
	for _, m := range methods {
		if m == "POST" {
			postCount++
		}
	}
	if postCount < 2 {
		t.Fatalf("expected at least 2 POST requests (reviewer + tester); methods: %v", methods)
	}
}

func TestEditRunRemoveAssignee(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	io, _, _, _ := iostreams.Test()
	var methods []string
	opts := &EditOptions{
		IO: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				methods = append(methods, req.Method)
				return editTestResponse(`{"number":123,"title":"test"}`), nil
			})}, nil
		},
		Repository:      "owner/repo",
		Number:          123,
		RemoveAssignees: []string{"user1"},
	}
	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}
	foundDelete := false
	for _, m := range methods {
		if m == "DELETE" {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Fatalf("expected DELETE request for remove-assignee; methods: %v", methods)
	}
}

func TestHasAssigneeChanges(t *testing.T) {
	tests := []struct {
		name string
		opts *EditOptions
		want bool
	}{
		{name: "no changes", opts: &EditOptions{}, want: false},
		{name: "add assignee", opts: &EditOptions{AddAssignees: []string{"u1"}}, want: true},
		{name: "remove reviewer", opts: &EditOptions{RemoveReviewers: []string{"u1"}}, want: true},
		{name: "add tester", opts: &EditOptions{AddTesters: []string{"u1"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasAssigneeChanges(tt.opts); got != tt.want {
				t.Errorf("hasAssigneeChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEditRunSetsPRMergers(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	var requests []string
	var mergerBody string
	ioStreams, _, out, _ := iostreams.Test()
	opts := &EditOptions{
		IO: ioStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.Path)

				var body string
				switch req.Method {
				case http.MethodPut:
					if req.URL.Path == "/api/v5/repos/owner/repo/pulls/123/mergers" {
						bodyBytes, err := io.ReadAll(req.Body)
						if err != nil {
							t.Fatalf("read PUT body: %v", err)
						}
						mergerBody = string(bodyBytes)
						body = `[{"id":1,"login":"user1","name":"User One","object_id":"o1"}]`
					} else {
						t.Fatalf("unexpected PUT path: %s", req.URL.Path)
					}
				case http.MethodGet:
					body = `{"number":123,"title":"pr title","html_url":"https://gitcode.com/owner/repo/pulls/123"}`
				default:
					t.Fatalf("unexpected request method: %s", req.Method)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			})}, nil
		},
		Repository: "owner/repo",
		Number:     123,
		Mergers:    []string{"user1"},
		MergersSet: true,
	}

	if err := editRun(opts); err != nil {
		t.Fatalf("editRun() error = %v", err)
	}

	wantRequests := []string{
		"PUT /api/v5/repos/owner/repo/pulls/123/mergers",
		"GET /api/v5/repos/owner/repo/pulls/123",
	}
	if strings.Join(requests, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requests = %q, want %q", requests, wantRequests)
	}
	if !strings.Contains(mergerBody, `"mergers":"user1"`) {
		t.Fatalf("mergers body = %q, want mergers field", mergerBody)
	}
	if !strings.Contains(out.String(), "mergers: user1") {
		t.Fatalf("output = %q, want mergers line in output", out.String())
	}
}

func TestNewCmdEditMergersFlag(t *testing.T) {
	f := cmdutil.TestFactory()
	var gotMergers []string
	var gotSet bool
	cmd := NewCmdEdit(f, func(opts *EditOptions) error {
		gotMergers = opts.Mergers
		gotSet = opts.MergersSet
		return nil
	})
	cmd.SetArgs([]string{"123", "-R", "owner/repo", "--mergers", "user1,user2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(gotMergers) != 2 || gotMergers[0] != "user1" || gotMergers[1] != "user2" {
		t.Fatalf("Mergers = %v, want [user1 user2]", gotMergers)
	}
	if !gotSet {
		t.Fatalf("MergersSet = false, want true")
	}
}
