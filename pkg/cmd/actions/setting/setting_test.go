package setting

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestSettingRunSingleRepositoryReadsBeforeConfirmingAndPreservesFields(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	jwt := futureWebJWT(t)
	ioStreams, in, out, errOut := iostreams.TestTTY()
	_, _ = in.WriteString(jwt + "\n")
	_, _ = in.WriteString("y\n")

	var putBody map[string]any
	var requests []string
	var v5Auth, settingAuth []string
	clientFactory := func() (*http.Client, error) {
		return testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.Method+" "+r.URL.Path)
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v5/repos/owner/repo":
				v5Auth = append(v5Auth, r.Header.Get("Authorization"))
				writeJSON(w, `{"id":"project-1","full_name":"owner/repo"}`)
			case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
				settingAuth = append(settingAuth, r.Header.Get("Authorization"))
				writeJSON(w, `{"data":{"action_enabled":false,"block_all_new_pipelines":true,"block_cross_repo_pr_triggers":false}}`)
			case r.Method == http.MethodPut && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
				settingAuth = append(settingAuth, r.Header.Get("Authorization"))
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read PUT body: %v", err)
					return
				}
				if err := json.Unmarshal(body, &putBody); err != nil {
					t.Errorf("decode PUT body: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(w, r)
			}
		})), nil
	}

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
		WithToken:  true,
	})
	if err != nil {
		t.Fatalf("settingRun() error = %v", err)
	}
	if strings.Join(requests, ",") != "GET /api/v5/repos/owner/repo,GET /api/v2/projects/project-1/actions/setting,PUT /api/v2/projects/project-1/actions/setting" {
		t.Fatalf("requests = %v, want repository GET, setting GET, then PUT", requests)
	}
	if len(v5Auth) != 1 || v5Auth[0] != "Bearer test-token" {
		t.Fatalf("v5 authorization = %v, want the classic token", v5Auth)
	}
	if len(settingAuth) != 2 || settingAuth[0] != "Bearer "+jwt || settingAuth[1] != "Bearer "+jwt {
		t.Fatalf("setting authorization = %v, want the web JWT on GET and PUT", settingAuth)
	}
	if putBody["action_enabled"] != true || putBody["block_all_new_pipelines"] != true || putBody["block_cross_repo_pr_triggers"] != false {
		t.Fatalf("PUT body = %#v, want only action_enabled changed", putBody)
	}
	if _, ok := putBody["project_id"]; ok {
		t.Fatalf("PUT body unexpectedly contains project_id: %#v", putBody)
	}
	if !strings.Contains(errOut.String(), "! Warning: This command calls a GitCode Web API endpoint") || !strings.Contains(errOut.String(), "! Type y to confirm:") {
		t.Fatalf("confirmation prompt = %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Actions enabled for 1/1 repositories") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestSettingRunPromptsForWebJWTInInteractiveMode(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	jwt := futureWebJWT(t)
	ioStreams, in, out, errOut := iostreams.TestTTY()
	_, _ = in.WriteString("y\n")
	passwordRead := false

	var settingAuth []string
	clientFactory := settingTestClient(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v5/repos/owner/repo":
			writeJSON(w, `{"id":"project-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
			settingAuth = append(settingAuth, r.Header.Get("Authorization"))
			writeJSON(w, `{"action_enabled":false}`)
		case r.Method == http.MethodPut:
			settingAuth = append(settingAuth, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
		readPassword: func(fd int) ([]byte, error) {
			passwordRead = true
			if fd != -1 {
				t.Fatalf("test password reader fd = %d, want test sentinel", fd)
			}
			return []byte(jwt), nil
		},
	})
	if err != nil {
		t.Fatalf("settingRun() error = %v", err)
	}
	if !strings.Contains(errOut.String(), "top-right corner") || !strings.Contains(errOut.String(), "Console") || !strings.Contains(errOut.String(), "copy(localStorage.getItem('access_token'))") || !strings.Contains(errOut.String(), "Web session JWT (input hidden; press Enter when complete):") {
		t.Fatalf("prompt output = %q, want JWT guidance and paste prompt", errOut.String())
	}
	if !passwordRead {
		t.Fatal("interactive JWT input did not use the hidden password reader")
	}
	if len(settingAuth) != 2 || settingAuth[0] != "Bearer "+jwt {
		t.Fatalf("setting authorization = %v, want the pasted web JWT", settingAuth)
	}
	if !strings.Contains(out.String(), "Actions enabled for 1/1 repositories") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestSettingRunOrganizationUpdatesOnlyChangedRepositories(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, in, out, errOut := iostreams.TestTTY()
	_, _ = in.WriteString(futureWebJWT(t) + "\n")
	_, _ = in.WriteString("y\n")

	var putIDs []string
	clientFactory := func() (*http.Client, error) {
		return testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v5/orgs/acme/repos":
				writeJSON(w, `[{"id":101,"name":"first","full_name":"acme/first"},{"id":102,"name":"second","full_name":"acme/second"}]`)
			case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/101/actions/setting":
				writeJSON(w, `{"action_enabled":true,"block_all_new_pipelines":false}`)
			case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/102/actions/setting":
				writeJSON(w, `{"action_enabled":false,"block_all_new_pipelines":true}`)
			case r.Method == http.MethodPut:
				putIDs = append(putIDs, r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(w, r)
			}
		})), nil
	}

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "acme",
		Action:     "enable",
		WithToken:  true,
	})
	if err != nil {
		t.Fatalf("settingRun() error = %v", err)
	}
	if len(putIDs) != 1 || putIDs[0] != "/api/v2/projects/102/actions/setting" {
		t.Fatalf("PUT paths = %v, want only project 102", putIDs)
	}
	if !strings.Contains(out.String(), "Actions enabled for 1/2 repositories") {
		t.Fatalf("output = %q", out.String())
	}
	preview := errOut.String()
	if !strings.Contains(preview, "Actions will be enabled for 1 of 2 repositories:") || !strings.Contains(preview, "acme/first: skipped (Actions already enabled)") {
		t.Fatalf("preview = %q, want changed count and skipped reason", preview)
	}
}

func TestSettingRunDoesNotWriteWhenConfirmationIsRejected(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, in, _, _ := iostreams.TestTTY()
	_, _ = in.WriteString(futureWebJWT(t) + "\n")
	_, _ = in.WriteString("n\n")
	putCalled := false
	clientFactory := settingTestClient(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v5/repos/owner/repo":
			writeJSON(w, `{"id":"project-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
			writeJSON(w, `{"action_enabled":false}`)
		case r.Method == http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
		WithToken:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "confirmation did not match") {
		t.Fatalf("settingRun() error = %v, want rejected confirmation", err)
	}
	if putCalled {
		t.Fatal("PUT called after rejected confirmation")
	}
}

func TestSettingRunSkipsWriteButStillConfirmsWhenAlreadyDesired(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, in, out, errOut := iostreams.TestTTY()
	_, _ = in.WriteString(futureWebJWT(t) + "\n")
	_, _ = in.WriteString("y\n")
	putCalled := false
	clientFactory := settingTestClient(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v5/repos/owner/repo":
			writeJSON(w, `{"id":"project-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
			writeJSON(w, `{"action_enabled":true}`)
		case r.Method == http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
		WithToken:  true,
	})
	if err != nil {
		t.Fatalf("settingRun() error = %v", err)
	}
	if putCalled {
		t.Fatal("PUT called even though Actions was already enabled")
	}
	if !strings.Contains(errOut.String(), "! Warning: This command calls a GitCode Web API endpoint") || !strings.Contains(errOut.String(), "owner/repo: skipped (Actions already enabled)") || !strings.Contains(errOut.String(), "! Type y to confirm:") {
		t.Fatalf("confirmation prompt = %q", errOut.String())
	}
	if !strings.Contains(out.String(), "Actions enabled for 0/1 repositories") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestSettingRunRequiresYesInNonInteractiveMode(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, in, _, _ := iostreams.Test()
	_, _ = in.WriteString(futureWebJWT(t) + "\n")
	clientFactory := settingTestClient(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v5/repos/owner/repo":
			writeJSON(w, `{"id":"project-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
			writeJSON(w, `{"action_enabled":false}`)
		default:
			http.NotFound(w, r)
		}
	})

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
		WithToken:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("settingRun() error = %v, want --yes guidance", err)
	}
}

func TestSettingRunFailsFastInNonInteractiveModeWithoutJWT(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	ioStreams, _, _, _ := iostreams.Test()
	var requests []string
	clientFactory := settingTestClient(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api/v5/repos/owner/repo" {
			writeJSON(w, `{"id":"project-1"}`)
			return
		}
		http.NotFound(w, r)
	})

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
	})
	if err == nil || !strings.Contains(err.Error(), "--with-token") {
		t.Fatalf("settingRun() error = %v, want --with-token guidance", err)
	}
	if cmdutil.ExitCode(err) != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d (ExitUsage)", cmdutil.ExitCode(err), cmdutil.ExitUsage)
	}
	for _, request := range requests {
		if strings.Contains(request, "/api/v2/") {
			t.Fatalf("setting API called without a JWT: %v", requests)
		}
	}
}

func TestSettingRunWithTokenReadsJWTFromStdin(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	jwt := futureWebJWT(t)
	ioStreams, in, out, _ := iostreams.Test()
	_, _ = in.WriteString(jwt + "\n")
	var putAuth string
	clientFactory := settingTestClient(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v5/repos/owner/repo":
			writeJSON(w, `{"id":"project-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/project-1/actions/setting":
			writeJSON(w, `{"action_enabled":false}`)
		case r.Method == http.MethodPut:
			putAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	err := settingRun(&SettingOptions{
		IO:         ioStreams,
		HttpClient: clientFactory,
		Repository: "owner/repo",
		Action:     "enable",
		WithToken:  true,
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("settingRun() error = %v", err)
	}
	if putAuth != "Bearer "+jwt {
		t.Fatalf("PUT authorization = %q, want the piped web JWT", putAuth)
	}
	if !strings.Contains(out.String(), "Actions enabled for 1/1 repositories") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestValidateWebJWT(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	past := time.Now().Add(-time.Hour).Unix()

	valid := futureWebJWT(t)
	if got, err := validateWebJWT("  " + valid + "  \n"); err != nil || got != valid {
		t.Fatalf("validateWebJWT() = %q, %v; want trimmed token without error", got, err)
	}
	if _, err := validateWebJWT(testWebJWT(t, future)); err != nil {
		t.Fatalf("validateWebJWT(future exp) error = %v, want nil", err)
	}

	if _, err := validateWebJWT(""); err == nil || !strings.Contains(err.Error(), "no web session JWT provided") {
		t.Fatalf("validateWebJWT(\"\") error = %v, want empty-input usage error", err)
	}
	if _, err := validateWebJWT("not-a-jwt"); err == nil || !strings.Contains(err.Error(), "not a valid GitCode web-session JWT") {
		t.Fatalf("validateWebJWT(bad shape) error = %v, want shape error", err)
	}
	if _, err := validateWebJWT(testWebJWT(t, past)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("validateWebJWT(expired) error = %v, want expiry error", err)
	}
	brokenPayload := "header." + base64.RawURLEncoding.EncodeToString([]byte("not-json")) + ".sig"
	if _, err := validateWebJWT(brokenPayload); err == nil || !strings.Contains(err.Error(), "failed to decode the web-session JWT payload") {
		t.Fatalf("validateWebJWT(broken payload) error = %v, want decode error", err)
	}
}

func TestNewCmdSettingParsesWithTokenFlag(t *testing.T) {
	var opts *SettingOptions
	cmd := NewCmdSetting(cmdutil.TestFactory(), func(received *SettingOptions) error {
		opts = received
		return nil
	})
	cmd.SetArgs([]string{"enable", "-R", "acme", "--with-token"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if opts == nil || !opts.WithToken {
		t.Fatalf("with-token flag = %#v, want opts.WithToken true", opts)
	}
}

func TestNewCmdSettingRequiresOneTarget(t *testing.T) {
	cmd := NewCmdSetting(cmdutil.TestFactory(), func(opts *SettingOptions) error {
		return nil
	})
	cmd.SetArgs([]string{"enable"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "-R/--repo") {
		t.Fatalf("Execute() error = %v, want target validation", err)
	}
}

func TestNewCmdSettingAcceptsOrganizationWithRepositoryFlag(t *testing.T) {
	var gotTarget string
	cmd := NewCmdSetting(cmdutil.TestFactory(), func(opts *SettingOptions) error {
		gotTarget = opts.Repository
		return nil
	})
	cmd.SetArgs([]string{"enable", "-R", "acme"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotTarget != "acme" {
		t.Fatalf("repository target = %q, want organization target", gotTarget)
	}
}

func TestListAllOrganizationRepositoriesPaginates(t *testing.T) {
	firstPage := make([]api.Repository, organizationRepoPageSize)
	for i := range firstPage {
		firstPage[i] = api.Repository{ID: i + 1, Name: "repo"}
	}
	secondPage := []api.Repository{{ID: 101, Name: "last"}}
	var paths []string
	client := api.NewClientFromHTTP(testutil.NewTestHTTPClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if r.URL.Query().Get("page") == "1" {
			writeJSONValue(w, firstPage)
			return
		}
		writeJSONValue(w, secondPage)
	})))

	repositories, err := listAllOrganizationRepositories(client, "acme")
	if err != nil {
		t.Fatalf("listAllOrganizationRepositories() error = %v", err)
	}
	if len(repositories) != 101 {
		t.Fatalf("repository count = %d, want 101", len(repositories))
	}
	if len(paths) != 2 || !strings.Contains(paths[0], "page=1") || !strings.Contains(paths[1], "page=2") {
		t.Fatalf("paths = %v, want pages 1 and 2", paths)
	}
}

func futureWebJWT(t *testing.T) string {
	t.Helper()
	return testWebJWT(t, time.Now().Add(time.Hour).Unix())
}

// testWebJWT builds a syntactically valid but fake JWT. The payload only
// carries an exp claim; no real credential ever appears in tests.
func testWebJWT(t *testing.T, exp int64) string {
	t.Helper()
	segment := func(value any) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal JWT segment: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(encoded)
	}
	header := segment(map[string]string{"alg": "HS512", "typ": "JWT"})
	payload := segment(map[string]any{"exp": exp, "sub": "tester"})
	return header + "." + payload + ".signature"
}

func settingTestClient(handler http.HandlerFunc) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		return testutil.NewTestHTTPClient(handler), nil
	}
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func writeJSONValue(w http.ResponseWriter, value any) {
	body, _ := json.Marshal(value)
	writeJSON(w, string(body))
}
