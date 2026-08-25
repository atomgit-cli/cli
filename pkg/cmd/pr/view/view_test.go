package view

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdViewTimeFormatEnumAnnotation(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdView(f, nil)
	flag := cmd.Flags().Lookup("time-format")
	if flag == nil {
		t.Fatal("time-format flag not found")
	}
	got := strings.Join(flag.Annotations[cmdutil.FlagEnumAnnotation], ",")
	if got != "absolute,relative" {
		t.Fatalf("time-format enum = %q", got)
	}
}

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view PR",
			args:    []string{"123"},
			wantErr: false,
		},
		{
			name:    "view with web flag",
			args:    []string{"123", "--web"},
			wantErr: false,
		},
		{
			name:    "view with relative time",
			args:    []string{"123", "--time-format", "relative"},
			wantErr: false,
		},
		{
			name:    "no PR number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "invalid time format",
			args:    []string{"123", "--time-format", "yaml"},
			wantErr: true,
		},
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

func TestParseTimeFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    output.TimeFormat
		wantErr bool
	}{
		{name: "default", input: "", want: output.TimeFormatAbsolute},
		{name: "absolute", input: "absolute", want: output.TimeFormatAbsolute},
		{name: "relative", input: "relative", want: output.TimeFormatRelative},
		{name: "invalid", input: "yaml", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseTimeFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseTimeFormat() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTimeFormat() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseTimeFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatViewTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	ts := api.FlexibleTime{Time: now.Add(-90 * time.Minute)}

	if got := formatViewTime(ts.Time, output.TimeFormatAbsolute, now); got != "2026-03-26 10:30" {
		t.Fatalf("absolute format = %q", got)
	}
	if got := formatViewTime(ts.Time, output.TimeFormatRelative, now); got != "1 hour ago" {
		t.Fatalf("relative format = %q", got)
	}
	if got := formatViewTime(time.Time{}, output.TimeFormatAbsolute, now); got != "unknown" {
		t.Fatalf("zero time format = %q", got)
	}
}

func TestRenderPRView(t *testing.T) {
	t.Parallel()
	io, _, _, _ := iostreams.Test()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	pr := &api.PullRequest{
		Title:        "Improve PR details",
		Number:       7,
		State:        "open",
		HTMLURL:      "https://gitcode.com/owner/repo/pulls/7",
		Body:         "PR body",
		Additions:    10,
		Deletions:    2,
		ChangedFiles: 3,
		Commits:      4,
		Comments:     1,
		CreatedAt:    api.FlexibleTime{Time: now.Add(-4 * time.Hour)},
		UpdatedAt:    api.FlexibleTime{Time: now.Add(-45 * time.Minute)},
		User:         &api.User{Login: "alice"},
		Head:         &api.PRBranch{Ref: "feature"},
		Base:         &api.PRBranch{Ref: "main"},
		Assignees:    []*api.User{&api.User{Login: "bob"}},
		Reviewers:    []*api.User{&api.User{Login: "carol"}},
		Labels:       []*api.Label{&api.Label{Name: "bug"}},
	}
	comments := []api.PRComment{
		{
			Body:      "Looks solid",
			User:      &api.User{Login: "dave"},
			CreatedAt: api.FlexibleTime{Time: now.Add(-10 * time.Minute)},
		},
	}

	var buf bytes.Buffer
	if err := renderPRView(&buf, io.ColorScheme(), pr, comments, output.TimeFormatRelative, now); err != nil {
		t.Fatalf("renderPRView() error = %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"Improve PR details #7",
		"State: open",
		"Author: alice",
		"Branch: feature -> main",
		"Created: 4 hours ago",
		"Updated: 45 minutes ago",
		"Additions: +10  Deletions: -2  Files: 3",
		"Commits: 4  Comments: 1",
		"Assignees: bob",
		"Reviewers: carol",
		"Labels: bug",
		"PR body",
		"--- Comments (1) ---",
		"dave at 10 minutes ago",
		"Looks solid",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("renderPRView() output missing %q: %s", want, output)
		}
	}
}

func TestRenderPRCommentsEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	if err := renderPRComments(&buf, nil, output.TimeFormatAbsolute, now); err != nil {
		t.Fatalf("renderPRComments() error = %v", err)
	}

	if got := buf.String(); got != "\n--- No comments ---\n\n" {
		t.Fatalf("renderPRComments() = %q", got)
	}
}

func TestViewRunRejectsJSONWithWebBeforeAuth(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	err := viewRun(&ViewOptions{
		IO:     io,
		Web:    true,
		JSON:   true,
		Number: 1,
		HttpClient: func() (*http.Client, error) {
			t.Fatal("HttpClient should not be called when --json and --web conflict")
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot use --json with --web") {
		t.Fatalf("viewRun() error = %v, want json/web usage error", err)
	}
}

func TestViewRunEnrichesZeroStatsAndDescription(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	var gotPaths []string
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","description":"remote description","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			_, _ = w.Write([]byte(`{"added_lines":7,"remove_lines":2,"count":3,"diffs":[{"new_path":"a.go"}]}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[{"sha":"abc","message":"fix"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		TimeFormat: "absolute",
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	var pr map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if pr["body"] != "remote description" || pr["description"] != "remote description" {
		t.Fatalf("body/description not normalized: %#v", pr)
	}
	if pr["additions"] != float64(7) || pr["deletions"] != float64(2) || pr["changed_files"] != float64(3) || pr["commits"] != float64(1) {
		t.Fatalf("stats not enriched: %#v", pr)
	}
	if strings.Join(gotPaths, ",") != "/api/v5/repos/owner/repo/pulls/123,/api/v5/repos/owner/repo/pulls/123/files.json,/api/v5/repos/owner/repo/pulls/123/commits" {
		t.Fatalf("paths = %#v", gotPaths)
	}
}

func TestViewRunJSONNormalizesNullMergedField(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{
				"number":123,
				"state":"merged",
				"merged":null,
				"merged_at":"2026-07-27T17:31:13+08:00",
				"additions":1,
				"commits":1
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		TimeFormat: "absolute",
	})
	if err != nil {
		t.Fatalf("viewRun() error = %v", err)
	}

	var pr map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		t.Fatalf("output is not JSON: %v; output=%q", err, out.String())
	}
	if pr["state"] != "merged" || pr["merged"] != true {
		t.Fatalf("merged state is contradictory: %#v", pr)
	}
}

func TestViewRunEnrichFailureReturnsErrorButOutputsPR(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, errOut := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`server boom`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[{"sha":"abc","message":"fix"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("error = %q, want 'incomplete'", err.Error())
	}
	if !strings.Contains(errOut.String(), "Failed to enrich") {
		t.Fatalf("stderr = %q, want 'Failed to enrich'", errOut.String())
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d (ExitError) for 500", code, cmdutil.ExitError)
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		t.Fatalf("stdout not JSON despite enrichment failure: %v; out=%q", err, out.String())
	}
	if pr["number"] != float64(123) {
		t.Fatalf("PR number = %v, want 123", pr["number"])
	}
}

func TestViewRunCommentsFailureReturnsErrorInTextMode(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, errOut := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			_, _ = w.Write([]byte(`{"added_lines":0,"remove_lines":0,"count":0,"diffs":[]}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v5/repos/owner/repo/pulls/123/comments":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`bad gateway`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		Comments:   true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("error = %q, want 'incomplete'", err.Error())
	}
	if !strings.Contains(errOut.String(), "Failed to get comments") {
		t.Fatalf("stderr = %q, want 'Failed to get comments'", errOut.String())
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d (ExitError) for 502 comments failure", code, cmdutil.ExitError)
	}
	if !strings.Contains(out.String(), "PR") {
		t.Fatalf("stdout = %q, want PR details rendered", out.String())
	}
}

func TestViewRunEnrich401ReturnsExitAuth(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, errOut := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[{"sha":"abc","message":"fix"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitAuth {
		t.Fatalf("ExitCode = %d, want %d (ExitAuth) for 401 enrichment failure", code, cmdutil.ExitAuth)
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("error = %q, want 'incomplete'", err.Error())
	}
	if !strings.Contains(errOut.String(), "Failed to enrich") {
		t.Fatalf("stderr = %q, want 'Failed to enrich'", errOut.String())
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		t.Fatalf("stdout not JSON despite enrichment failure: %v; out=%q", err, out.String())
	}
	if pr["number"] != float64(123) {
		t.Fatalf("PR number = %v, want 123", pr["number"])
	}
}

func TestViewRunEnrich404ReturnsExitNotFound(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, errOut := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[{"sha":"abc","message":"fix"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (ExitNotFound) for 404 enrichment failure", code, cmdutil.ExitNotFound)
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("error = %q, want 'incomplete'", err.Error())
	}
	if !strings.Contains(errOut.String(), "Failed to enrich") {
		t.Fatalf("stderr = %q, want 'Failed to enrich'", errOut.String())
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		t.Fatalf("stdout not JSON despite enrichment failure: %v; out=%q", err, out.String())
	}
	if pr["number"] != float64(123) {
		t.Fatalf("PR number = %v, want 123", pr["number"])
	}
}

func TestViewRunJSONCommentsFailureWritesJSONAndReturnsError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, errOut := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			_, _ = w.Write([]byte(`{"added_lines":0,"remove_lines":0,"count":0,"diffs":[]}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[]`))
		case "/api/v5/repos/owner/repo/pulls/123/comments":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`bad gateway`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		Comments:   true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("error = %q, want 'incomplete'", err.Error())
	}
	if !strings.Contains(errOut.String(), "Failed to get comments") {
		t.Fatalf("stderr = %q, want 'Failed to get comments'", errOut.String())
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitError {
		t.Fatalf("ExitCode = %d, want %d (ExitError) for 502 comments failure", code, cmdutil.ExitError)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("stdout not JSON despite comments failure: %v; out=%q", err, out.String())
	}
	pr, ok := result["pull_request"].(map[string]interface{})
	if !ok {
		t.Fatalf("stdout JSON missing pull_request: %#v", result)
	}
	if pr["number"] != float64(123) {
		t.Fatalf("PR number = %v, want 123", pr["number"])
	}
	if _, ok := result["comments"]; !ok {
		t.Fatalf("stdout JSON missing comments key (should be null): %#v", result)
	}
}

func TestViewRunEnrichAndCommentsBothFail(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, errOut := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			_, _ = w.Write([]byte(`[{"sha":"abc","message":"fix"}]`))
		case "/api/v5/repos/owner/repo/pulls/123/comments":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`bad gateway`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		Comments:   true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitAuth {
		t.Fatalf("ExitCode = %d, want %d (ExitAuth) when enrichment 401 + comments 502", code, cmdutil.ExitAuth)
	}
	if !strings.Contains(errOut.String(), "Failed to enrich") {
		t.Fatalf("stderr = %q, want 'Failed to enrich'", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Failed to get comments") {
		t.Fatalf("stderr = %q, want 'Failed to get comments'", errOut.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("stdout not JSON despite failures: %v; out=%q", err, out.String())
	}
	pr, ok := result["pull_request"].(map[string]interface{})
	if !ok {
		t.Fatalf("stdout JSON missing pull_request: %#v", result)
	}
	if pr["number"] != float64(123) {
		t.Fatalf("PR number = %v, want 123", pr["number"])
	}
}

func TestViewRunEnrichFilesAndCommitsBothFail(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v5/repos/owner/repo/pulls/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"PR","head":{"ref":"feature"},"base":{"ref":"main"}}`))
		case "/api/v5/repos/owner/repo/pulls/123/files.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		case "/api/v5/repos/owner/repo/pulls/123/commits":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`server boom`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	err := viewRun(&ViewOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		BaseRepo:   func() (string, error) { return "owner/repo", nil },
		Number:     123,
		JSON:       true,
		TimeFormat: "absolute",
	})
	if err == nil {
		t.Fatal("viewRun() error = nil, want incomplete-data error")
	}
	if code := cmdutil.ExitCode(err); code != cmdutil.ExitNotFound {
		t.Fatalf("ExitCode = %d, want %d (ExitNotFound) when files 404 + commits 500 (first error wins)", code, cmdutil.ExitNotFound)
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil {
		t.Fatalf("stdout not JSON despite enrichment failure: %v; out=%q", err, out.String())
	}
	if pr["number"] != float64(123) {
		t.Fatalf("PR number = %v, want 123", pr["number"])
	}
}
