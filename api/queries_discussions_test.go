package api

import (
	"net/http"
	"testing"
)

func TestListOrgDiscussionsBuildsPathAndParams(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.RawQuery
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `[]`), nil
	})

	discussions, err := ListOrgDiscussions(client, "my-org", &ListOrgDiscussionsOptions{
		Page:      2,
		PerPage:   50,
		Sort:      "comment_size",
		Direction: "asc",
		Search:    "release",
	})
	if err != nil {
		t.Fatalf("ListOrgDiscussions() error = %v", err)
	}
	if len(discussions) != 0 {
		t.Fatalf("len(discussions) = %d, want 0 for empty array", len(discussions))
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	// Path must target the v5 orgs discuss endpoint (per gc-api-doc).
	if gotPath != "/api/v5/orgs/my-org/discuss" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss", gotPath)
	}
	// Query params must match the official OpenAPI names.
	for _, want := range []string{"page=2", "per_page=50", "sort=comment_size", "direction=asc", "search=release"} {
		if !containsParam(gotQuery, want) {
			t.Fatalf("query = %q, want to contain %q", gotQuery, want)
		}
	}
}

func TestListOrgDiscussionsNoParamsOmitsQuery(t *testing.T) {
	var gotQuery string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotQuery = req.URL.RawQuery
		return authTestResponse(http.StatusOK, `[]`), nil
	})

	if _, err := ListOrgDiscussions(client, "my-org", nil); err != nil {
		t.Fatalf("ListOrgDiscussions() error = %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("query = %q, want empty when no options", gotQuery)
	}
}

func TestListOrgDiscussionsParsesResponse(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `[{"id":"abc","number":7,"title":"release plan","comment_total":3,"is_closed":0}]`), nil
	})

	discussions, err := ListOrgDiscussions(client, "my-org", &ListOrgDiscussionsOptions{PerPage: 20})
	if err != nil {
		t.Fatalf("ListOrgDiscussions() error = %v", err)
	}
	if len(discussions) != 1 {
		t.Fatalf("len = %d, want 1", len(discussions))
	}
	d := discussions[0]
	if d.ID != "abc" || d.Number != 7 || d.Title != "release plan" || d.CommentTotal != 3 {
		t.Fatalf("discussion = %+v, want id=abc number=7 title=release plan comment_total=3", d)
	}
}

func TestGetOrgDiscussionBuildsPath(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"abc","number":42,"title":"q","md_content":"body"}`), nil
	})

	d, err := GetOrgDiscussion(client, "my-org", 42)
	if err != nil {
		t.Fatalf("GetOrgDiscussion() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/42" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/42", gotPath)
	}
	if d.ID != "abc" || d.Number != 42 || d.Title != "q" || d.MdContent != "body" {
		t.Fatalf("discussion = %+v, want id=abc number=42 title=q md_content=body", d)
	}
}

func TestGetOrgDiscussionNotFoundReturnsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})

	_, err := GetOrgDiscussion(client, "my-org", 999)
	if err == nil {
		t.Fatal("GetOrgDiscussion() error = nil, want error for 404")
	}
}

func TestListRepoDiscussionsParsesResponse(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `[{"id":"a","number":7,"title":"release plan","comment_total":3,"is_closed":0}]`), nil
	})

	discussions, err := ListRepoDiscussions(client, "owner", "repo", &ListOrgDiscussionsOptions{PerPage: 20})
	if err != nil {
		t.Fatalf("ListRepoDiscussions() error = %v", err)
	}
	if len(discussions) != 1 {
		t.Fatalf("len = %d, want 1", len(discussions))
	}
	d := discussions[0]
	if d.ID != "a" || d.Number != 7 || d.Title != "release plan" || d.CommentTotal != 3 {
		t.Fatalf("discussion = %+v, want id=a number=7 title=release plan comment_total=3", d)
	}
}

func TestListRepoDiscussionsBuildsPathAndParams(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.RawQuery
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `[]`), nil
	})

	if _, err := ListRepoDiscussions(client, "owner", "repo", &ListOrgDiscussionsOptions{
		Page: 2, PerPage: 50, Sort: "comment_size", Direction: "asc", Search: "release",
	}); err != nil {
		t.Fatalf("ListRepoDiscussions() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss", gotPath)
	}
	for _, want := range []string{"page=2", "per_page=50", "sort=comment_size", "direction=asc", "search=release"} {
		if !containsParam(gotQuery, want) {
			t.Fatalf("query = %q, want to contain %q", gotQuery, want)
		}
	}
}

func TestGetRepoDiscussionBuildsPath(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"a","number":42,"title":"q","md_content":"body"}`), nil
	})

	d, err := GetRepoDiscussion(client, "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetRepoDiscussion() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/42" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/42", gotPath)
	}
	if d.ID != "a" || d.Number != 42 || d.Title != "q" || d.MdContent != "body" {
		t.Fatalf("discussion = %+v, want id=a number=42 title=q md_content=body", d)
	}
}

// containsParam checks that the url-encoded query contains the given k=v pair.
func containsParam(query, want string) bool {
	for _, p := range splitQuery(query) {
		if p == want {
			return true
		}
	}
	return false
}

func splitQuery(query string) []string {
	if query == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '&' {
			out = append(out, query[start:i])
			start = i + 1
		}
	}
	out = append(out, query[start:])
	return out
}

func TestListRepoDiscussionsNoParamsOmitsQuery(t *testing.T) {
	var gotQuery string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotQuery = req.URL.RawQuery
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	if _, err := ListRepoDiscussions(client, "owner", "repo", nil); err != nil {
		t.Fatalf("ListRepoDiscussions() error = %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("query = %q, want empty when no options", gotQuery)
	}
}

func TestGetRepoDiscussionNotFoundReturnsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})
	if _, err := GetRepoDiscussion(client, "owner", "repo", 999); err == nil {
		t.Fatal("GetRepoDiscussion() error = nil, want error for 404")
	}
}

func TestListOrgDiscussionCommentsBuildsPathAndParams(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.RawQuery
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	if _, err := ListOrgDiscussionComments(client, "my-org", 7, &ListDiscussionCommentsOptions{
		Page: 1, PerPage: 50, Order: "hot_desc",
	}); err != nil {
		t.Fatalf("ListOrgDiscussionComments() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7/comment" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7/comment", gotPath)
	}
	for _, want := range []string{"page=1", "per_page=50", "order=hot_desc"} {
		if !containsParam(gotQuery, want) {
			t.Fatalf("query = %q, want to contain %q", gotQuery, want)
		}
	}
}

func TestListOrgDiscussionCommentRepliesBuildsPath(t *testing.T) {
	var gotPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return authTestResponse(http.StatusOK, `[{"id":"c1","content":"reply","author":{"login":"alice"}}]`), nil
	})
	rs, err := ListOrgDiscussionCommentReplies(client, "my-org", 7, "c1", &ListDiscussionCommentsOptions{Page: 2})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7/comment/c1/reply" {
		t.Fatalf("path = %q, want .../comment/c1/reply", gotPath)
	}
	if len(rs) != 1 || rs[0].ID != "c1" || rs[0].Content != "reply" {
		t.Fatalf("replies = %+v, want id=c1 content=reply", rs)
	}
}

func TestListRepoDiscussionCommentsBuildsPathAndParams(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotQuery = req.URL.RawQuery
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `[{"id":"c","content":"hi","author":{"login":"bob"},"like_total":2,"reply_total":1}]`), nil
	})
	cs, err := ListRepoDiscussionComments(client, "owner", "repo", 3, &ListDiscussionCommentsOptions{Order: "time_desc"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/3/comment" {
		t.Fatalf("path = %q, want .../discuss/3/comment", gotPath)
	}
	if !containsParam(gotQuery, "order=time_desc") {
		t.Fatalf("query = %q, want to contain order=time_desc", gotQuery)
	}
	if len(cs) != 1 || cs[0].ID != "c" || cs[0].LikeTotal != 2 {
		t.Fatalf("comments = %+v, want id=c like_total=2", cs)
	}
}

func TestListRepoDiscussionCommentRepliesBuildsPath(t *testing.T) {
	var gotPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	if _, err := ListRepoDiscussionCommentReplies(client, "owner", "repo", 3, "c1", nil); err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/3/comment/c1/reply" {
		t.Fatalf("path = %q, want .../discuss/3/comment/c1/reply", gotPath)
	}
}

// --- Write operation tests ---

func TestCreateOrgDiscussionBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"new1","number":1,"title":"hello"}`), nil
	})
	d, err := CreateOrgDiscussion(client, "my-org", &CreateOrgDiscussionOptions{Title: "hello", MdContent: "body"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss", gotPath)
	}
	if d.ID != "new1" || d.Number != 1 || d.Title != "hello" {
		t.Fatalf("discussion = %+v, want id=new1 number=1 title=hello", d)
	}
}

func TestUpdateOrgDiscussionBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"a","number":7,"title":"updated"}`), nil
	})
	d, err := UpdateOrgDiscussion(client, "my-org", 7, &UpdateOrgDiscussionOptions{Title: "updated"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7", gotPath)
	}
	if d.Title != "updated" {
		t.Fatalf("title = %q, want updated", d.Title)
	}
}

func TestDeleteOrgDiscussionBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	if err := DeleteOrgDiscussion(client, "my-org", 7); err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7", gotPath)
	}
}

func TestCreateOrgDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"c1","md_content":"hi"}`), nil
	})
	c, err := CreateOrgDiscussionComment(client, "my-org", 7, &CreateOrgDiscussionCommentOptions{MdContent: "hi"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7/comment" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7/comment", gotPath)
	}
	if c.ID != "c1" || c.MdContent != "hi" {
		t.Fatalf("comment = %+v, want id=c1 md_content=hi", c)
	}
}

func TestUpdateOrgDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"c1","md_content":"edited"}`), nil
	})
	c, err := UpdateOrgDiscussionComment(client, "my-org", 7, "c1", &UpdateOrgDiscussionCommentOptions{MdContent: "edited"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7/comment/c1" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7/comment/c1", gotPath)
	}
	if c.MdContent != "edited" {
		t.Fatalf("md_content = %q, want edited", c.MdContent)
	}
}

func TestDeleteOrgDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	if err := DeleteOrgDiscussionComment(client, "my-org", 7, "c1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7/comment/c1" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7/comment/c1", gotPath)
	}
}

func TestReplyOrgDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"r1","md_content":"reply"}`), nil
	})
	c, err := ReplyOrgDiscussionComment(client, "my-org", 7, "c1", &ReplyOrgDiscussionCommentOptions{MdContent: "reply"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v5/orgs/my-org/discuss/7/comment/c1/reply" {
		t.Fatalf("path = %q, want /api/v5/orgs/my-org/discuss/7/comment/c1/reply", gotPath)
	}
	if c.ID != "r1" || c.MdContent != "reply" {
		t.Fatalf("comment = %+v, want id=r1 md_content=reply", c)
	}
}

func TestCreateRepoDiscussionBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"new1","number":1,"title":"hello"}`), nil
	})
	d, err := CreateRepoDiscussion(client, "owner", "repo", &CreateRepoDiscussionOptions{Title: "hello"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss", gotPath)
	}
	if d.ID != "new1" || d.Title != "hello" {
		t.Fatalf("discussion = %+v, want id=new1 title=hello", d)
	}
}

func TestUpdateRepoDiscussionBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"a","number":7,"title":"updated"}`), nil
	})
	d, err := UpdateRepoDiscussion(client, "owner", "repo", 7, &UpdateRepoDiscussionOptions{Title: "updated"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/7" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/7", gotPath)
	}
	if d.Title != "updated" {
		t.Fatalf("title = %q, want updated", d.Title)
	}
}

func TestDeleteRepoDiscussionBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	if err := DeleteRepoDiscussion(client, "owner", "repo", 7); err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/7" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/7", gotPath)
	}
}

func TestCreateRepoDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"c1","md_content":"hi"}`), nil
	})
	c, err := CreateRepoDiscussionComment(client, "owner", "repo", 7, &CreateRepoDiscussionCommentOptions{MdContent: "hi"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/7/comment" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/7/comment", gotPath)
	}
	if c.ID != "c1" {
		t.Fatalf("comment = %+v, want id=c1", c)
	}
}

func TestUpdateRepoDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"c1","md_content":"edited"}`), nil
	})
	c, err := UpdateRepoDiscussionComment(client, "owner", "repo", 7, "c1", &UpdateRepoDiscussionCommentOptions{MdContent: "edited"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/7/comment/c1" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/7/comment/c1", gotPath)
	}
	if c.MdContent != "edited" {
		t.Fatalf("md_content = %q, want edited", c.MdContent)
	}
}

func TestDeleteRepoDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	if err := DeleteRepoDiscussionComment(client, "owner", "repo", 7, "c1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/7/comment/c1" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/7/comment/c1", gotPath)
	}
}

func TestReplyRepoDiscussionCommentBuildsRequest(t *testing.T) {
	var gotPath, gotMethod string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		return authTestResponse(http.StatusOK, `{"id":"r1","md_content":"reply"}`), nil
	})
	c, err := ReplyRepoDiscussionComment(client, "owner", "repo", 7, "c1", &ReplyRepoDiscussionCommentOptions{MdContent: "reply"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/discuss/7/comment/c1/reply" {
		t.Fatalf("path = %q, want /api/v5/repos/owner/repo/discuss/7/comment/c1/reply", gotPath)
	}
	if c.ID != "r1" || c.MdContent != "reply" {
		t.Fatalf("comment = %+v, want id=r1 md_content=reply", c)
	}
}
