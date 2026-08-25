package clone

import (
	"fmt"
	"io"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestNewCmdClone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "clone with owner/repo format",
			args:    []string{"owner/repo"},
			wantErr: false,
		},
		{
			name:    "clone with full URL",
			args:    []string{"https://gitcode.com/owner/repo.git"},
			wantErr: false,
		},
		{
			name:    "clone with SSH URL",
			args:    []string{"git@gitcode.com:owner/repo.git"},
			wantErr: false,
		},
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdClone(f, func(opts *CloneOptions) error {
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

func TestParseRepoURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		repo     string
		protocol string
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "https URL",
			repo:     "https://gitcode.com/owner/repo.git",
			protocol: "https",
			wantURL:  "https://gitcode.com/owner/repo.git",
			wantErr:  false,
		},
		{
			name:     "ssh URL",
			repo:     "git@gitcode.com:owner/repo.git",
			protocol: "ssh",
			wantURL:  "git@gitcode.com:owner/repo.git",
			wantErr:  false,
		},
		{
			name:     "owner/repo format with https",
			repo:     "owner/repo",
			protocol: "https",
			wantURL:  "https://gitcode.com/owner/repo.git",
			wantErr:  false,
		},
		{
			name:     "owner/repo format with ssh",
			repo:     "owner/repo",
			protocol: "ssh",
			wantURL:  "git@gitcode.com:owner/repo.git",
			wantErr:  false,
		},
		{
			name:     "invalid format",
			repo:     "invalid",
			protocol: "https",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotURL, err := parseRepoURL(tt.repo, tt.protocol, io.Discard)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotURL != tt.wantURL {
				t.Errorf("parseRepoURL() = %v, want %v", gotURL, tt.wantURL)
			}
		})
	}
}

func TestResolveGitProtocolUsesConfig(t *testing.T) {
	t.Parallel()
	opts := &CloneOptions{
		Config: func() (config.Config, error) {
			return cloneConfig{protocol: "ssh"}, nil
		},
	}

	got, err := resolveGitProtocol(opts)
	if err != nil {
		t.Fatalf("resolveGitProtocol() error = %v", err)
	}
	if got != "ssh" {
		t.Fatalf("resolveGitProtocol() = %q, want ssh", got)
	}
}

func TestResolveGitProtocolFlagOverridesConfig(t *testing.T) {
	t.Parallel()
	opts := &CloneOptions{
		GitProtocol: "https",
		Config: func() (config.Config, error) {
			return cloneConfig{protocol: "ssh"}, nil
		},
	}

	got, err := resolveGitProtocol(opts)
	if err != nil {
		t.Fatalf("resolveGitProtocol() error = %v", err)
	}
	if got != "https" {
		t.Fatalf("resolveGitProtocol() = %q, want https", got)
	}
}

func TestResolveGitProtocolDefaultsToSSHWithoutConfig(t *testing.T) {
	t.Parallel()
	opts := &CloneOptions{}

	got, err := resolveGitProtocol(opts)
	if err != nil {
		t.Fatalf("resolveGitProtocol() error = %v", err)
	}
	if got != "ssh" {
		t.Fatalf("resolveGitProtocol() = %q, want ssh", got)
	}
}

func TestResolveGitProtocolDefaultsToSSHWhenConfigUnset(t *testing.T) {
	t.Parallel()
	opts := &CloneOptions{
		Config: func() (config.Config, error) {
			return cloneConfig{}, nil
		},
	}

	got, err := resolveGitProtocol(opts)
	if err != nil {
		t.Fatalf("resolveGitProtocol() error = %v", err)
	}
	if got != "ssh" {
		t.Fatalf("resolveGitProtocol() = %q, want ssh", got)
	}
}

func TestCloneRunUsesCorrectURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		repo     string
		protocol string
		wantURL  string
	}{
		{
			name:     "owner/repo format with https",
			repo:     "owner/repo",
			protocol: "https",
			wantURL:  "https://gitcode.com/owner/repo.git",
		},
		{
			name:     "owner/repo format with ssh",
			repo:     "owner/repo",
			protocol: "ssh",
			wantURL:  "git@gitcode.com:owner/repo.git",
		},
		{
			name:     "full URL passed through",
			repo:     "https://gitcode.com/owner/repo.git",
			protocol: "https",
			wantURL:  "https://gitcode.com/owner/repo.git",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var capturedArgs []string
			io, _, _, _ := iostreams.Test()

			opts := &CloneOptions{
				IO:          io,
				Repository:  tt.repo,
				GitProtocol: tt.protocol,
				GitClone: func(gitArgs []string, opts *CloneOptions) error {
					capturedArgs = append([]string{}, gitArgs...)
					return nil
				},
			}

			err := cloneRun(opts)
			if err != nil {
				t.Fatalf("cloneRun() error = %v", err)
			}

			// Find the URL argument (should be after "clone" and any flags)
			found := false
			for _, arg := range capturedArgs {
				arg := arg
				if arg == tt.wantURL {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("cloneRun() args = %v, want URL %q in args", capturedArgs, tt.wantURL)
			}
		})
	}
}

func TestCloneRunWithDepthFlag(t *testing.T) {
	t.Parallel()
	var capturedArgs []string
	io, _, _, _ := iostreams.Test()

	opts := &CloneOptions{
		IO:          io,
		Repository:  "owner/repo",
		GitProtocol: "https",
		Depth:       1,
		GitClone: func(gitArgs []string, opts *CloneOptions) error {
			capturedArgs = append([]string{}, gitArgs...)
			return nil
		},
	}

	err := cloneRun(opts)
	if err != nil {
		t.Fatalf("cloneRun() error = %v", err)
	}

	// Verify --depth flag is present with value
	foundDepth := false
	for i, arg := range capturedArgs {
		if arg == "--depth" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "1" {
			foundDepth = true
			break
		}
	}
	if !foundDepth {
		t.Errorf("cloneRun() args = %v, want --depth 1", capturedArgs)
	}
}

func TestCloneRunWithBranchFlag(t *testing.T) {
	t.Parallel()
	var capturedArgs []string
	io, _, _, _ := iostreams.Test()

	opts := &CloneOptions{
		IO:         io,
		Repository: "owner/repo",
		Branch:     "develop",
		GitClone: func(gitArgs []string, opts *CloneOptions) error {
			capturedArgs = append([]string{}, gitArgs...)
			return nil
		},
	}

	err := cloneRun(opts)
	if err != nil {
		t.Fatalf("cloneRun() error = %v", err)
	}

	foundBranch := false
	for i, arg := range capturedArgs {
		if arg == "--branch" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "develop" {
			foundBranch = true
			break
		}
	}
	if !foundBranch {
		t.Errorf("cloneRun() args = %v, want --branch develop", capturedArgs)
	}
}

func TestCloneRunGitFailure(t *testing.T) {
	t.Parallel()
	io, _, _, _ := iostreams.Test()

	opts := &CloneOptions{
		IO:         io,
		Repository: "owner/repo",
		GitClone: func(gitArgs []string, opts *CloneOptions) error {
			return fmt.Errorf("git clone failed: network error")
		},
	}

	err := cloneRun(opts)
	if err == nil {
		t.Fatal("cloneRun() expected error, got nil")
	}
}

func TestCloneRunInvalidDepth(t *testing.T) {
	t.Parallel()
	io, _, _, _ := iostreams.Test()

	opts := &CloneOptions{
		IO:         io,
		Repository: "owner/repo",
		Depth:      -1,
	}

	err := cloneRun(opts)
	if err == nil {
		t.Fatal("cloneRun() expected error for negative depth, got nil")
	}
}

func TestCloneRunInvalidBranch(t *testing.T) {
	t.Parallel()
	io, _, _, _ := iostreams.Test()

	opts := &CloneOptions{
		IO:         io,
		Repository: "owner/repo",
		Branch:     "-bad-branch",
	}

	err := cloneRun(opts)
	if err == nil {
		t.Fatal("cloneRun() expected error for invalid branch name, got nil")
	}
}

func TestCloneRunRejectsOptionInjectionDirectory(t *testing.T) {
	t.Parallel()
	injections := []string{
		"--config=/tmp/evil",
		"--template=/tmp/malicious",
		"--upload-pack=/tmp/evil",
		"-c core.gitProxy=evil",
		"--separate-git-dir=/tmp/evil",
	}

	for _, inj := range injections {
		inj := inj
		t.Run(inj, func(t *testing.T) {
			t.Parallel()
			io, _, _, _ := iostreams.Test()

			opts := &CloneOptions{
				IO:         io,
				Repository: "owner/repo",
				Directory:  inj,
				GitClone: func(gitArgs []string, opts *CloneOptions) error {
					t.Errorf("GitClone should not be called for injection attempt %q, args=%v", inj, gitArgs)
					return nil
				},
			}

			err := cloneRun(opts)
			if err == nil {
				t.Fatalf("cloneRun() expected error for injection directory %q, got nil", inj)
			}
		})
	}
}

func TestCloneRunUsesSeparatorForDirectory(t *testing.T) {
	t.Parallel()
	var capturedArgs []string
	io, _, _, _ := iostreams.Test()

	opts := &CloneOptions{
		IO:          io,
		Repository:  "owner/repo",
		Directory:   "my-project",
		GitProtocol: "https",
		GitClone: func(gitArgs []string, opts *CloneOptions) error {
			capturedArgs = append([]string{}, gitArgs...)
			return nil
		},
	}

	err := cloneRun(opts)
	if err != nil {
		t.Fatalf("cloneRun() error = %v", err)
	}

	// Verify "--" separator is present before the repository URL
	foundSep := false
	foundDir := false
	for i, arg := range capturedArgs {
		if arg == "--" {
			foundSep = true
			// Directory should appear after "--"
			for j := i + 1; j < len(capturedArgs); j++ {
				if capturedArgs[j] == "my-project" {
					foundDir = true
					break
				}
			}
			break
		}
	}
	if !foundSep {
		t.Errorf("cloneRun() args = %v, want \"--\" separator", capturedArgs)
	}
	if !foundDir {
		t.Errorf("cloneRun() args = %v, want directory \"my-project\" after \"--\"", capturedArgs)
	}
}

func TestCloneRunAcceptsValidDirectory(t *testing.T) {
	t.Parallel()
	validDirs := []string{
		"my-project",
		"subdir/my-project",
		"my project",
		"我的项目",
		"./subdir/repo",
	}

	for _, dir := range validDirs {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			var capturedArgs []string
			io, _, _, _ := iostreams.Test()

			opts := &CloneOptions{
				IO:          io,
				Repository:  "owner/repo",
				Directory:   dir,
				GitProtocol: "https",
				GitClone: func(gitArgs []string, opts *CloneOptions) error {
					capturedArgs = append([]string{}, gitArgs...)
					return nil
				},
			}

			err := cloneRun(opts)
			if err != nil {
				t.Fatalf("cloneRun() unexpected error for directory %q: %v", dir, err)
			}

			// Verify directory appears in args
			found := false
			for _, arg := range capturedArgs {
				arg := arg
				if arg == dir {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("cloneRun() args = %v, want directory %q in args", capturedArgs, dir)
			}
		})
	}
}

type cloneConfig struct {
	protocol string
}

func (c cloneConfig) Get(host, key string) (string, error) { return "", nil }
func (c cloneConfig) Set(host, key, value string) error    { return nil }
func (c cloneConfig) GitProtocol(host string) config.ConfigEntry {
	return config.ConfigEntry{Value: c.protocol, Source: "test"}
}
func (c cloneConfig) Editor(host string) config.ConfigEntry  { return config.ConfigEntry{} }
func (c cloneConfig) Browser(host string) config.ConfigEntry { return config.ConfigEntry{} }
func (c cloneConfig) Pager(host string) config.ConfigEntry   { return config.ConfigEntry{} }
func (c cloneConfig) Authentication() config.AuthConfig      { return nil }
func (c cloneConfig) Write() error                           { return nil }
