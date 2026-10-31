package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestUpdateLabel_LabelEscaping tests that UpdateLabel properly escapes labels with slashes
func TestUpdateLabel_LabelEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `{"name":"bug/critical","color":"red"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &UpdateLabelOptions{Color: "blue"}
	_, err := UpdateLabel(client, "owner", "repo", "bug/critical", opts)
	if err != nil {
		t.Fatalf("UpdateLabel() error = %v", err)
	}

	// Verify the slash in the label is escaped to %2F in the full URL
	expectedEscaped := "bug%2Fcritical"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped label '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestDeleteLabel_LabelEscaping tests that DeleteLabel properly escapes labels with slashes
func TestDeleteLabel_LabelEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusNoContent, ""), nil
	})
	client.SetToken("test-token", "test")

	err := DeleteLabel(client, "owner", "repo", "status/verified")
	if err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}

	// Verify the slash in the label is escaped to %2F in the full URL
	expectedEscaped := "status%2Fverified"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped label '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestRemoveLabelFromIssue_LabelEscaping tests that RemoveLabelFromIssue properly escapes labels with slashes
func TestRemoveLabelFromIssue_LabelEscaping(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusNoContent, ""), nil
	})
	client.SetToken("test-token", "test")

	err := RemoveLabelFromIssue(client, "owner", "repo", 123, "type/bug")
	if err != nil {
		t.Fatalf("RemoveLabelFromIssue() error = %v", err)
	}

	// Verify the slash in the label is escaped to %2F in the full URL
	expectedEscaped := "type%2Fbug"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped label '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}

// TestUpdateLabel_NormalLabel tests that UpdateLabel works with normal labels
func TestUpdateLabel_NormalLabel(t *testing.T) {
	var gotPath string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return authTestResponse(http.StatusOK, `{"name":"bug","color":"blue"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &UpdateLabelOptions{Color: "blue"}
	_, err := UpdateLabel(client, "owner", "repo", "bug", opts)
	if err != nil {
		t.Fatalf("UpdateLabel() error = %v", err)
	}

	// Verify normal label is in the path
	expectedPath := "/api/v5/repos/owner/repo/labels/bug"
	if gotPath != expectedPath {
		t.Errorf("Expected path '%s', got %s", expectedPath, gotPath)
	}
}

// TestRemoveLabelFromIssue_NormalLabel tests that RemoveLabelFromIssue works with normal labels
func TestRemoveLabelFromIssue_NormalLabel(t *testing.T) {
	var gotPath string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return authTestResponse(http.StatusNoContent, ""), nil
	})
	client.SetToken("test-token", "test")

	err := RemoveLabelFromIssue(client, "owner", "repo", 123, "bug")
	if err != nil {
		t.Fatalf("RemoveLabelFromIssue() error = %v", err)
	}

	// Verify normal label is in the path
	expectedPath := "/api/v5/repos/owner/repo/issues/123/labels/bug"
	if gotPath != expectedPath {
		t.Errorf("Expected path '%s', got %s", expectedPath, gotPath)
	}
}

// TestDeleteLabel_NormalLabel tests that DeleteLabel works with normal labels
func TestDeleteLabel_NormalLabel(t *testing.T) {
	var gotPath string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		return authTestResponse(http.StatusNoContent, ""), nil
	})
	client.SetToken("test-token", "test")

	err := DeleteLabel(client, "owner", "repo", "feature")
	if err != nil {
		t.Fatalf("DeleteLabel() error = %v", err)
	}

	// Verify normal label is in the path
	expectedPath := "/api/v5/repos/owner/repo/labels/feature"
	if gotPath != expectedPath {
		t.Errorf("Expected path '%s', got %s", expectedPath, gotPath)
	}
}

// TestCreateLabel_SlashLabel tests that CreateLabel works with slash labels using QueryEscape
func TestCreateLabel_SlashLabel(t *testing.T) {
	var gotURL string

	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL.String()
		return authTestResponse(http.StatusOK, `{"name":"risk/high","color":"red"}`), nil
	})
	client.SetToken("test-token", "test")

	opts := &CreateLabelOptions{Name: "risk/high", Color: "red"}
	_, err := CreateLabel(client, "owner", "repo", opts)
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}

	// CreateLabel uses QueryEscape for query parameters, not PathEscape
	// Verify the slash in the label name is escaped in query parameter
	expectedEscaped := "name=risk%2Fhigh"
	if !strings.Contains(gotURL, expectedEscaped) {
		t.Errorf("Expected escaped query param '%s' in URL, got %s", expectedEscaped, gotURL)
	}
}
