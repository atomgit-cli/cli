// Package reviews implements the pr reviews command.
package reviews

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

const reviewStateFixture = `{
  "number": 123,
  "approval_reviewers": [
    {"id":"1","login":"alice","name":"Alice","avatar_url":"a.png","html_url":"u/alice",
     "assignee":true,"code_owner":true,"accept":true,"accept_status":"true"},
    {"id":"2","login":"bob","name":"Bob","html_url":"u/bob",
     "assignee":false,"code_owner":false,"accept":false,"accept_status":"reject"},
    {"id":"3","login":"dave","name":"Dave","html_url":"u/dave",
     "assignee":false,"code_owner":false,"accept":false,"accept_status":"optional"}
  ],
  "testers": [
    {"id":"4","login":"carol","name":"Carol","html_url":"u/carol",
     "assignee":false,"code_owner":false,"accept":true},
    {"id":"6","login":"frank","name":"Frank","html_url":"u/frank",
     "assignee":false,"code_owner":false,"accept":false}
  ],
  "assignees": [
    {"id":"5","login":"erin","name":"Erin","html_url":"u/erin","assignee":true,"code_owner":false,"accept":false}
  ]
}`

func TestNewCmdReviews(t *testing.T) {
	f := cmdutil.TestFactory()
	cmd := NewCmdReviews(f, nil)
	if cmd == nil {
		t.Fatal("NewCmdReviews returned nil")
	}
	if cmd.Use != "reviews <number>" {
		t.Errorf("Use = %q, want reviews <number>", cmd.Use)
	}
	if cmd.Flags().Lookup("repo") == nil {
		t.Error("--repo flag not registered")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("--json flag not registered")
	}
	if cmd.Long == "" {
		t.Error("Long description is empty")
	}
}

func TestReviewsRunJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/repos/owner/repo/pulls/123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reviewStateFixture))
	}))

	err := reviewsRun(&ReviewsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("reviewsRun() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output=%q", err, out.String())
	}
	reviewers, ok := result["approval_reviewers"].([]interface{})
	if !ok || len(reviewers) != 3 {
		t.Fatalf("approval_reviewers = %#v, want three entries", result["approval_reviewers"])
	}
	if reviewers[2].(map[string]interface{})["accept_status"] != "optional" {
		t.Fatalf("dave = %#v, want optional", reviewers[2])
	}
	alice := reviewers[0].(map[string]interface{})
	if alice["login"] != "alice" || alice["accept_status"] != "true" || alice["code_owner"] != true {
		t.Fatalf("alice = %#v", alice)
	}
	bob := reviewers[1].(map[string]interface{})
	if bob["accept_status"] != "reject" {
		t.Fatalf("bob = %#v", bob)
	}
	testers, ok := result["testers"].([]interface{})
	if !ok || len(testers) != 2 || testers[0].(map[string]interface{})["login"] != "carol" {
		t.Fatalf("testers = %#v", result["testers"])
	}
	if testers[1].(map[string]interface{})["accept"] != false {
		t.Fatalf("frank = %#v, want accept=false", testers[1])
	}
	assignees, ok := result["assignees"].([]interface{})
	if !ok || len(assignees) != 1 || assignees[0].(map[string]interface{})["login"] != "erin" {
		t.Fatalf("assignees = %#v", result["assignees"])
	}
}

func TestReviewsRunJSONEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":123,"approval_reviewers":[],"testers":[],"assignees":[]}`))
	}))

	err := reviewsRun(&ReviewsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
		JSON:       true,
	})
	if err != nil {
		t.Fatalf("reviewsRun() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, key := range []string{"approval_reviewers", "testers", "assignees"} {
		arr, ok := result[key].([]interface{})
		if !ok || len(arr) != 0 {
			t.Fatalf("%s = %#v, want empty array (not null)", key, result[key])
		}
	}
}

func TestReviewsRunText(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reviewStateFixture))
	}))

	err := reviewsRun(&ReviewsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
	})
	if err != nil {
		t.Fatalf("reviewsRun() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"alice", "approved", "code owner",
		"bob", "rejected",
		"dave", "pending",
		"carol", "Testers", "accepted", "frank (pending)",
		"erin", "Assignees",
		"Approval reviewers",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("text output missing %q: %q", want, got)
		}
	}
}

func TestReviewsRunTextEmpty(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	io, _, out, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":123,"approval_reviewers":[],"testers":[],"assignees":[]}`))
	}))

	err := reviewsRun(&ReviewsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     123,
	})
	if err != nil {
		t.Fatalf("reviewsRun() error = %v", err)
	}
	if !strings.Contains(out.String(), "No review") {
		t.Fatalf("output = %q, want no-review message", out.String())
	}
}

func TestReviewsRunInvalidNumber(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	// The guard must fire before any request: the handler fails the test
	// if it is ever reached, so no real network call can happen.
	var requests int
	io, _, _, _ := testutil.NewTestIOStreams()
	client := testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{}`))
	}))

	err := reviewsRun(&ReviewsOptions{
		IO:         io,
		HttpClient: func() (*http.Client, error) { return client, nil },
		Repository: "owner/repo",
		Number:     -1,
	})
	if err == nil {
		t.Fatal("reviewsRun() error = nil, want invalid PR number error")
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0 (validation must precede any request)", requests)
	}
}
