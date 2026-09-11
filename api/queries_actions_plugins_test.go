package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestListActionsPluginsBuildsV2Query(t *testing.T) {
	var gotHost string
	var gotEscapedPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotHost = req.URL.Host
		gotEscapedPath = req.URL.EscapedPath()
		if req.URL.RawQuery != "" {
			gotEscapedPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListActionsPlugins(client, "owner/repo", &ActionsListPluginsOptions{
		PerPage: 50,
		Page:    2,
	})
	if err != nil {
		t.Fatalf("ListActionsPlugins() error = %v", err)
	}

	if gotHost != WebAPIHost {
		t.Fatalf("host = %q, want %q", gotHost, WebAPIHost)
	}
	wantPath := "/api/v2/projects/owner%2Frepo/actions/plugins/all"
	if !strings.HasPrefix(gotEscapedPath, wantPath) {
		t.Fatalf("escaped path = %q, want prefix %q", gotEscapedPath, wantPath)
	}
	parsed, err := url.Parse(gotEscapedPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	q := parsed.Query()
	if q.Get("per_page") != "50" {
		t.Fatalf("per_page param = %q, want 50", q.Get("per_page"))
	}
	if q.Get("page") != "2" {
		t.Fatalf("page param = %q, want 2", q.Get("page"))
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	assertNoAccessTokenQuery(t, gotEscapedPath)
}

func TestListActionsPluginsSendsRefererHeader(t *testing.T) {
	var gotReferer string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotReferer = req.Header.Get("Referer")
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListActionsPlugins(client, "owner/repo", nil)
	if err != nil {
		t.Fatalf("ListActionsPlugins() error = %v", err)
	}
	if gotReferer != "https://gitcode.com/" {
		t.Fatalf("Referer = %q, want https://gitcode.com/", gotReferer)
	}
}

func TestViewActionsPluginSendsRefererHeader(t *testing.T) {
	var gotReferer string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotReferer = req.Header.Get("Referer")
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ViewActionsPlugin(client, "owner/repo", "checkout")
	if err != nil {
		t.Fatalf("ViewActionsPlugin() error = %v", err)
	}
	if gotReferer != "https://gitcode.com/" {
		t.Fatalf("Referer = %q, want https://gitcode.com/", gotReferer)
	}
}

func TestListActionsPluginsNoOptions(t *testing.T) {
	var gotEscapedPath string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotEscapedPath = req.URL.EscapedPath()
		return authTestResponse(http.StatusOK, `[]`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListActionsPlugins(client, "owner/repo", nil)
	if err != nil {
		t.Fatalf("ListActionsPlugins() error = %v", err)
	}

	wantPath := "/api/v2/projects/owner%2Frepo/actions/plugins/all"
	if gotEscapedPath != wantPath {
		t.Fatalf("escaped path = %q, want %q", gotEscapedPath, wantPath)
	}
}

func TestListActionsPluginsReturnsBody(t *testing.T) {
	body := `[{"name":"checkout","display_name":"Checkout","description":"checkout repo"}]`
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, body), nil
	})
	client.SetToken("test-token", "test")

	raw, err := ListActionsPlugins(client, "owner/repo", nil)
	if err != nil {
		t.Fatalf("ListActionsPlugins() error = %v", err)
	}
	if string(raw) != body {
		t.Fatalf("body = %q, want %q", string(raw), body)
	}
}

func TestListActionsPluginsError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ListActionsPlugins(client, "owner/repo", nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestViewActionsPluginBuildsV2Query(t *testing.T) {
	var gotHost string
	var gotEscapedPath string
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotHost = req.URL.Host
		gotEscapedPath = req.URL.EscapedPath()
		if req.URL.RawQuery != "" {
			gotEscapedPath += "?" + req.URL.RawQuery
		}
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := ViewActionsPlugin(client, "owner/repo", "checkout")
	if err != nil {
		t.Fatalf("ViewActionsPlugin() error = %v", err)
	}

	if gotHost != WebAPIHost {
		t.Fatalf("host = %q, want %q", gotHost, WebAPIHost)
	}
	parsed, err := url.Parse(gotEscapedPath)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	wantPathPrefix := "/api/v2/projects/owner%2Frepo/actions/plugins/detail"
	if !strings.HasPrefix(gotEscapedPath, wantPathPrefix) {
		t.Fatalf("escaped path = %q, want prefix %q", gotEscapedPath, wantPathPrefix)
	}
	if parsed.Query().Get("name") != "checkout" {
		t.Fatalf("name param = %q, want checkout", parsed.Query().Get("name"))
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	assertNoAccessTokenQuery(t, gotEscapedPath)
}

func TestViewActionsPluginReturnsBody(t *testing.T) {
	body := `{"name":"checkout","vision_content":[{"version":"v1","readme":"# Checkout"}]}`
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, body), nil
	})
	client.SetToken("test-token", "test")

	raw, err := ViewActionsPlugin(client, "owner/repo", "checkout")
	if err != nil {
		t.Fatalf("ViewActionsPlugin() error = %v", err)
	}
	if string(raw) != body {
		t.Fatalf("body = %q, want %q", string(raw), body)
	}
}

func TestParseActionsPluginsListPlainArray(t *testing.T) {
	raw := []byte(`[{"name":"checkout","display_name":"Checkout","description":"checkout repo","version":"v1.0"}]`)
	plugins, err := ParseActionsPluginsList(raw)
	if err != nil {
		t.Fatalf("ParseActionsPluginsList() error = %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("len = %d, want 1", len(plugins))
	}
	if plugins[0].Name != "checkout" {
		t.Fatalf("name = %q, want checkout", plugins[0].Name)
	}
	if plugins[0].Version != "v1.0" {
		t.Fatalf("version = %q, want v1.0", plugins[0].Version)
	}
}

func TestParseActionsPluginsListWrapper(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "plugins wrapper", raw: `{"plugins":[{"name":"a"}]}`, want: 1},
		{name: "list wrapper", raw: `{"list":[{"name":"b"}]}`, want: 1},
		{name: "data wrapper", raw: `{"data":[{"name":"c"}]}`, want: 1},
		{name: "content wrapper", raw: `{"content":[{"name":"d"}]}`, want: 1},
		{name: "empty array", raw: `[]`, want: 0},
		{name: "empty wrapper", raw: `{"plugins":[]}`, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins, err := ParseActionsPluginsList([]byte(tt.raw))
			if err != nil {
				t.Fatalf("ParseActionsPluginsList() error = %v", err)
			}
			if len(plugins) != tt.want {
				t.Fatalf("len = %d, want %d", len(plugins), tt.want)
			}
		})
	}
}

func TestParseActionsPluginsListUnrecognized(t *testing.T) {
	_, err := ParseActionsPluginsList([]byte(`{"unrelated":"field"}`))
	if err == nil {
		t.Fatal("expected an error for an unrecognized response object")
	}
}

func TestParseActionsPluginsPageContentMetadata(t *testing.T) {
	raw := []byte(`{"page_num":2,"page_size":50,"total":51,"page_count":2,"content":[{"name":"official_shell"}]}`)
	page, err := ParseActionsPluginsPage(raw)
	if err != nil {
		t.Fatalf("ParseActionsPluginsPage() error = %v", err)
	}
	if page.PageNum != 2 || page.PageSize != 50 || page.Total != 51 || page.PageCount != 2 {
		t.Fatalf("metadata = %+v, want page 2/50 total 51 count 2", page)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(page.Entries))
	}
}

func TestParseActionsPluginsListInvalid(t *testing.T) {
	_, err := ParseActionsPluginsList([]byte(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseActionsPluginsListRawPreservesFields(t *testing.T) {
	raw := []byte(`[{"name":"checkout","display_name":"Checkout","custom_field":"extra","version":"v1"}]`)
	entries, err := ParseActionsPluginsListRaw(raw)
	if err != nil {
		t.Fatalf("ParseActionsPluginsListRaw() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	var m map[string]interface{}
	if err := json.Unmarshal(entries[0], &m); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if m["custom_field"] != "extra" {
		t.Fatalf("custom_field = %v, want extra (full fields not preserved)", m["custom_field"])
	}
}

func TestCountPluginsEntries(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "plain array 3", raw: `[{"name":"a"},{"name":"b"},{"name":"c"}]`, want: 3},
		{name: "empty", raw: `[]`, want: 0},
		{name: "wrapper", raw: `{"plugins":[{"name":"a"}]}`, want: 1},
		{name: "content wrapper", raw: `{"content":[{"name":"a"}]}`, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountPluginsEntries([]byte(tt.raw))
			if got != tt.want {
				t.Fatalf("CountPluginsEntries() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRawRESTToHostWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "web-api host", host: WebAPIHost, wantErr: false},
		{name: "default host", host: DefaultHost, wantErr: false},
		{name: "arbitrary host", host: "evil.example.com", wantErr: true},
		{name: "empty host", host: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
				return authTestResponse(http.StatusOK, `{}`), nil
			})
			client.SetToken("test-token", "test")

			_, err := client.RawRESTToHost("GET", tt.host, "/api/v2/test", nil, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RawRESTToHost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRawRESTToHostInjectsToken(t *testing.T) {
	var gotAuth string
	var gotHost string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		gotHost = req.URL.Host
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	client.SetToken("secret-token", "test")

	_, err := client.RawRESTToHost("GET", WebAPIHost, "/api/v2/projects/test/actions/plugins/all", nil, nil)
	if err != nil {
		t.Fatalf("RawRESTToHost() error = %v", err)
	}

	if gotHost != WebAPIHost {
		t.Fatalf("host = %q, want %q", gotHost, WebAPIHost)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want Bearer secret-token", gotAuth)
	}
}

func TestRawRESTToHostError(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusNotFound, `{"message":"not found"}`), nil
	})
	client.SetToken("test-token", "test")

	_, err := client.RawRESTToHost("GET", WebAPIHost, "/api/v2/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention not found: %v", err)
	}
}

func TestRawRESTToHostCustomHeaders(t *testing.T) {
	var gotAccept string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotAccept = req.Header.Get("Accept")
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	client.SetToken("test-token", "test")

	headers := map[string]string{"X-Custom": "value"}
	_, err := client.RawRESTToHost("GET", WebAPIHost, "/api/v2/test", nil, headers)
	if err != nil {
		t.Fatalf("RawRESTToHost() error = %v", err)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q, want application/json", gotAccept)
	}
}

func TestRawRESTToHostNoToken(t *testing.T) {
	var gotAuth string
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		return authTestResponse(http.StatusOK, `{}`), nil
	})

	_, err := client.RawRESTToHost("GET", WebAPIHost, "/api/v2/test", nil, nil)
	if err != nil {
		t.Fatalf("RawRESTToHost() error = %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
}

func TestRawRESTToHostWithBody(t *testing.T) {
	client := newAuthTestClient(func(req *http.Request) (*http.Response, error) {
		return authTestResponse(http.StatusOK, `{}`), nil
	})
	client.SetToken("test-token", "test")

	body := io.NopCloser(strings.NewReader(`{"key":"value"}`))
	_, err := client.RawRESTToHost("POST", WebAPIHost, "/api/v2/test", body, nil)
	if err != nil {
		t.Fatalf("RawRESTToHost() error = %v", err)
	}
}
