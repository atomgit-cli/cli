package testutil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"

	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// NewTestIOStreams creates IOStreams for testing
func NewTestIOStreams() (*iostreams.IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &iostreams.IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}, in, out, errOut
}

// NewTestHTTPClient creates an HTTP client for testing with a mock server
func NewTestHTTPClient(handler http.Handler) *http.Client {
	server := httptest.NewServer(handler)
	return &http.Client{
		Transport: &testTransport{server: server},
	}
}

type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.server.URL[:4] // http
	req.URL.Host = t.server.URL[7:]   // remove http://
	return http.DefaultTransport.RoundTrip(req)
}

// SetTestToken sets test environment token
func SetTestToken() func() {
	os.Setenv("GC_TOKEN", "test-token")
	return func() {
		os.Unsetenv("GC_TOKEN")
	}
}

// MockAPIHandler creates a mock API handler for testing
func MockAPIHandler() http.Handler {
	mux := http.NewServeMux()

	// User endpoints
	mux.HandleFunc("/api/v5/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "1", "login": "testuser", "name": "Test User"}`))
	})

	// Repository endpoints
	mux.HandleFunc("/api/v5/repos/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": "1", "name": "test-repo", "full_name": "owner/test-repo", "html_url": "https://gitcode.com/owner/test-repo"}`))
		} else if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// User repos list and create
	mux.HandleFunc("/api/v5/user/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "1", "name": "new-repo", "full_name": "owner/new-repo", "html_url": "https://gitcode.com/owner/new-repo"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "name": "test-repo", "full_name": "owner/test-repo", "html_url": "https://gitcode.com/owner/test-repo"}]`))
		}
	})

	// Issue endpoints
	mux.HandleFunc("/api/v5/repos/owner/test-repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "number": 1, "title": "Test Issue", "state": "open", "html_url": "https://gitcode.com/owner/test-repo/issues/1"}]`))
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "1", "number": 1, "title": "New Issue", "state": "open", "html_url": "https://gitcode.com/owner/test-repo/issues/1"}`))
		}
	})

	mux.HandleFunc("/api/v5/repos/owner/test-repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"number":"1","title":"Test Issue","state":"open","body":"Issue body","html_url":"https://gitcode.com/owner/test-repo/issues/1","labels":[{"id":1,"name":"enhancement","color":"00ff00"}]}`))
		} else if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"number":"1","title":"Updated Issue","state":"closed","html_url":"https://gitcode.com/owner/test-repo/issues/1","labels":[{"id":1,"name":"bug","color":"ff0000"}]}`))
		}
	})

	mux.HandleFunc("/api/v5/repos/owner/test-repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "body": "Test comment"}]`))
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "1", "body": "New comment"}`))
		}
	})

	// PR endpoints
	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "number": 1, "title": "Test PR", "state": "open", "html_url": "https://gitcode.com/owner/test-repo/pull/1"}]`))
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "1", "number": 1, "title": "New PR", "state": "open", "html_url": "https://gitcode.com/owner/test-repo/pull/1"}`))
		}
	})

	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": "1", "number": 1, "title": "Test PR", "state": "open", "body": "PR body", "head": {"ref": "feature"}, "base": {"ref": "main"}, "html_url": "https://gitcode.com/owner/test-repo/pull/1"}`))
		} else if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": "1", "number": 1, "title": "Updated PR", "state": "closed", "html_url": "https://gitcode.com/owner/test-repo/pull/1"}`))
		}
	})

	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/1/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "1", "number": 1, "merged": true}`))
	})

	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/1/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"2","body":"LGTM","discussion_id":"d2"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "body": "Test comment"}]`))
		}
	})

	// Label endpoints
	mux.HandleFunc("/api/v5/repos/owner/test-repo/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "name": "bug", "color": "ff0000"}]`))
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "1", "name": "bug", "color": "ff0000"}`))
		}
	})

	// Milestone endpoints
	mux.HandleFunc("/api/v5/repos/owner/test-repo/milestones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": "1", "number": 1, "title": "v1.0", "state": "open"}]`))
		} else if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": "1", "number": 1, "title": "v1.0", "state": "open"}`))
		}
	})

	mux.HandleFunc("/api/v5/repos/owner/test-repo/milestones/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"number":1,"title":"v1.1","state":"closed","description":"Updated"}`))
		} else if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"number":1,"title":"v1.0","state":"open","description":"First release"}`))
		}
	})

	// Commit list endpoint
	mux.HandleFunc("/api/v5/repos/owner/test-repo/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"1","body":"nice fix","discussion_id":"d1"}]`))
	})

	// Commit comment update
	mux.HandleFunc("/api/v5/repos/owner/test-repo/comments/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"1","body":"updated body","discussion_id":"d1"}`))
	})

	// Commit endpoint (subtree match for /commits/<sha>)
	mux.HandleFunc("/api/v5/repos/owner/test-repo/commits/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sha":"abc123","html_url":"https://gitcode.com/owner/test-repo/commit/abc123","commit":{"message":"fix: bug","author":{"name":"dev","email":"dev@test.com","date":"2026-01-01T00:00:00Z"}},"author":{"id":1,"login":"dev","avatar_url":"https://gitcode.com/avatar.png"},"stats":{"total":1,"additions":10,"deletions":2}}`))
	})

	// Commit comments for specific commit
	mux.HandleFunc("/api/v5/repos/owner/test-repo/commits/abc123/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"2","body":"nice","discussion_id":"d2"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"1","body":"nice fix","discussion_id":"d1"}]`))
		}
	})

	// Issue comment update/delete
	mux.HandleFunc("/api/v5/repos/owner/test-repo/issues/comments/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"body":"updated comment"}`))
		} else if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// UpdateIssue path (repo-less, used by AddIssueLabels internally)
	mux.HandleFunc("/api/v5/repos/owner/issues/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"number":"1","title":"Test Issue","state":"open","labels":[{"id":1,"name":"bug","color":"ff0000"}]}`))
	})

	// Issue labels (add/update)
	mux.HandleFunc("/api/v5/repos/owner/test-repo/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"name":"bug","color":"ff0000"}]`))
	})

	// Delete label
	mux.HandleFunc("/api/v5/repos/owner/test-repo/labels/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})

	// PR test endpoint
	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/1/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// PR comment edit / delete
	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/comments/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"1","body":"edited comment","discussion_id":"d1"}`))
		}
	})

	// PR discussion reply
	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/1/discussions/d1/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"2","noteId":2,"body":"reply"}`))
	})

	// Resolve PR comment
	mux.HandleFunc("/api/v5/repos/owner/test-repo/pulls/1/comments/d1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return mux
}
