package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestGitCodeUpdateReleaseOptions_JSON tests JSON serialization of GitCodeUpdateReleaseOptions
func TestGitCodeUpdateReleaseOptions_JSON(t *testing.T) {
	opts := &GitCodeUpdateReleaseOptions{
		Name:          "v1.0.0",
		Body:          "Release notes",
		ReleaseStatus: "pre",
	}

	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("Failed to marshal GitCodeUpdateReleaseOptions: %v", err)
	}

	// Verify required fields are present
	if !strings.Contains(string(data), `"name":"v1.0.0"`) {
		t.Errorf("Expected name field in JSON: %s", string(data))
	}
	if !strings.Contains(string(data), `"body":"Release notes"`) {
		t.Errorf("Expected body field in JSON: %s", string(data))
	}
	if !strings.Contains(string(data), `"release_status":"pre"`) {
		t.Errorf("Expected release_status field in JSON: %s", string(data))
	}
}

// TestGitCodeUpdateReleaseOptions_EmptyReleaseStatus tests that empty release_status is omitted
func TestGitCodeUpdateReleaseOptions_EmptyReleaseStatus(t *testing.T) {
	opts := &GitCodeUpdateReleaseOptions{
		Name: "v1.0.0",
		Body: "Release notes",
	}

	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("Failed to marshal GitCodeUpdateReleaseOptions: %v", err)
	}

	// Verify release_status is omitted when empty
	if strings.Contains(string(data), "release_status") {
		t.Errorf("Expected release_status to be omitted when empty: %s", string(data))
	}
}

func TestCreateReleaseOptionsReleaseStatusJSON(t *testing.T) {
	opts := &CreateReleaseOptions{
		TagName:       "v1.0.0-rc1",
		Name:          "v1.0.0 RC1",
		Prerelease:    true,
		ReleaseStatus: "pre",
	}

	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("Failed to marshal CreateReleaseOptions: %v", err)
	}

	if !strings.Contains(string(data), `"release_status":"pre"`) {
		t.Fatalf("Expected release_status field in JSON: %s", string(data))
	}
}

// TestUpdateReleaseByTagDirect_PatchPath tests that UpdateReleaseByTagDirect uses correct PATCH path
func TestUpdateReleaseByTagDirect_PatchPath(t *testing.T) {
	var gotMethod string
	var gotPath string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		return authTestResponse(http.StatusOK, `{"tag_name":"v1.0.0","name":"Updated","body":"Notes"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &GitCodeUpdateReleaseOptions{
		Name: "Updated",
		Body: "Notes",
	}

	_, err := UpdateReleaseByTagDirect(client, "owner", "repo", "v1.0.0", opts)
	if err != nil {
		t.Fatalf("UpdateReleaseByTagDirect() error = %v", err)
	}

	if gotMethod != "PATCH" {
		t.Errorf("Expected PATCH method, got %s", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/releases/v1.0.0" {
		t.Errorf("Expected path '/api/v5/repos/owner/repo/releases/v1.0.0', got %s", gotPath)
	}
}

// TestUpdateReleaseByTagDirect_TagEscaping tests that tags with slashes are properly escaped
func TestUpdateReleaseByTagDirect_TagEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		// Use req.URL.String() to get the encoded URL, not req.URL.Path which is decoded
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `{"tag_name":"release/v1.0.0","name":"Updated","body":"Notes"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &GitCodeUpdateReleaseOptions{
		Name: "Updated",
		Body: "Notes",
	}

	_, err := UpdateReleaseByTagDirect(client, "owner", "repo", "release/v1.0.0", opts)
	if err != nil {
		t.Fatalf("UpdateReleaseByTagDirect() error = %v", err)
	}

	// Verify the slash in the tag is escaped to %2F in the full URL
	expectedEscaped := "release%2Fv1.0.0"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped tag '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestUpdateReleaseByTagDirect_RequestBody tests that request body contains correct fields
func TestUpdateReleaseByTagDirect_RequestBody(t *testing.T) {
	var gotBody string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		gotBody = string(body)
		return authTestResponse(http.StatusOK, `{"tag_name":"v1.0.0","name":"Updated Title","body":"Updated Notes"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &GitCodeUpdateReleaseOptions{
		Name:          "Updated Title",
		Body:          "Updated Notes",
		ReleaseStatus: "pre",
	}

	_, err := UpdateReleaseByTagDirect(client, "owner", "repo", "v1.0.0", opts)
	if err != nil {
		t.Fatalf("UpdateReleaseByTagDirect() error = %v", err)
	}

	// Verify request body contains required fields
	if !strings.Contains(gotBody, `"name":"Updated Title"`) {
		t.Errorf("Expected name in request body: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"body":"Updated Notes"`) {
		t.Errorf("Expected body in request body: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"release_status":"pre"`) {
		t.Errorf("Expected release_status in request body: %s", gotBody)
	}
}

// TestUpdateReleaseByTagDirect_ReturnsRelease tests that response is parsed correctly
func TestUpdateReleaseByTagDirect_ReturnsRelease(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{
			"tag_name": "v1.0.0",
			"name": "Updated Title",
			"body": "Updated Notes",
			"prerelease": true,
			"html_url": "https://gitcode.com/owner/repo/releases/v1.0.0"
		}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &GitCodeUpdateReleaseOptions{
		Name:          "Updated Title",
		Body:          "Updated Notes",
		ReleaseStatus: "pre",
	}

	release, err := UpdateReleaseByTagDirect(client, "owner", "repo", "v1.0.0", opts)
	if err != nil {
		t.Fatalf("UpdateReleaseByTagDirect() error = %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Expected TagName 'v1.0.0', got '%s'", release.TagName)
	}
	if release.Name != "Updated Title" {
		t.Errorf("Expected Name 'Updated Title', got '%s'", release.Name)
	}
	if release.Body != "Updated Notes" {
		t.Errorf("Expected Body 'Updated Notes', got '%s'", release.Body)
	}
	if !release.Prerelease {
		t.Errorf("Expected Prerelease true, got false")
	}
}

// TestUpdateReleaseByTagDirect_Error tests error handling
func TestUpdateReleaseByTagDirect_Error(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"error_message":"release not found"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &GitCodeUpdateReleaseOptions{
		Name: "Updated",
		Body: "Notes",
	}

	_, err := UpdateReleaseByTagDirect(client, "owner", "repo", "nonexistent", opts)
	if err == nil {
		t.Fatal("Expected error for nonexistent release")
	}
}

// TestGetRelease_TagEscaping tests that GetRelease properly escapes tags with slashes
func TestGetRelease_TagEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `{"tag_name":"release/v1.0.0","name":"Test Release","body":"Notes"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := GetRelease(client, "owner", "repo", "release/v1.0.0")
	if err != nil {
		t.Fatalf("GetRelease() error = %v", err)
	}

	// Verify the slash in the tag is escaped to %2F in the full URL
	expectedEscaped := "release%2Fv1.0.0"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped tag '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestGetReleaseUploadURL_TagEscaping tests that GetReleaseUploadURL properly escapes tags with slashes
func TestGetReleaseUploadURL_TagEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `{"url":"https://uploads.example.test/file","headers":{}}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := GetReleaseUploadURL(client, "owner", "repo", "release/v1.0.0", "test.txt")
	if err != nil {
		t.Fatalf("GetReleaseUploadURL() error = %v", err)
	}

	// Verify the slash in the tag is escaped to %2F in the full URL
	expectedEscaped := "release%2Fv1.0.0"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped tag '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestDeleteReleaseByTagDirect_DeletePath tests that DeleteReleaseByTagDirect uses correct DELETE path
func TestDeleteReleaseByTagDirect_DeletePath(t *testing.T) {
	var gotMethod string
	var gotPath string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		return authTestResponse(http.StatusNoContent, ``), nil
	})
	client.SetToken("test-token", "test")

	err := DeleteReleaseByTagDirect(client, "owner", "repo", "v1.0.0")
	if err != nil {
		t.Fatalf("DeleteReleaseByTagDirect() error = %v", err)
	}

	if gotMethod != "DELETE" {
		t.Errorf("Expected DELETE method, got %s", gotMethod)
	}
	if gotPath != "/api/v5/repos/owner/repo/releases/v1.0.0" {
		t.Errorf("Expected path '/api/v5/repos/owner/repo/releases/v1.0.0', got %s", gotPath)
	}
}

// TestDeleteReleaseByTagDirect_TagEscaping tests that tags with slashes are properly escaped
func TestDeleteReleaseByTagDirect_TagEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusNoContent, ``), nil
	})
	client.SetToken("test-token", "test")

	err := DeleteReleaseByTagDirect(client, "owner", "repo", "release/v1.0.0")
	if err != nil {
		t.Fatalf("DeleteReleaseByTagDirect() error = %v", err)
	}

	expectedEscaped := "release%2Fv1.0.0"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped tag '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestIsNotFoundError tests the isNotFoundError helper
func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"API 404 error", &APIError{StatusCode: 404}, true},
		{"API 403 error", &APIError{StatusCode: 403}, false},
		{"non-API error", ErrNoReleaseID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFoundError(tt.err); got != tt.want {
				t.Errorf("isNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestListReleases_SendsDirectionWhenSet verifies that ListReleases sends the
// direction query parameter when ReleaseListOptions.Direction is set, so the
// API returns newest releases first (#256).
func TestListReleases_SendsDirectionWhenSet(t *testing.T) {
	var gotURL string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListReleases(client, "owner", "repo", &ReleaseListOptions{
		PerPage:   5,
		Direction: "desc",
	})
	if err != nil {
		t.Fatalf("ListReleases() error = %v", err)
	}

	if !strings.Contains(gotURL, "direction=desc") {
		t.Errorf("expected direction=desc in URL, got %s", gotURL)
	}
	if !strings.Contains(gotURL, "per_page=5") {
		t.Errorf("expected per_page=5 in URL, got %s", gotURL)
	}
}

// TestListReleases_OmitsDirectionWhenEmpty verifies that an empty Direction
// does not append direction to the query string (#256).
func TestListReleases_OmitsDirectionWhenEmpty(t *testing.T) {
	var gotURL string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListReleases(client, "owner", "repo", &ReleaseListOptions{
		PerPage: 5,
	})
	if err != nil {
		t.Fatalf("ListReleases() error = %v", err)
	}

	if strings.Contains(gotURL, "direction") {
		t.Errorf("expected no direction in URL when empty, got %s", gotURL)
	}
}

// TestBuildPath_QueryParams is a table-driven test for buildPath parameter
// assembly, covering nil, empty, and combined options including Direction.
func TestBuildPath_QueryParams(t *testing.T) {
	tests := []struct {
		name string
		base string
		opts *ReleaseListOptions
		want string
	}{
		{
			name: "nil opts",
			base: "/repos/o/r/releases",
			opts: nil,
			want: "/repos/o/r/releases",
		},
		{
			name: "empty opts",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{},
			want: "/repos/o/r/releases",
		},
		{
			name: "per_page only",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{PerPage: 5},
			want: "/repos/o/r/releases?per_page=5",
		},
		{
			name: "page only",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{Page: 2},
			want: "/repos/o/r/releases?page=2",
		},
		{
			name: "direction trims whitespace",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{Direction: "  desc  "},
			want: "/repos/o/r/releases?direction=desc",
		},
		{
			name: "per_page and direction",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{PerPage: 5, Direction: "desc"},
			want: "/repos/o/r/releases?direction=desc&per_page=5",
		},
		{
			name: "all params",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{PerPage: 10, Page: 2, Direction: "desc"},
			want: "/repos/o/r/releases?direction=desc&page=2&per_page=10",
		},
		{
			name: "zero per_page omitted",
			base: "/repos/o/r/releases",
			opts: &ReleaseListOptions{PerPage: 0, Direction: "desc"},
			want: "/repos/o/r/releases?direction=desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPath(tt.base, tt.opts)
			if got != tt.want {
				t.Errorf("buildPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFmtErrorPreservesErrorChain verifies that fmtError wraps the cause so
// errors.Unwrap/As can reach the original API error (#321).
func TestFmtErrorPreservesErrorChain(t *testing.T) {
	original := &APIError{StatusCode: 500, Message: "server error"}
	err := fmtError(original, "failed to get upload URL")

	if unwrapped := errors.Unwrap(err); unwrapped != original {
		t.Errorf("errors.Unwrap() = %v, want %v", unwrapped, original)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("errors.As(err, &apiErr) = false, want true")
	}
	if apiErr != original {
		t.Errorf("errors.As matched %v, want %v", apiErr, original)
	}

	if !strings.Contains(err.Error(), "failed to get upload URL") || !strings.Contains(err.Error(), "server error") {
		t.Errorf("Error() = %q, want contains both msg and cause", err.Error())
	}
}

// TestFmtErrorNilCauseNoUnwrap verifies that fmtError(nil, msg) has no cause
// to unwrap and the message stands alone (#321).
func TestFmtErrorNilCauseNoUnwrap(t *testing.T) {
	err := fmtError(nil, "upload URL is empty")

	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		t.Errorf("errors.Unwrap() = %v, want nil for nil cause", unwrapped)
	}
	if want := "upload URL is empty"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}
