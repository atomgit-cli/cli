package create

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestNewCmdCreate(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "create PR with title and head",
			args:    []string{"--title", "Feature", "--head", "feature-branch", "--repo", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "create draft PR",
			args:    []string{"--title", "WIP", "--head", "draft", "--draft", "--repo", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "create with base",
			args:    []string{"--title", "Feature", "--head", "feature", "--base", "develop", "--repo", "owner/repo"},
			wantErr: false,
		},
		{
			name:    "create cross-repo PR with fork",
			args:    []string{"--title", "Feature", "--head", "feature", "--fork", "myfork/repo", "--repo", "upstream/repo"},
			wantErr: false,
		},
		{
			name:    "create with json output",
			args:    []string{"--title", "Feature", "--head", "feature", "--repo", "owner/repo", "--json"},
			wantErr: false,
		},
		{
			name:    "create with body file",
			args:    []string{"--title", "Feature", "--head", "feature", "--repo", "owner/repo", "--body-file", "body.md"},
			wantErr: false,
		},
		{
			name:    "missing title",
			args:    []string{"--head", "feature", "--repo", "owner/repo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			// For validation tests, don't provide runF so actual validation runs
			var cmd *cobra.Command
			if tt.name == "missing title" {
				cmd = NewCmdCreate(f, nil)
			} else {
				cmd = NewCmdCreate(f, func(opts *CreateOptions) error {
					return nil
				})
			}
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateRunJSONWritesCreatedPR(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		Head:       "feature-branch",
		Base:       "main",
		JSON:       true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			return &api.PullRequest{Number: 7, Title: createOpts.Title, Body: createOpts.Body, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).String()
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON output did not parse: %v", err)
	}
	if got["number"] != float64(7) || got["html_url"] != "https://gitcode.com/owner/repo/merge_requests/7" {
		t.Fatalf("JSON output = %#v", got)
	}
	if strings.Contains(out, "Created PR") {
		t.Fatalf("JSON output contains text banner: %q", out)
	}
	if errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String(); errOut != "" {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
}

func TestCreateRunJSONWarnsWhenBodyMissingFromRemoteResponse(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		Head:       "feature-branch",
		Base:       "main",
		JSON:       true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			return &api.PullRequest{Number: 7, Title: createOpts.Title, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).String()
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON output did not parse: %v", err)
	}
	if got["body"] != "" {
		t.Fatalf("JSON body = %#v, want empty remote body", got["body"])
	}
	errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String()
	for _, want := range []string{
		"warning:",
		"PR 描述未能从远端返回",
		"gitcode pr view 7 -R owner/repo --json",
	} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("stderr %q does not contain %q", errOut, want)
		}
	}
}

func TestCreateRunReadsBodyFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "body.md")
	if err := os.WriteFile(bodyPath, []byte("file body\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		BodyFile:   bodyPath,
		Head:       "feature-branch",
		Base:       "main",
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Body != "file body" {
		t.Fatalf("CreatePR Body = %q", createdOpts.Body)
	}
}

func TestCreateRunReadsBodyFileStripsUTF8BOM(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "body.md")
	if err := os.WriteFile(bodyPath, []byte{0xef, 0xbb, 0xbf, 'f', 'i', 'l', 'e', ' ', 'b', 'o', 'd', 'y', '\n'}, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		BodyFile:   bodyPath,
		Head:       "feature-branch",
		Base:       "main",
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Body != "file body" {
		t.Fatalf("CreatePR Body = %q", createdOpts.Body)
	}
}

func TestCreateRunReadsBodyFileFromStdin(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	f.IOStreams.In = strings.NewReader("stdin body\n")

	var createdOpts *api.CreatePROptions
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		BodyFile:   "-",
		Head:       "feature-branch",
		Base:       "main",
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Body != "stdin body" {
		t.Fatalf("CreatePR Body = %q", createdOpts.Body)
	}
}

func TestCreateRunBodyAndBodyFileMutualExclusion(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	called := false
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		BodyFile:   "body.md",
		Head:       "feature-branch",
		Base:       "main",
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			called = true
			return nil, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	err := createRun(opts)
	if err == nil {
		t.Fatal("createRun() error = nil")
	}
	if !strings.Contains(err.Error(), "cannot use both --body and --body-file") {
		t.Fatalf("createRun() error = %v", err)
	}
	if called {
		t.Fatal("CreatePR should not be called when body inputs conflict")
	}
}

func TestCreateRunJSONWithWebReturnsUsageBeforeCreate(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	called := false
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Head:       "feature-branch",
		Base:       "main",
		JSON:       true,
		Web:        true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			called = true
			return nil, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	err := createRun(opts)
	if err == nil {
		t.Fatal("createRun() error = nil")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
	if called {
		t.Fatal("CreatePR should not be called when --json and --web conflict")
	}
}

func TestCreateRunJSONWithWebReturnsUsageBeforeAuth(t *testing.T) {
	f := cmdutil.TestFactory()
	called := false
	opts := &CreateOptions{
		IO: f.IOStreams,
		HttpClient: func() (*http.Client, error) {
			t.Fatal("HttpClient should not be called when --json and --web conflict")
			return nil, nil
		},
		Repository: "owner/repo",
		Title:      "title",
		Head:       "feature-branch",
		Base:       "main",
		JSON:       true,
		Web:        true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			called = true
			return nil, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	err := createRun(opts)
	if err == nil {
		t.Fatal("createRun() error = nil")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
	if called {
		t.Fatal("CreatePR should not be called when --json and --web conflict")
	}
}

func TestFillFromLastCommit(t *testing.T) {
	opts := &CreateOptions{
		ExecGitCommand: func(name string, args ...string) (string, error) {
			return "feat: add api auth cleanup\n\nBody line 1\nBody line 2\n", nil
		},
	}

	if err := fillFromLastCommit(opts); err != nil {
		t.Fatalf("fillFromLastCommit() error = %v", err)
	}

	if opts.Title != "feat: add api auth cleanup" {
		t.Fatalf("Title = %q", opts.Title)
	}
	if opts.Body != "Body line 1\nBody line 2" {
		t.Fatalf("Body = %q", opts.Body)
	}
}

func TestCreateRunFillAndWeb(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions
	var openedURL string

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Base:       "main",
		Fill:       true,
		Web:        true,
		Branch: func() (string, error) {
			return "feature-branch", nil
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			commandLine := name + " " + strings.Join(args, " ")
			switch commandLine {
			case "git log -1 --pretty=%B":
				return "feat: add fill behavior\n\ncommit body", nil
			default:
				return "", nil
			}
		},
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{
				Number:  12,
				HTMLURL: "https://gitcode.com/owner/repo/merge_requests/12",
			}, nil
		},
		OpenBrowser: func(url string) error {
			openedURL = url
			return nil
		},
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Title != "feat: add fill behavior" {
		t.Fatalf("CreatePR Title = %q", createdOpts.Title)
	}
	if createdOpts.Body != "commit body" {
		t.Fatalf("CreatePR Body = %q", createdOpts.Body)
	}
	if createdOpts.Head != "feature-branch" {
		t.Fatalf("CreatePR Head = %q", createdOpts.Head)
	}
	if openedURL != "https://gitcode.com/owner/repo/merge_requests/12" {
		t.Fatalf("opened URL = %q", openedURL)
	}
}

func TestCreateRunFillPreservesExplicitTitleAndBody(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "explicit title",
		Body:       "explicit body",
		Head:       "feature-branch",
		Fill:       true,
		Branch: func() (string, error) {
			return "ignored", nil
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			return "commit title\n\ncommit body", nil
		},
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{
				Number:  1,
				HTMLURL: "https://gitcode.com/owner/repo/merge_requests/1",
			}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}

	if createdOpts.Title != "explicit title" || createdOpts.Body != "explicit body" {
		t.Fatalf("explicit values were overwritten: %+v", createdOpts)
	}
}

func TestCreateRunFillPreservesBodyFile(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "body.md")
	if err := os.WriteFile(bodyPath, []byte("file body\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "explicit title",
		BodyFile:   bodyPath,
		Head:       "feature-branch",
		Fill:       true,
		Branch: func() (string, error) {
			return "ignored", nil
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			return "commit title\n\ncommit body", nil
		},
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{
				Number:  1,
				HTMLURL: "https://gitcode.com/owner/repo/merge_requests/1",
			}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts.Title != "explicit title" || createdOpts.Body != "file body" {
		t.Fatalf("explicit values were overwritten: %+v", createdOpts)
	}
}

func TestCreateRunUsesFactoryBranch(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		Base:       "main",
		Branch: func() (string, error) {
			return "feature/from-factory", nil
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			t.Fatalf("ExecGitCommand should not be used for branch detection")
			return "", nil
		},
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Head != "feature/from-factory" {
		t.Fatalf("CreatePR Head = %q", createdOpts.Head)
	}
}

func TestResolveHead(t *testing.T) {
	tests := []struct {
		name        string
		head        string
		fork        string
		want        string
		wantWarning string
		wantErr     bool
	}{
		{
			name: "no fork leaves head unchanged",
			head: "feature",
			fork: "",
			want: "feature",
		},
		{
			name: "fork builds owner/repo:branch from owner/repo",
			head: "feature/issue-495",
			fork: "myfork/repo",
			want: "myfork/repo:feature/issue-495",
		},
		{
			name:    "fork without repo segment is rejected",
			head:    "feature",
			fork:    "myfork",
			wantErr: true,
		},
		{
			name: "head with matching owner is preserved without warning",
			head: "myfork:feature",
			fork: "myfork/repo",
			want: "myfork:feature",
		},
		{
			name:        "head owner overriding mismatched fork owner warns",
			head:        "other:feature",
			fork:        "myfork/repo",
			want:        "other:feature",
			wantWarning: `--head owner "other" overrides --fork owner "myfork"`,
		},
		{
			name: "explicit owner/repo:branch head is preserved without warning",
			head: "myfork/repo:feature",
			fork: "myfork/repo",
			want: "myfork/repo:feature",
		},
		{
			name:    "fork missing owner is rejected",
			head:    "feature",
			fork:    "/repo",
			wantErr: true,
		},
		{
			name:    "fork missing repo is rejected",
			head:    "feature",
			fork:    "myfork/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warning, err := resolveHead(tt.head, tt.fork)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveHead() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Fatalf("resolveHead() = %q, want %q", got, tt.want)
			}
			if warning != tt.wantWarning {
				t.Fatalf("resolveHead() warning = %q, want %q", warning, tt.wantWarning)
			}
		})
	}
}

func TestCreateRunForkNormalizesHeadToOwnerRepoBranch(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "upstream/repo",
		Title:      "title",
		Body:       "body",
		Head:       "feature/issue-259",
		Base:       "main",
		Fork:       "myfork/repo",
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, HTMLURL: "https://gitcode.com/upstream/repo/merge_requests/7"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Head != "myfork/repo:feature/issue-259" {
		t.Fatalf("CreatePR Head = %q, want %q", createdOpts.Head, "myfork/repo:feature/issue-259")
	}
}

func TestCreateRunForkPreservesExplicitOwnerHead(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "upstream/repo",
		Title:      "title",
		Body:       "body",
		Head:       "explicitowner:feature",
		Base:       "main",
		Fork:       "myfork/repo",
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 8, HTMLURL: "https://gitcode.com/upstream/repo/merge_requests/8"}, nil
		},
		OpenBrowser: func(url string) error { return nil },
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if createdOpts.Head != "explicitowner:feature" {
		t.Fatalf("CreatePR Head = %q, want %q", createdOpts.Head, "explicitowner:feature")
	}
	errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String()
	if !strings.Contains(errOut, `--head owner "explicitowner" overrides --fork owner "myfork"`) {
		t.Fatalf("expected owner-conflict warning on stderr, got %q", errOut)
	}
}

func TestCreateRunBranchError(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		Branch: func() (string, error) {
			return "", fmt.Errorf("not in a git repository")
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			t.Fatalf("ExecGitCommand should not be used for branch detection")
			return "", nil
		},
		CreatePR:    api.CreatePullRequest,
		OpenBrowser: func(url string) error { return nil },
	}

	err := createRun(opts)
	if err == nil {
		t.Fatalf("createRun() error = nil")
	}
	if !strings.Contains(err.Error(), "could not determine current branch") {
		t.Fatalf("createRun() error = %v", err)
	}
}

func TestCreateRunPassesLabelsToAPI(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var receivedLabels []string

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		Head:       "feature-branch",
		Base:       "main",
		Labels:     []string{"bug", "enhancement"},
		JSON:       true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			receivedLabels = createOpts.Labels
			return &api.PullRequest{Number: 7, Title: createOpts.Title, Body: createOpts.Body, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
	}

	err := createRun(opts)
	if err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if len(receivedLabels) != 2 || receivedLabels[0] != "bug" || receivedLabels[1] != "enhancement" {
		t.Fatalf("expected labels [bug, enhancement], got %v", receivedLabels)
	}
}

func TestCreateRunNoLabels(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var receivedLabels []string

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Body:       "body",
		Head:       "feature-branch",
		Base:       "main",
		JSON:       true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			receivedLabels = createOpts.Labels
			return &api.PullRequest{Number: 7, Title: createOpts.Title, Body: createOpts.Body, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
	}

	err := createRun(opts)
	if err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if len(receivedLabels) != 0 {
		t.Fatalf("expected no labels, got %v", receivedLabels)
	}
}

func TestNewCmdCreate_LabelsFlag(t *testing.T) {
	cmd := NewCmdCreate(cmdutil.TestFactory(), func(opts *CreateOptions) error {
		if len(opts.Labels) != 2 || opts.Labels[0] != "bug" || opts.Labels[1] != "feature" {
			t.Fatalf("expected labels [bug, feature], got %v", opts.Labels)
		}
		return nil
	})
	cmd.SetArgs([]string{"--title", "Feature", "--head", "feature-branch", "--repo", "owner/repo", "--labels", "bug,feature"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestNewCmdCreate_IssueFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantIssues   []string
		wantClose    bool
		wantCloseSet bool
	}{
		{
			name:       "comma separated issue numbers",
			args:       []string{"--title", "Feature", "--issue", "12,13"},
			wantIssues: []string{"12", "13"},
		},
		{
			name:       "hash prefixed issue number",
			args:       []string{"--title", "Feature", "--issue", "#12"},
			wantIssues: []string{"#12"},
		},
		{
			name:       "repeated issue flags accumulate",
			args:       []string{"--title", "Feature", "-i", "12", "--issue", "13"},
			wantIssues: []string{"12", "13"},
		},
		{
			name: "close policy stays unset without the flag",
			args: []string{"--title", "Feature"},
		},
		{
			name:         "bare close related issue means true",
			args:         []string{"--title", "Feature", "--close-related-issue"},
			wantClose:    true,
			wantCloseSet: true,
		},
		{
			name:         "explicit false close related issue",
			args:         []string{"--title", "Feature", "--close-related-issue=false"},
			wantClose:    false,
			wantCloseSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			var got *CreateOptions
			cmd := NewCmdCreate(f, func(opts *CreateOptions) error {
				got = opts
				return nil
			})
			cmd.SetArgs(append(tt.args, "--repo", "owner/repo"))

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got == nil {
				t.Fatalf("runF was not called")
			}
			if strings.Join(got.Issues, ",") != strings.Join(tt.wantIssues, ",") {
				t.Fatalf("Issues = %v, want %v", got.Issues, tt.wantIssues)
			}
			if got.CloseRelatedIssue != tt.wantClose {
				t.Fatalf("CloseRelatedIssue = %v, want %v", got.CloseRelatedIssue, tt.wantClose)
			}
			if got.CloseRelatedIssueSet != tt.wantCloseSet {
				t.Fatalf("CloseRelatedIssueSet = %v, want %v", got.CloseRelatedIssueSet, tt.wantCloseSet)
			}
		})
	}
}

func TestCreateRunPassesLinkedIssuesAndClosePolicy(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions

	opts := &CreateOptions{
		IO:                   f.IOStreams,
		HttpClient:           f.HttpClient,
		Repository:           "owner/repo",
		Title:                "title",
		Head:                 "feature-branch",
		Base:                 "main",
		Issues:               []string{"12", "#13", "12"},
		CloseRelatedIssue:    true,
		CloseRelatedIssueSet: true,
		JSON:                 true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, Title: createOpts.Title, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if fmt.Sprint(createdOpts.IssueNumbers) != "[12 13]" {
		t.Fatalf("IssueNumbers = %v, want [12 13]", createdOpts.IssueNumbers)
	}
	if createdOpts.CloseRelatedIssue == nil || !*createdOpts.CloseRelatedIssue {
		t.Fatalf("CloseRelatedIssue = %v, want true", createdOpts.CloseRelatedIssue)
	}
}

func TestCreateRunLeavesIssueLinkageUnset(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var createdOpts *api.CreatePROptions

	opts := &CreateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "title",
		Head:       "feature-branch",
		Base:       "main",
		JSON:       true,
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			createdOpts = createOpts
			return &api.PullRequest{Number: 7, Title: createOpts.Title, HTMLURL: "https://gitcode.com/owner/repo/merge_requests/7"}, nil
		},
	}

	if err := createRun(opts); err != nil {
		t.Fatalf("createRun() error = %v", err)
	}
	if createdOpts == nil {
		t.Fatalf("CreatePR() was not called")
	}
	if len(createdOpts.IssueNumbers) != 0 {
		t.Fatalf("IssueNumbers = %v, want none", createdOpts.IssueNumbers)
	}
	if createdOpts.CloseRelatedIssue != nil {
		t.Fatalf("CloseRelatedIssue = %v, want nil so the repository setting applies", *createdOpts.CloseRelatedIssue)
	}
}

func TestCreateRunRejectsInvalidIssueValues(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	tests := []struct {
		name   string
		issues []string
	}{
		{name: "non numeric", issues: []string{"abc"}},
		{name: "zero", issues: []string{"0"}},
		{name: "negative", issues: []string{"-3"}},
		{name: "decimal", issues: []string{"1.5"}},
		{name: "double hash", issues: []string{"##12"}},
		{name: "empty element", issues: []string{"12", ""}},
		{name: "empty list", issues: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmdutil.TestFactory()
			created := false
			opts := &CreateOptions{
				IO:         f.IOStreams,
				HttpClient: f.HttpClient,
				Repository: "owner/repo",
				Title:      "title",
				Head:       "feature-branch",
				Base:       "main",
				Issues:     tt.issues,
				CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
					created = true
					return &api.PullRequest{Number: 7}, nil
				},
			}

			err := createRun(opts)
			if err == nil {
				t.Fatalf("createRun() error = nil, want usage error")
			}
			if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
				t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
			}
			if created {
				t.Fatalf("CreatePR() was called for an invalid --issue value")
			}
		})
	}
}

func TestCreateRunBrowserFailureNonTTY(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, _, errBuf := iostreams.Test()
	f := cmdutil.TestFactory()

	opts := &CreateOptions{
		IO:         ios,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "test PR",
		Body:       "test body",
		Head:       "feature-branch",
		Web:        true,
		Branch: func() (string, error) {
			return "feature-branch", nil
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			return "commit title\n\ncommit body", nil
		},
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			return &api.PullRequest{
				Number:  1,
				HTMLURL: "https://gitcode.com/owner/repo/merge_requests/1",
			}, nil
		},
		OpenBrowser: func(url string) error {
			return fmt.Errorf("browser not available")
		},
	}

	err := createRun(opts)
	if err != nil {
		t.Fatalf("createRun() error = %v, want nil in non-TTY mode", err)
	}
	if !strings.Contains(errBuf.String(), "Failed to open browser") {
		t.Fatalf("expected warning on stderr, got %q", errBuf.String())
	}
}

func TestCreateRunBrowserFailureTTY(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	ios, _, _, _ := iostreams.TestTTY()
	f := cmdutil.TestFactory()

	opts := &CreateOptions{
		IO:         ios,
		HttpClient: f.HttpClient,
		Repository: "owner/repo",
		Title:      "test PR",
		Body:       "test body",
		Head:       "feature-branch",
		Web:        true,
		Branch: func() (string, error) {
			return "feature-branch", nil
		},
		ExecGitCommand: func(name string, args ...string) (string, error) {
			return "commit title\n\ncommit body", nil
		},
		CreatePR: func(client *api.Client, owner, repo string, createOpts *api.CreatePROptions) (*api.PullRequest, error) {
			return &api.PullRequest{
				Number:  1,
				HTMLURL: "https://gitcode.com/owner/repo/merge_requests/1",
			}, nil
		},
		OpenBrowser: func(url string) error {
			return fmt.Errorf("browser not available")
		},
	}

	err := createRun(opts)
	if err == nil {
		t.Fatalf("createRun() error = nil, want error in TTY mode")
	}
	if !strings.Contains(err.Error(), "failed to open browser") {
		t.Fatalf("expected browser error, got %v", err)
	}
}
