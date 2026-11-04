package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestCreateIssueUsesAssigneeIDs(t *testing.T) {
	var gotBody url.Values

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		return authTestResponse(http.StatusOK, `{"number":"1","title":"created"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := CreateIssue(client, "owner", "repo", &CreateIssueOptions{
		Title:       "created",
		AssigneeIDs: []string{"4744798"},
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if gotBody.Get("assignee_id") != "4744798" {
		t.Fatalf("assignee_id = %q, want %q", gotBody.Get("assignee_id"), "4744798")
	}
	if got := gotBody["assignee_ids[]"]; len(got) != 1 || got[0] != "4744798" {
		t.Fatalf("assignee_ids[] = %v, want [%q]", got, "4744798")
	}
}

func TestCreateIssueUsesOwnerPathForAssigneeUsernames(t *testing.T) {
	var gotBody map[string]interface{}

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v5/repos/owner/issues" {
			t.Fatalf("request path = %s, want /api/v5/repos/owner/issues", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		return authTestResponse(http.StatusOK, `{"number":"2","title":"created"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := CreateIssue(client, "owner", "repo", &CreateIssueOptions{
		Title:     "created",
		Assignees: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if gotBody["assignee"] != "alice,bob" {
		t.Fatalf("assignee = %#v, want %q", gotBody["assignee"], "alice,bob")
	}
}

func TestCreateIssueUsesOwnerPathForAdvancedFields(t *testing.T) {
	var gotBody map[string]interface{}

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v5/repos/owner/issues" {
			t.Fatalf("request path = %s, want /api/v5/repos/owner/issues", req.URL.Path)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q, want application/json", got)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		return authTestResponse(http.StatusOK, `{"number":"2","title":"created"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := CreateIssue(client, "owner", "repo", &CreateIssueOptions{
		Title:         "created",
		Assignees:     []string{"alice", "bob"},
		Labels:        []string{"bug", "enhancement"},
		TemplatePath:  ".gitcode/ISSUE_TEMPLATE/feature.yaml",
		SecurityHole:  "true",
		IssueType:     "需求",
		IssueSeverity: "高",
		CustomFields: []map[string]interface{}{
			{"id": "field", "value": "demo"},
		},
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if gotBody["repo"] != "repo" {
		t.Fatalf("repo = %#v, want %q", gotBody["repo"], "repo")
	}
	if gotBody["assignee"] != "alice,bob" {
		t.Fatalf("assignee = %#v, want %q", gotBody["assignee"], "alice,bob")
	}
	if gotBody["labels"] != "bug,enhancement" {
		t.Fatalf("labels = %#v, want %q", gotBody["labels"], "bug,enhancement")
	}
	if gotBody["template_path"] != ".gitcode/ISSUE_TEMPLATE/feature.yaml" {
		t.Fatalf("template_path = %#v", gotBody["template_path"])
	}
	if gotBody["security_hole"] != "true" {
		t.Fatalf("security_hole = %#v, want %q", gotBody["security_hole"], "true")
	}
	fields, ok := gotBody["custom_fields"].([]interface{})
	if !ok || len(fields) != 1 {
		t.Fatalf("custom_fields = %#v, want one element", gotBody["custom_fields"])
	}
}

func TestCreateIssueOwnerPathFallsBackToAssigneeIDs(t *testing.T) {
	var gotBody map[string]interface{}

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v5/repos/owner/issues" {
			t.Fatalf("request path = %s, want /api/v5/repos/owner/issues", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		return authTestResponse(http.StatusOK, `{"number":"3","title":"created"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := CreateIssue(client, "owner", "repo", &CreateIssueOptions{
		Title:        "created",
		AssigneeIDs:  []string{"101", "202"},
		TemplatePath: ".gitcode/ISSUE_TEMPLATE/feature.yaml",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if gotBody["assignee"] != "101,202" {
		t.Fatalf("assignee = %#v, want %q", gotBody["assignee"], "101,202")
	}
}

func TestUpdateIssueUsesAssigneeIDs(t *testing.T) {
	var gotBody url.Values

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		return authTestResponse(http.StatusOK, `{"number":"1","title":"updated"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := UpdateIssue(client, "owner", "repo", 1, &UpdateIssueOptions{
		AssigneeIDs: []string{"4744798"},
	})
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}

	if gotBody.Get("assignee_id") != "4744798" {
		t.Fatalf("assignee_id = %q, want %q", gotBody.Get("assignee_id"), "4744798")
	}
	if got := gotBody["assignee_ids[]"]; len(got) != 1 || got[0] != "4744798" {
		t.Fatalf("assignee_ids[] = %v, want [%q]", got, "4744798")
	}
}

func TestUpdateIssueUsesAssigneeUsernames(t *testing.T) {
	var gotBody url.Values

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("ParseQuery() error = %v", err)
		}
		return authTestResponse(http.StatusOK, `{"number":"1","title":"updated"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := UpdateIssue(client, "owner", "repo", 1, &UpdateIssueOptions{
		Assignees: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
	if gotBody.Get("assignee") != "alice,bob" {
		t.Fatalf("assignee = %q, want %q", gotBody.Get("assignee"), "alice,bob")
	}
	if gotBody.Get("assignee_id") != "" || len(gotBody["assignee_ids[]"]) != 0 {
		t.Fatalf("legacy assignee ID fields unexpectedly sent: %v", gotBody)
	}
}
