package fork

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/git"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
)

func TestNewCmdFork(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "fork with repo",
			args:    []string{"owner/repo"},
			wantErr: false,
		},
		{
			name:    "fork with clone flag",
			args:    []string{"owner/repo", "--clone"},
			wantErr: false,
		},
		{
			name:    "fork with json output",
			args:    []string{"owner/repo", "--json"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdFork(f, func(opts *ForkOptions) error {
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

func TestForkRunJSONWritesForkedRepository(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()

	opts := &ForkOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return fakeConfig{}, nil
		},
		ParseRepo: git.ParseRepo,
		ForkRepo: func(client *api.Client, owner, name string) (*api.Repository, error) {
			return &api.Repository{
				FullName: "fork-owner/fork-repo",
				HTMLURL:  "https://gitcode.com/fork-owner/fork-repo",
				CloneURL: "https://gitcode.com/fork-owner/fork-repo.git",
			}, nil
		},
		CloneRepo: func(repo *git.Repo, dir string, protocol string, depth int) error {
			t.Fatalf("CloneRepo() should not be called when --json is used without --clone")
			return nil
		},
		Repository: "infra-test/gctest1",
		JSON:       true,
	}

	if err := forkRun(opts); err != nil {
		t.Fatalf("forkRun() error = %v", err)
	}

	var got map[string]interface{}
	out := f.IOStreams.Out.(*bytes.Buffer).Bytes()
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, string(out))
	}
	if got["full_name"] != "fork-owner/fork-repo" || got["web_url"] != "https://gitcode.com/fork-owner/fork-repo" {
		t.Fatalf("JSON output = %#v", got)
	}
	if strings.Contains(string(out), "Forked repository") {
		t.Fatalf("JSON output contains text banner: %q", string(out))
	}
}

func TestForkRunRejectsJSONWithCloneBeforeFork(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &ForkOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return fakeConfig{}, nil
		},
		ParseRepo: git.ParseRepo,
		ForkRepo: func(client *api.Client, owner, name string) (*api.Repository, error) {
			t.Fatalf("ForkRepo() should not be called when --json and --clone conflict")
			return nil, nil
		},
		CloneRepo: func(repo *git.Repo, dir string, protocol string, depth int) error {
			t.Fatalf("CloneRepo() should not be called when --json and --clone conflict")
			return nil
		},
		Repository: "infra-test/gctest1",
		JSON:       true,
		Clone:      true,
	}

	err := forkRun(opts)
	if err == nil {
		t.Fatal("forkRun() error = nil")
	}
	if got := cmdutil.ExitCode(err); got != cmdutil.ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", got, cmdutil.ExitUsage)
	}
	if out := f.IOStreams.Out.(*bytes.Buffer).String(); out != "" {
		t.Fatalf("stdout = %q, want empty output", out)
	}
}

func TestForkRunUsesRequestedRepository(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	var gotOwner string
	var gotRepo string

	opts := &ForkOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return fakeConfig{}, nil
		},
		ParseRepo: git.ParseRepo,
		ForkRepo: func(client *api.Client, owner, name string) (*api.Repository, error) {
			gotOwner = owner
			gotRepo = name
			if client.Token() != "test-token" {
				t.Fatalf("Token() = %q, want %q", client.Token(), "test-token")
			}
			return &api.Repository{
				FullName: "fork-owner/fork-repo",
				HTMLURL:  "https://gitcode.com/fork-owner/fork-repo",
			}, nil
		},
		CloneRepo: func(repo *git.Repo, dir string, protocol string, depth int) error {
			t.Fatalf("CloneRepo() should not be called when --clone is false")
			return nil
		},
		Repository: "infra-test/gctest1",
	}

	err := forkRun(opts)
	if err != nil {
		t.Fatalf("forkRun() error = %v", err)
	}

	if gotOwner != "infra-test" || gotRepo != "gctest1" {
		t.Fatalf("ForkRepo() called with %s/%s, want infra-test/gctest1", gotOwner, gotRepo)
	}
}

func TestForkRunUsesConfiguredTokenWhenEnvUnset(t *testing.T) {
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")
	t.Setenv("GC_CONFIG_DIR", t.TempDir())

	cfg := config.New()
	if _, err := cfg.Authentication().Login("gitcode.com", "demo", "config-token", "https", false); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	f := cmdutil.TestFactory()
	var gotToken string

	opts := &ForkOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return fakeConfig{}, nil
		},
		ParseRepo: git.ParseRepo,
		ForkRepo: func(client *api.Client, owner, name string) (*api.Repository, error) {
			gotToken = client.Token()
			return &api.Repository{
				FullName: "fork-owner/fork-repo",
				HTMLURL:  "https://gitcode.com/fork-owner/fork-repo",
			}, nil
		},
		CloneRepo: func(repo *git.Repo, dir string, protocol string, depth int) error {
			t.Fatalf("CloneRepo() should not be called when --clone is false")
			return nil
		},
		Repository: "infra-test/gctest1",
	}

	err := forkRun(opts)
	if err != nil {
		t.Fatalf("forkRun() error = %v", err)
	}
	if gotToken != "config-token" {
		t.Fatalf("client token = %q, want %q", gotToken, "config-token")
	}
}

func TestForkRunCloneUsesForkedRepository(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	t.Setenv("GC_GIT_PROTOCOL", "ssh")

	f := cmdutil.TestFactory()
	var clonedRepo *git.Repo
	var clonedProtocol string

	opts := &ForkOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return fakeConfig{}, nil
		},
		ParseRepo: git.ParseRepo,
		ForkRepo: func(client *api.Client, owner, name string) (*api.Repository, error) {
			return &api.Repository{
				FullName: "fork-owner/forked-gctest1",
				HTMLURL:  "https://gitcode.com/fork-owner/forked-gctest1",
			}, nil
		},
		CloneRepo: func(repo *git.Repo, dir string, protocol string, depth int) error {
			clonedRepo = repo
			clonedProtocol = protocol
			return nil
		},
		Repository: "infra-test/gctest1",
		Clone:      true,
	}

	err := forkRun(opts)
	if err != nil {
		t.Fatalf("forkRun() error = %v", err)
	}

	if clonedRepo == nil {
		t.Fatalf("CloneRepo() was not called")
	}
	if clonedRepo.Owner != "fork-owner" || clonedRepo.Name != "forked-gctest1" {
		t.Fatalf("CloneRepo() repo = %s/%s, want fork-owner/forked-gctest1", clonedRepo.Owner, clonedRepo.Name)
	}
	if clonedProtocol != "ssh" {
		t.Fatalf("CloneRepo() protocol = %q, want ssh", clonedProtocol)
	}
}

func TestForkRunRejectsInvalidRepository(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")

	f := cmdutil.TestFactory()
	opts := &ForkOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config: func() (config.Config, error) {
			return fakeConfig{}, nil
		},
		ParseRepo: git.ParseRepo,
		ForkRepo: func(client *api.Client, owner, name string) (*api.Repository, error) {
			t.Fatalf("ForkRepo() should not be called for invalid repository")
			return nil, nil
		},
		CloneRepo: func(repo *git.Repo, dir string, protocol string, depth int) error {
			t.Fatalf("CloneRepo() should not be called for invalid repository")
			return nil
		},
		Repository: "invalid",
	}

	err := forkRun(opts)
	if err == nil {
		t.Fatalf("forkRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid repository") {
		t.Fatalf("forkRun() error = %q, want substring %q", err.Error(), "invalid repository")
	}
}

type fakeConfig struct{}

func (fakeConfig) ResolutionErr() error                 { return nil }
func (fakeConfig) Get(host, key string) (string, error) { return "", nil }

func (fakeConfig) Set(host, key, value string) error { return nil }

func (fakeConfig) GitProtocol(host string) config.ConfigEntry {
	if value := strings.TrimSpace(strings.ToLower(os.Getenv("GC_GIT_PROTOCOL"))); value != "" {
		return config.ConfigEntry{Value: value, Source: "environment"}
	}
	return config.ConfigEntry{Value: "https", Source: "default"}
}

func (fakeConfig) Editor(host string) config.ConfigEntry { return config.ConfigEntry{} }

func (fakeConfig) Browser(host string) config.ConfigEntry { return config.ConfigEntry{} }

func (fakeConfig) Pager(host string) config.ConfigEntry { return config.ConfigEntry{} }

func (fakeConfig) Authentication() config.AuthConfig { return nil }

func (fakeConfig) Write() error { return nil }
