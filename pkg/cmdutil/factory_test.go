package cmdutil

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNewFactoryReturnsUsableDefaults verifies that the default Factory wires
// non-nil dependency accessors so commands can rely on them without nil checks.
func TestNewFactoryReturnsUsableDefaults(t *testing.T) {
	f := NewFactory()

	if f == nil {
		t.Fatal("NewFactory() = nil, want non-nil Factory")
	}
	if f.IOStreams == nil {
		t.Fatal("NewFactory().IOStreams = nil, want system IOStreams")
	}

	httpClient, err := f.HttpClient()
	if err != nil {
		t.Fatalf("HttpClient() error = %v, want nil", err)
	}
	if httpClient == nil {
		t.Fatal("HttpClient() = nil, want non-nil http.Client")
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatalf("Config() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Config() = nil, want non-nil config")
	}
}

// TestNewFactoryBaseRepoInsideGitRepo verifies BaseRepo resolves the current
// repository when the working directory is inside a git repository (the test
// runs inside this repository's checkout).
func TestNewFactoryBaseRepoInsideGitRepo(t *testing.T) {
	f := NewFactory()

	repo, err := f.BaseRepo()
	if err != nil {
		t.Fatalf("BaseRepo() error = %v, want nil inside git repository", err)
	}
	if !strings.Contains(repo, "/") {
		t.Fatalf("BaseRepo() = %q, want owner/repo form", repo)
	}
}

// TestNewFactoryBranchInsideGitRepo verifies Branch resolves the current
// branch when the working directory is inside a git repository.
func TestNewFactoryBranchInsideGitRepo(t *testing.T) {
	f := NewFactory()

	branch, err := f.Branch()
	if err != nil {
		t.Fatalf("Branch() error = %v, want nil inside git repository", err)
	}
	if branch == "" || branch == "HEAD" {
		t.Fatalf("Branch() = %q, want a concrete branch name", branch)
	}
}

// TestNewFactoryBaseRepoOutsideGitRepo verifies the "not in a git repository"
// error path when the working directory is outside any git repository.
func TestNewFactoryBaseRepoOutsideGitRepo(t *testing.T) {
	restoreWd(t)

	f := NewFactory()

	_, err := f.BaseRepo()
	if err == nil {
		t.Fatal("BaseRepo() error = nil outside git repository, want error")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Fatalf("BaseRepo() error = %q, want 'not in a git repository'", err.Error())
	}
}

// TestNewFactoryBranchOutsideGitRepo verifies the Branch error path outside a
// git repository.
func TestNewFactoryBranchOutsideGitRepo(t *testing.T) {
	restoreWd(t)

	f := NewFactory()

	_, err := f.Branch()
	if err == nil {
		t.Fatal("Branch() error = nil outside git repository, want error")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Fatalf("Branch() error = %q, want 'not in a git repository'", err.Error())
	}
}

// TestTestFactoryReturnsStableDefaults verifies the test Factory stubs: IOStreams
// from iostreams.Test(), a default HTTP client, a fresh config, and fixed
// owner/repo + branch values.
func TestTestFactoryReturnsStableDefaults(t *testing.T) {
	f := TestFactory()

	if f == nil {
		t.Fatal("TestFactory() = nil, want non-nil Factory")
	}
	if f.IOStreams == nil {
		t.Fatal("TestFactory().IOStreams = nil, want test IOStreams")
	}

	httpClient, err := f.HttpClient()
	if err != nil {
		t.Fatalf("HttpClient() error = %v, want nil", err)
	}
	if httpClient == nil {
		t.Fatal("HttpClient() = nil, want non-nil http.Client")
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatalf("Config() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Config() = nil, want non-nil config")
	}

	repo, err := f.BaseRepo()
	if err != nil {
		t.Fatalf("BaseRepo() error = %v, want nil", err)
	}
	if repo != "owner/repo" {
		t.Fatalf("BaseRepo() = %q, want %q", repo, "owner/repo")
	}

	branch, err := f.Branch()
	if err != nil {
		t.Fatalf("Branch() error = %v, want nil", err)
	}
	if branch != "main" {
		t.Fatalf("Branch() = %q, want %q", branch, "main")
	}
}

// TestNewFactoryHTTPClientDebugMode verifies the debug branch wires a retry
// client with an stderr logger when GC_DEBUG is enabled.
func TestNewFactoryHTTPClientDebugMode(t *testing.T) {
	t.Setenv("GC_DEBUG", "1")

	f := NewFactory()

	httpClient, err := f.HttpClient()
	if err != nil {
		t.Fatalf("HttpClient() error = %v, want nil", err)
	}
	if httpClient == nil {
		t.Fatal("HttpClient() = nil, want non-nil http.Client")
	}
}

// TestNewFactoryBaseRepoWithoutRemote verifies the CurrentRepo error path in a
// git repository that has no remote configured.
func TestNewFactoryBaseRepoWithoutRemote(t *testing.T) {
	dir := initBareGitRepo(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp repo: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWd); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	f := NewFactory()

	_, err = f.BaseRepo()
	if err == nil {
		t.Fatal("BaseRepo() error = nil in repo without remote, want error")
	}
}

// TestNewFactoryBranchDetachedHead verifies the "could not determine current
// branch" path in a repository with HEAD detached at a commit.
func TestNewFactoryBranchDetachedHead(t *testing.T) {
	dir := initDetachedGitRepo(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp repo: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWd); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	f := NewFactory()

	_, err = f.Branch()
	if err == nil {
		t.Fatal("Branch() error = nil with detached HEAD, want error")
	}
	if !strings.Contains(err.Error(), "could not determine current branch") {
		t.Fatalf("Branch() error = %q, want 'could not determine current branch'", err.Error())
	}
}

// initBareGitRepo creates a fresh git repository with no commits and no
// remotes in a temp directory, returning its path.
func initBareGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}

// initDetachedGitRepo creates a git repository with a single commit and HEAD
// detached at it, so `git branch --show-current` returns empty.
func initDetachedGitRepo(t *testing.T) string {
	t.Helper()

	dir := initBareGitRepo(t)
	steps := [][]string{
		{"git", "-C", dir, "-c", "user.email=dev@example.invalid", "-c", "user.name=dev",
			"commit", "-q", "--allow-empty", "-m", "init"},
		{"git", "-C", dir, "checkout", "-q", "--detach"},
	}
	for _, args := range steps {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %v: %s", args, err, out)
		}
	}
	return dir
}

// restoreWd changes the working directory to a fresh temp directory (outside
// any git repository) for the duration of the test and restores the original
// directory afterwards.
func restoreWd(t *testing.T) {
	t.Helper()

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWd); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
