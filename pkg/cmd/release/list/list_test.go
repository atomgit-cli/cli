package list

import (
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
	"io"
	"net/http"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "list default",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "list with limit",
			args:    []string{"--limit", "10"},
			wantErr: false,
		},
		{
			name:    "list with repo",
			args:    []string{"-R", "owner/repo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdList(f, func(opts *ListOptions) error {
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

func TestListRunMarksOnlyFirstPublishedReleaseAsLatest(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`[
							{"tag_name":"v2.0.0","html_url":"https://gitcode.com/owner/repo/-/releases/v2.0.0","draft":false,"prerelease":false,"created_at":"2026-06-01T00:00:00Z","published_at":"2026-06-01T00:00:00Z"},
							{"tag_name":"v1.9.0","html_url":"https://gitcode.com/owner/repo/-/releases/v1.9.0","draft":false,"prerelease":false,"created_at":"2026-05-01T00:00:00Z","published_at":"2026-05-01T00:00:00Z"},
							{"tag_name":"v2.1.0-rc1","html_url":"https://gitcode.com/owner/repo/-/releases/v2.1.0-rc1","draft":false,"prerelease":true,"created_at":"2026-06-05T00:00:00Z","published_at":"2026-06-05T00:00:00Z"}
						]`)),
					}, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	output := out.String()
	if strings.Count(output, "(latest)") != 1 {
		t.Fatalf("output = %q, want exactly one latest marker", output)
	}
	if !strings.Contains(output, "(published)") {
		t.Fatalf("output = %q, want published marker", output)
	}
	if !strings.Contains(output, "(pre-release)") {
		t.Fatalf("output = %q, want pre-release marker", output)
	}
}

func TestListRunSortsByPublishedAtDescending(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					// API returns releases in creation order (oldest first),
					// which is the default GitCode API behavior that causes #256.
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`[
							{"tag_name":"v0.2.3","html_url":"https://gitcode.com/owner/repo/-/releases/v0.2.3","draft":false,"prerelease":false,"created_at":"2026-03-01T00:00:00Z","published_at":"2026-03-02T00:00:00Z"},
							{"tag_name":"v0.3.1","html_url":"https://gitcode.com/owner/repo/-/releases/v0.3.1","draft":false,"prerelease":false,"created_at":"2026-04-01T00:00:00Z","published_at":"2026-04-02T00:00:00Z"},
							{"tag_name":"v0.5.9","html_url":"https://gitcode.com/owner/repo/-/releases/v0.5.9","draft":false,"prerelease":false,"created_at":"2026-06-01T00:00:00Z","published_at":"2026-06-01T00:00:00Z"},
							{"tag_name":"v0.4.0","html_url":"https://gitcode.com/owner/repo/-/releases/v0.4.0","draft":false,"prerelease":false,"created_at":"2026-05-01T00:00:00Z","published_at":"2026-05-01T00:00:00Z"}
						]`)),
					}, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	output := out.String()

	// Verify latest release (v0.5.9) appears first
	v059Pos := strings.Index(output, "v0.5.9")
	v040Pos := strings.Index(output, "v0.4.0")
	v031Pos := strings.Index(output, "v0.3.1")
	v023Pos := strings.Index(output, "v0.2.3")

	if v059Pos < 0 || v040Pos < 0 || v031Pos < 0 || v023Pos < 0 {
		t.Fatalf("output = %q, missing expected releases", output)
	}

	// Newest first: v0.5.9 > v0.4.0 > v0.3.1 > v0.2.3
	if v059Pos > v040Pos || v040Pos > v031Pos || v031Pos > v023Pos {
		t.Fatalf("output = %q, releases not sorted by newest first", output)
	}

	// v0.5.9 should be marked as (latest), not v0.2.3
	if !strings.Contains(output, "v0.5.9 (latest)") {
		t.Fatalf("output = %q, v0.5.9 should be marked as latest", output)
	}
}

func TestListRunSortReleasesByDateHandlesNilPublishedAt(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					// v0.2.3 has no published_at (should fall back to created_at)
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`[
							{"tag_name":"v0.2.3","html_url":"https://gitcode.com/owner/repo/-/releases/v0.2.3","draft":false,"prerelease":false,"created_at":"2026-03-01T00:00:00Z"},
							{"tag_name":"v0.5.9","html_url":"https://gitcode.com/owner/repo/-/releases/v0.5.9","draft":false,"prerelease":false,"created_at":"2026-06-01T00:00:00Z","published_at":"2026-06-01T00:00:00Z"}
						]`)),
					}, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	output := out.String()

	// v0.5.9 (newer) should appear before v0.2.3 (older)
	v059Pos := strings.Index(output, "v0.5.9")
	v023Pos := strings.Index(output, "v0.2.3")

	if v059Pos > v023Pos {
		t.Fatalf("output = %q, newer release should appear first even when older has nil published_at", output)
	}
}

func TestListRunSortReleasesByDateAllNilPublishedAt(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					// All releases have no published_at — must fall back to created_at.
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`[
							{"tag_name":"v0.2.3","html_url":"https://gitcode.com/owner/repo/-/releases/v0.2.3","draft":false,"prerelease":false,"created_at":"2026-03-01T00:00:00Z"},
							{"tag_name":"v0.5.9","html_url":"https://gitcode.com/owner/repo/-/releases/v0.5.9","draft":false,"prerelease":false,"created_at":"2026-06-01T00:00:00Z"},
							{"tag_name":"v0.4.0","html_url":"https://gitcode.com/owner/repo/-/releases/v0.4.0","draft":false,"prerelease":false,"created_at":"2026-05-01T00:00:00Z"}
						]`)),
					}, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	output := out.String()

	// When all PublishedAt are nil, sort falls back to CreatedAt:
	// Newest created first: v0.5.9 (Jun) > v0.4.0 (May) > v0.2.3 (Mar)
	v059Pos := strings.Index(output, "v0.5.9")
	v040Pos := strings.Index(output, "v0.4.0")
	v023Pos := strings.Index(output, "v0.2.3")

	if v059Pos < 0 || v040Pos < 0 || v023Pos < 0 {
		t.Fatalf("output = %q, missing expected releases", output)
	}
	if v059Pos > v040Pos || v040Pos > v023Pos {
		t.Fatalf("output = %q, releases not sorted by created_at when all published_at are nil", output)
	}

	// v0.5.9 should be marked as (latest)
	if !strings.Contains(output, "v0.5.9 (latest)") {
		t.Fatalf("output = %q, v0.5.9 should be marked as latest", output)
	}
}

func TestListRunSortReleasesByDatePublishedAtPriority(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out

	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					// v1.0.0 was created first but published last (backport release).
					// v2.0.0 was created later but published first.
					// PublishedAt must take priority over CreatedAt for sorting.
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`[
							{"tag_name":"v1.0.0","html_url":"https://gitcode.com/owner/repo/-/releases/v1.0.0","draft":false,"prerelease":false,"created_at":"2026-01-01T00:00:00Z","published_at":"2026-06-01T00:00:00Z"},
							{"tag_name":"v2.0.0","html_url":"https://gitcode.com/owner/repo/-/releases/v2.0.0","draft":false,"prerelease":false,"created_at":"2026-04-01T00:00:00Z","published_at":"2026-04-01T00:00:00Z"}
						]`)),
					}, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}

	output := out.String()

	// v1.0.0 published Jun > v2.0.0 published Apr, so v1.0.0 should appear first
	// even though v2.0.0 was created later.
	v100Pos := strings.Index(output, "v1.0.0")
	v200Pos := strings.Index(output, "v2.0.0")

	if v100Pos < 0 || v200Pos < 0 {
		t.Fatalf("output = %q, missing expected releases", output)
	}
	if v100Pos > v200Pos {
		t.Fatalf("output = %q, v1.0.0 should appear before v2.0.0 (published_at takes priority over created_at)", output)
	}

	// v1.0.0 published later, should be (latest)
	if !strings.Contains(output, "v1.0.0 (latest)") {
		t.Fatalf("output = %q, v1.0.0 should be marked as latest", output)
	}
}

func TestListRunUsesBaseRepoWhenRepoOmitted(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotPath string
	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotPath = req.URL.Path
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`[]`)),
					}, nil
				}),
			}, nil
		},
		BaseRepo: func() (string, error) {
			return "owner/repo", nil
		},
		Limit: 30,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if gotPath != "/api/v5/repos/owner/repo/releases" {
		t.Fatalf("request path = %q, want /api/v5/repos/owner/repo/releases", gotPath)
	}
}

// TestListRunRequestsDirectionDesc verifies that listRun requests the API with
// direction=desc so --limit receives the latest releases (#256).
func TestListRunRequestsDirectionDesc(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotQuery string
	err := listRun(&ListOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{
				Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					gotQuery = req.URL.RawQuery
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`[]`)),
					}, nil
				}),
			}, nil
		},
		Repository: "owner/repo",
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("listRun() error = %v", err)
	}
	if !strings.Contains(gotQuery, "direction=desc") {
		t.Errorf("expected direction=desc in query, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "per_page=5") {
		t.Errorf("expected per_page=5 in query, got %q", gotQuery)
	}
}
