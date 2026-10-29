package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestListPullRequestsBuildsQuery(t *testing.T) {
	var gotPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		return authTestResponse(http.StatusOK, `[]`), nil
	})

	_, err := ListPullRequests(client, "owner", "repo", &PRListOptions{
		State:     "open",
		Head:      "feature/login",
		Base:      "main",
		Sort:      "updated",
		Direction: "asc",
		PerPage:   50,
		Page:      2,
	})
	if err != nil {
		t.Fatalf("ListPullRequests() error = %v", err)
	}

	assertPRListRequest(t, gotPath, "/api/v5/repos/owner/repo/pulls", map[string]string{
		"state":     "open",
		"head":      "feature/login",
		"base":      "main",
		"sort":      "updated",
		"direction": "asc",
		"per_page":  "50",
		"page":      "2",
	})
}

func TestCreatePullRequestUsesFormEncoding(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotContentType string
	var gotBody string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		gotContentType = req.Header.Get("Content-Type")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		return authTestResponse(http.StatusOK, `{"number":123,"title":"created","body":"body text"}`), nil
	})
	client.SetToken("test-token", "test")

	pr, err := CreatePullRequest(client, "owner", "repo", &CreatePROptions{
		Title:  "created",
		Body:   "body text",
		Head:   "myfork:feature",
		Base:   "main",
		Draft:  true,
		Labels: []string{"bug", "enhancement"},
	})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	if gotPath != "/api/v5/repos/owner/repo/pulls" {
		t.Fatalf("request path = %q, want %q", gotPath, "/api/v5/repos/owner/repo/pulls")
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want %q", gotContentType, "application/x-www-form-urlencoded")
	}

	expectedPairs := []string{
		"title=created",
		"body=body+text",
		"head=myfork%3Afeature",
		"base=main",
		"draft=true",
		"labels%5B%5D=bug",
		"labels%5B%5D=enhancement",
	}
	if strings.Contains(gotBody, "fork_path") {
		t.Fatalf("request body %q should not contain fork_path", gotBody)
	}
	for _, pair := range expectedPairs {
		if !strings.Contains(gotBody, pair) {
			t.Fatalf("request body %q does not contain %q", gotBody, pair)
		}
	}
	if pr.Body != "body text" {
		t.Fatalf("response body = %q, want %q", pr.Body, "body text")
	}
}

func TestCreatePullRequestHydratesMissingBodyFromView(t *testing.T) {
	var gotPaths []string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPaths = append(gotPaths, req.URL.Path)
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/repos/owner/repo/pulls":
			return authTestResponse(http.StatusOK, `{"number":123,"title":"created"}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/api/v5/repos/owner/repo/pulls/123":
			return authTestResponse(http.StatusOK, `{"number":123,"title":"created","body":"body text","html_url":"https://gitcode.com/owner/repo/merge_requests/123"}`), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	pr, err := CreatePullRequest(client, "owner", "repo", &CreatePROptions{
		Title: "created",
		Body:  "body text",
		Head:  "feature",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}
	if pr.Body != "body text" {
		t.Fatalf("response body = %q, want %q", pr.Body, "body text")
	}
	if pr.HTMLURL != "https://gitcode.com/owner/repo/merge_requests/123" {
		t.Fatalf("HTMLURL = %q", pr.HTMLURL)
	}
	if strings.Join(gotPaths, ",") != "/api/v5/repos/owner/repo/pulls,/api/v5/repos/owner/repo/pulls/123" {
		t.Fatalf("request paths = %v", gotPaths)
	}
}

func TestCreatePullRequestLeavesMissingBodyWhenViewFails(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/repos/owner/repo/pulls":
			return authTestResponse(http.StatusOK, `{"number":123,"title":"created"}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/api/v5/repos/owner/repo/pulls/123":
			return authTestResponse(http.StatusInternalServerError, `{"message":"temporary"}`), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	pr, err := CreatePullRequest(client, "owner", "repo", &CreatePROptions{
		Title: "created",
		Body:  "body text",
		Head:  "feature",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}
	if pr.Body != "" {
		t.Fatalf("response body = %q, want empty because remote body could not be verified", pr.Body)
	}
}

func TestUpdatePullRequestUsesFormEncoding(t *testing.T) {
	draft := false
	closeRelated := true

	var gotPath string
	var gotAuth string
	var gotContentType string
	var gotBody string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		gotContentType = req.Header.Get("Content-Type")
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		return authTestResponse(http.StatusOK, `{"number":123,"title":"updated"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := UpdatePullRequest(client, "owner", "repo", 123, &UpdatePROptions{
		Title:             "updated",
		Body:              "body text",
		State:             "open",
		StateEvent:        "reopen",
		Base:              "main",
		Draft:             &draft,
		MilestoneNumber:   5,
		Labels:            []string{"type/feature", "risk/medium"},
		CloseRelatedIssue: &closeRelated,
	})
	if err != nil {
		t.Fatalf("UpdatePullRequest() error = %v", err)
	}

	assertNoAccessTokenQuery(t, gotPath)
	if gotPath != "/api/v5/repos/owner/repo/pulls/123" {
		t.Fatalf("request path = %q, want %q", gotPath, "/api/v5/repos/owner/repo/pulls/123")
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want %q", gotContentType, "application/x-www-form-urlencoded")
	}

	expectedPairs := []string{
		"title=updated",
		"body=body+text",
		"state=open",
		"state_event=reopen",
		"base=main",
		"draft=false",
		"milestone_number=5",
		"labels%5B%5D=type%2Ffeature",
		"labels%5B%5D=risk%2Fmedium",
		"close_related_issue=true",
	}
	for _, pair := range expectedPairs {
		if !strings.Contains(gotBody, pair) {
			t.Fatalf("request body %q does not contain %q", gotBody, pair)
		}
	}
}

func TestBuildPRUpdateFormValuesNilOptions(t *testing.T) {
	formValues := buildPRUpdateFormValues(nil)
	if len(formValues) != 0 {
		t.Fatalf("expected empty form values, got %v", formValues)
	}
}

func TestBuildPRUpdateFormValuesExplicitLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{
			name:   "replace labels",
			labels: []string{" type/feature ", "risk/medium"},
			want:   "type/feature,risk/medium",
		},
		{
			name: "clear labels",
			want: ",",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formValues := buildPRUpdateFormValues(&UpdatePROptions{
				Labels:    tt.labels,
				LabelsSet: true,
			})
			if got := formValues.Get("labels"); got != tt.want {
				t.Fatalf("labels = %q, want %q", got, tt.want)
			}
			if _, ok := formValues["labels"]; !ok {
				t.Fatal("labels field is missing")
			}
			if _, ok := formValues["labels[]"]; ok {
				t.Fatal("labels[] should not be sent for an explicit label update")
			}
		})
	}
}

func TestListPRCommentsPaginatesUntilShortPage(t *testing.T) {
	var gotPaths []string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath := req.URL.Path
		if req.URL.RawQuery != "" {
			gotPath += "?" + req.URL.RawQuery
		}
		gotPaths = append(gotPaths, gotPath)

		query := req.URL.Query()
		if got := query.Get("per_page"); got != "100" {
			t.Fatalf("per_page = %q, want %q", got, "100")
		}

		switch query.Get("page") {
		case "1":
			return authTestResponse(http.StatusOK, prCommentsJSON(100, 0)), nil
		case "2":
			return authTestResponse(http.StatusOK, prCommentsJSON(2, 100)), nil
		default:
			t.Fatalf("unexpected page %q", query.Get("page"))
			return nil, nil
		}
	})

	comments, err := ListPRComments(client, "owner", "repo", 123)
	if err != nil {
		t.Fatalf("ListPRComments() error = %v", err)
	}

	if len(comments) != 102 {
		t.Fatalf("len(comments) = %d, want 102", len(comments))
	}
	if comments[0].Body != "comment-1" || comments[101].Body != "comment-102" {
		t.Fatalf("unexpected comment order: first=%q last=%q", comments[0].Body, comments[101].Body)
	}
	if len(gotPaths) != 2 {
		t.Fatalf("request count = %d, want 2", len(gotPaths))
	}
}

func TestListPRCommentsStopsAfterEmptyPage(t *testing.T) {
	var requests int

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		requests++
		if got := req.URL.Query().Get("page"); got != "1" {
			t.Fatalf("page = %q, want %q", got, "1")
		}
		if got := req.URL.Query().Get("per_page"); got != "100" {
			t.Fatalf("per_page = %q, want %q", got, "100")
		}
		return authTestResponse(http.StatusOK, `[]`), nil
	})

	comments, err := ListPRComments(client, "owner", "repo", 123)
	if err != nil {
		t.Fatalf("ListPRComments() error = %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("len(comments) = %d, want 0", len(comments))
	}
	if requests != 1 {
		t.Fatalf("request count = %d, want 1", requests)
	}
}

func TestPullRequestUnmarshal(t *testing.T) {
	jsonResp := `{
		"id": 8483763,
		"number": 95,
		"title": "feat: complete issue output contracts and view details",
		"body": "body",
		"state": "open",
		"html_url": "https://gitcode.com/gitcode-cli/cli/merge_requests/95",
		"draft": false,
		"created_at": "2026-04-07T10:20:21+08:00",
		"updated_at": "2026-04-07T11:08:27+08:00",
		"user": {"login": "aflyingto"},
		"labels": [{"name": "type/feature"}],
		"requested_reviewers": [{"login": "reviewer1"}]
	}`

	var pr PullRequest
	if err := json.Unmarshal([]byte(jsonResp), &pr); err != nil {
		t.Fatalf("Failed to unmarshal PullRequest: %v", err)
	}

	if pr.Number != 95 {
		t.Fatalf("Expected Number 95, got %d", pr.Number)
	}
	if pr.User == nil || pr.User.Login != "aflyingto" {
		t.Fatalf("Expected User.Login aflyingto, got %#v", pr.User)
	}
	if len(pr.Labels) != 1 || pr.Labels[0].Name != "type/feature" {
		t.Fatalf("Expected labels to include type/feature, got %#v", pr.Labels)
	}
	if len(pr.Reviewers) != 1 || pr.Reviewers[0].Login != "reviewer1" {
		t.Fatalf("Expected requested_reviewers to include reviewer1, got %#v", pr.Reviewers)
	}
}

func TestNormalizePullRequestMergedState(t *testing.T) {
	mergedAt := "2026-07-27T17:31:13+08:00"
	emptyMergedAt := ""
	tests := []struct {
		name string
		pr   PullRequest
		want bool
	}{
		{name: "explicit merged flag", pr: PullRequest{Merged: true}, want: true},
		{name: "merged state", pr: PullRequest{State: "merged"}, want: true},
		{name: "merged state case insensitive", pr: PullRequest{State: "MERGED"}, want: true},
		{name: "merged timestamp", pr: PullRequest{State: "closed", MergedAt: &mergedAt}, want: true},
		{name: "empty merged timestamp", pr: PullRequest{State: "closed", MergedAt: &emptyMergedAt}, want: false},
		{name: "open pull request", pr: PullRequest{State: "open"}, want: false},
		{name: "closed unmerged pull request", pr: PullRequest{State: "closed"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalState := tt.pr.State
			normalizePullRequest(&tt.pr)
			if tt.pr.Merged != tt.want {
				t.Fatalf("Merged = %v, want %v", tt.pr.Merged, tt.want)
			}
			if tt.pr.IsMerged() != tt.want {
				t.Fatalf("IsMerged() = %v, want %v", tt.pr.IsMerged(), tt.want)
			}
			if tt.pr.State != originalState {
				t.Fatalf("State = %q, want original state %q", tt.pr.State, originalState)
			}
		})
	}
}

func TestGetPullRequestNormalizesNullMergedField(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{
			"number":123,
			"state":"merged",
			"merged":null,
			"merged_at":"2026-07-27T17:31:13+08:00"
		}`), nil
	})

	pr, err := GetPullRequest(client, "owner", "repo", 123)
	if err != nil {
		t.Fatalf("GetPullRequest() error = %v", err)
	}
	if !pr.Merged {
		t.Fatalf("Merged = false, want true: %#v", pr)
	}
}

func TestGetPullRequestNormalizesDescription(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v5/repos/owner/repo/pulls/123" {
			t.Fatalf("request path = %q", req.URL.Path)
		}
		return authTestResponse(http.StatusOK, `{"number":123,"title":"PR","description":"description text"}`), nil
	})

	pr, err := GetPullRequest(client, "owner", "repo", 123)
	if err != nil {
		t.Fatalf("GetPullRequest() error = %v", err)
	}
	if pr.Body != "description text" || pr.Description != "description text" {
		t.Fatalf("body/description not normalized: %#v", pr)
	}
}

func TestListPRCommitsPaginatesAndReadsNestedMessage(t *testing.T) {
	var requests int
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/api/v5/repos/owner/repo/pulls/123/commits" {
			t.Fatalf("request path = %q", req.URL.Path)
		}
		if req.URL.Query().Get("per_page") != "100" {
			t.Fatalf("per_page = %q", req.URL.Query().Get("per_page"))
		}
		switch req.URL.Query().Get("page") {
		case "1":
			return authTestResponse(http.StatusOK, prCommitsJSON(100, "first page")), nil
		case "2":
			return authTestResponse(http.StatusOK, `[{"sha":"last","commit":{"message":"target message"}}]`), nil
		default:
			t.Fatalf("unexpected page %q", req.URL.Query().Get("page"))
			return nil, nil
		}
	})

	commits, err := ListPRCommits(client, "owner", "repo", 123)
	if err != nil {
		t.Fatalf("ListPRCommits() error = %v", err)
	}
	if len(commits) != 101 {
		t.Fatalf("len(commits) = %d, want 101", len(commits))
	}
	if got := commits[100].MessageText(); got != "target message" {
		t.Fatalf("MessageText() = %q", got)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestClosePullRequestUsesStateAndVerifiesClosedState(t *testing.T) {
	var requests []string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		if req.URL.RawQuery != "" {
			path += "?" + req.URL.RawQuery
		}
		requests = append(requests, req.Method+" "+path)

		switch len(requests) {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("request 1 method = %s, want GET", req.Method)
			}
			return authTestResponse(http.StatusOK, `{"number":123,"title":"updated","state":"open"}`), nil
		case 2:
			if req.Method != http.MethodPatch {
				t.Fatalf("request 2 method = %s, want PATCH", req.Method)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("ParseQuery() error = %v", err)
			}
			if got := values.Get("state"); got != "closed" {
				t.Fatalf("state = %q, want %q", got, "closed")
			}
			if got := values.Get("title"); got != "updated" {
				t.Fatalf("title = %q, want %q", got, "updated")
			}
			if got := values.Get("state_event"); got != "" {
				t.Fatalf("state_event = %q, want empty", got)
			}
			return authTestResponse(http.StatusOK, `{"number":123,"title":"updated","state":"closed"}`), nil
		default:
			t.Fatalf("unexpected extra request %d: %s", len(requests), requests[len(requests)-1])
			return nil, nil
		}
	})
	client.SetToken("test-token", "test")

	pr, err := ClosePullRequest(client, "owner", "repo", 123)
	if err != nil {
		t.Fatalf("ClosePullRequest() error = %v", err)
	}
	if pr.State != "closed" {
		t.Fatalf("pr.State = %q, want %q", pr.State, "closed")
	}
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
}

func TestClosePullRequestErrorsWhenVerificationStillOpen(t *testing.T) {
	var requests int

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			return authTestResponse(http.StatusOK, `{"number":123,"title":"updated","state":"open"}`), nil
		case 2:
			return authTestResponse(http.StatusOK, `{"number":123,"title":"updated","state":"open"}`), nil
		case 3:
			return authTestResponse(http.StatusOK, `{"number":123,"title":"updated","state":"open"}`), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})
	client.SetToken("test-token", "test")

	_, err := ClosePullRequest(client, "owner", "repo", 123)
	if err == nil {
		t.Fatal("expected ClosePullRequest() to return an error")
	}
	if !strings.Contains(err.Error(), "still open") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 3 {
		t.Fatalf("request count = %d, want 3", requests)
	}
}

func TestPRCommentAndReviewUnmarshal(t *testing.T) {
	commentJSON := `{
		"id": 1,
		"discussion_id": "thread-1",
		"body": "Looks good",
		"user": {"login": "reviewer1"},
		"comment_type": "discussion",
		"resolved": false,
		"diff_file": "pkg/cmd/pr/view/view.go",
		"created_at": "2026-04-07T10:20:21+08:00"
	}`
	reviewJSON := `{
		"id": 2,
		"user": {"login": "reviewer2"},
		"body": "LGTM",
		"state": "approved",
		"submitted_at": "2026-04-07T10:30:21+08:00"
	}`

	var comment PRComment
	if err := json.Unmarshal([]byte(commentJSON), &comment); err != nil {
		t.Fatalf("Failed to unmarshal PRComment: %v", err)
	}
	if comment.DiscussionID != "thread-1" || comment.User == nil || comment.User.Login != "reviewer1" {
		t.Fatalf("Unexpected comment payload: %#v", comment)
	}

	var review PRReview
	if err := json.Unmarshal([]byte(reviewJSON), &review); err != nil {
		t.Fatalf("Failed to unmarshal PRReview: %v", err)
	}
	if review.State != "approved" || review.User == nil || review.User.Login != "reviewer2" {
		t.Fatalf("Unexpected review payload: %#v", review)
	}
}

func assertPRListRequest(t *testing.T, gotPath, wantPath string, wantQuery map[string]string) {
	t.Helper()

	if gotPath == "" {
		t.Fatal("request path was empty")
	}
	if !strings.HasPrefix(gotPath, wantPath) {
		t.Fatalf("request path = %q, want prefix %q", gotPath, wantPath)
	}

	rawQuery := ""
	if len(gotPath) > len(wantPath) {
		rawQuery = strings.TrimPrefix(gotPath[len(wantPath):], "?")
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("url.ParseQuery() error = %v", err)
	}
	for key, want := range wantQuery {
		if got := query.Get(key); got != want {
			t.Fatalf("query[%q] = %q, want %q", key, got, want)
		}
	}
	if len(query) != len(wantQuery) {
		t.Fatalf("query = %#v, want %#v", query, wantQuery)
	}
}

func prCommentsJSON(count, offset int) string {
	comments := make([]PRComment, count)
	for i := range comments {
		comments[i].ID = offset + i + 1
		comments[i].Body = "comment-" + strconv.Itoa(offset+i+1)
	}

	data, err := json.Marshal(comments)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func prCommitsJSON(count int, message string) string {
	commits := make([]Commit, count)
	for i := range commits {
		commits[i].SHA = strconv.Itoa(i + 1)
		commits[i].Message = message
	}

	data, err := json.Marshal(commits)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestSetPRMergersBuildsPutRequest(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotBody = string(bodyBytes)
		return authTestResponse(http.StatusOK, `[{"id":1,"login":"user1","name":"User One","object_id":"o1"},{"id":2,"login":"user2","name":"User Two","object_id":"o2"}]`), nil
	})

	mergers, err := SetPRMergers(client, "owner", "repo", 123, []string{"user1", "user2"})
	if err != nil {
		t.Fatalf("SetPRMergers() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/pulls/123/mergers" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/pulls/123/mergers", gotPath)
	}
	if !strings.Contains(gotBody, `"mergers":"user1,user2"`) {
		t.Fatalf("body = %q, want mergers field with comma-separated users", gotBody)
	}
	if len(mergers) != 2 {
		t.Fatalf("len(mergers) = %d, want 2", len(mergers))
	}
	if mergers[0].Login != "user1" || mergers[0].ObjectID != "o1" {
		t.Fatalf("mergers[0] = %+v, want login user1 and object_id o1", mergers[0])
	}
}

func TestSetPRMergersAPIError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"Not Found"}`), nil
	})

	_, err := SetPRMergers(client, "owner", "repo", 999, []string{"user1"})
	if err == nil {
		t.Fatalf("SetPRMergers() expected error for 404 response")
	}
}
