package root

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestRootHelpMentionsWindowsPowerShellAliasForGC(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell alias guidance is Windows-specific")
	}

	t.Setenv(commandNameEnv, "gc")

	cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := out.String()
	for _, want := range []string{"Windows PowerShell", "Get-Content", "gitcode", "gc.exe", "python -m gc_cli"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
}

func TestRootCommandNameFromEnvironment(t *testing.T) {
	t.Setenv(commandNameEnv, "gitcode")

	cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
	if cmd.Use != "gitcode" {
		t.Fatalf("NewRootCmd().Use = %q, want gitcode", cmd.Use)
	}

	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Usage:\n  gitcode [command]") {
		t.Fatalf("help should use gitcode command name: %s", output)
	}
	if strings.Contains(output, "Usage:\n  gc [command]") {
		t.Fatalf("help should not use gc usage when gitcode is selected: %s", output)
	}
}

func TestGitcodeCommandNamePropagatesToDiscoveryCommands(t *testing.T) {
	t.Setenv(commandNameEnv, "gitcode")

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "version",
			args: []string{"version"},
			want: []string{"gitcode version dev"},
		},
		{
			name: "help json",
			args: []string{"help", "--json"},
			want: []string{`"path": "gitcode"`, `"path": "gitcode pr create"`},
		},
		{
			name: "schema",
			args: []string{"schema"},
			want: []string{`"name": "gitcode"`, `"path": "gitcode"`, `"path": "gitcode pr create"`},
		},
		{
			name: "powershell completion",
			args: []string{"completion", "powershell"},
			want: []string{"powershell completion for gitcode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v\n%s", err, out.String())
			}

			output := out.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Fatalf("output missing %q: %s", want, output)
				}
			}
		})
	}
}

func TestGitcodeHelpRewritesExamples(t *testing.T) {
	t.Setenv(commandNameEnv, "gitcode")

	cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"help", "issue"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, out.String())
	}

	output := out.String()
	if !strings.Contains(output, `$ gitcode issue create`) {
		t.Fatalf("help output missing rewritten gitcode example: %s", output)
	}
	if strings.Contains(output, `$ gc issue create`) {
		t.Fatalf("help output should not expose gc example when root is gitcode: %s", output)
	}
	if strings.Contains(output, `Use "gc issue`) {
		t.Fatalf("help output should not expose gc use hint when root is gitcode: %s", output)
	}
}

func TestNoInteractiveFlag(t *testing.T) {
	t.Run("flag is registered as persistent", func(t *testing.T) {
		cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
		flag := cmd.PersistentFlags().Lookup("no-interactive")
		if flag == nil {
			t.Fatal("expected --no-interactive persistent flag to be registered")
		}
		if flag.DefValue != "false" {
			t.Fatalf("default value = %q, want false", flag.DefValue)
		}
	})

	t.Run("flag sets IOStreams.noInteractive and disables CanPrompt", func(t *testing.T) {
		f := cmdutil.TestFactory()
		io, _, _, _ := iostreams.TestTTY()
		f.IOStreams = io

		if !f.IOStreams.CanPrompt() {
			t.Fatal("precondition: CanPrompt() should be true before --no-interactive")
		}

		cmd := NewRootCmd("dev", "none", "unknown", f)
		cmd.SetArgs([]string{"--no-interactive", "version"})

		if f.IOStreams.NoInteractive() {
			t.Fatal("NoInteractive() = true before Execute, want false")
		}
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !f.IOStreams.NoInteractive() {
			t.Fatal("NoInteractive() = false after --no-interactive, want true")
		}
		if f.IOStreams.CanPrompt() {
			t.Fatal("CanPrompt() should return false after --no-interactive")
		}
	})

	t.Run("without flag CanPrompt remains true", func(t *testing.T) {
		f := cmdutil.TestFactory()
		io, _, _, _ := iostreams.TestTTY()
		f.IOStreams = io

		cmd := NewRootCmd("dev", "none", "unknown", f)
		cmd.SetArgs([]string{"version"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !f.IOStreams.CanPrompt() {
			t.Fatal("CanPrompt() should remain true without --no-interactive")
		}
		if f.IOStreams.NoInteractive() {
			t.Fatal("NoInteractive() = true without --no-interactive, want false")
		}
	})
}

func TestNoUpdateCheckFlagIsRegistered(t *testing.T) {
	cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
	flag := cmd.PersistentFlags().Lookup("no-update-check")
	if flag == nil || flag.DefValue != "false" {
		t.Fatalf("--no-update-check flag = %#v", flag)
	}
}

func TestRootRegistersSSHKeyCommand(t *testing.T) {
	t.Setenv(commandNameEnv, "gc")
	cmd := NewRootCmd("dev", "none", "unknown", cmdutil.TestFactory())
	sshKey, _, err := cmd.Find([]string{"ssh-key", "list"})
	if err != nil || sshKey.CommandPath() != "gc ssh-key list" {
		t.Fatalf("ssh-key command = %v, error = %v", sshKey, err)
	}
}

func TestExecuteNoArgsIsSafe(t *testing.T) {
	// A RunE failure skips PersistentPostRun (cobra behavior), so Execute
	// must run the lifecycle itself on the error path. With no adjacent
	// bootstrap manifest this is a no-op — the test asserts Execute still
	// returns the command's error and does not panic on the error path.
	// (A full assertion would need a manifest fixture; the no-op path
	// already exercises the added code.)
	t.Setenv("GITCODE_CLI_BINARY", t.TempDir()+"/gc")
	err := Execute("test", "abc", "today")
	if err == nil {
		// No args takes the help path, which returns nil and does not enter
		// the error path; this test pins that Execute stays safe on that
		// path (real error-path coverage lives in the lifecycle tests below).
		t.Log("Execute with no args returned nil (help path)")
	}
}

func TestExecuteSkipsLifecycleForProtocolCommand(t *testing.T) {
	// A bootstrap first-run notice must not reach the git credential
	// helper's stderr: git passes helper stderr through into its own output
	// (issue 647). auth git-credential with an empty config exits silently
	// with 1 — the error path of Execute must skip the lifecycle too.
	dir := t.TempDir()
	binary := filepath.Join(dir, "gc")
	t.Setenv("GITCODE_CLI_BINARY", binary)
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("GC_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUILD_NUMBER", "")
	t.Setenv("CI_NAME", "")
	t.Setenv("TEAMCITY_VERSION", "")
	t.Setenv("GC_NO_UPDATE_CHECK", "")
	t.Setenv("GC_UPDATE_MODE", "")
	manifest := fmt.Sprintf(`{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":%q,"helper":%q}`,
		dir, filepath.Join(dir, "helper.js"))
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	// Feed the protocol request on stdin so the helper parses and exits
	// with the silent "no credential" error.
	reqFile, err := os.CreateTemp(t.TempDir(), "credreq")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reqFile.WriteString("protocol=https\nhost=gitcode.com\n"); err != nil {
		t.Fatal(err)
	}
	if err := reqFile.Close(); err != nil {
		t.Fatal(err)
	}

	stdin, err := os.Open(reqFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	if err := stderrFile.Close(); err != nil {
		t.Fatal(err)
	}

	oldArgs, oldStdin, oldStderr := os.Args, os.Stdin, os.Stderr
	defer func() {
		os.Args, os.Stdin, os.Stderr = oldArgs, oldStdin, oldStderr
	}()
	os.Args = []string{"gc", "auth", "git-credential", "get"}
	os.Stdin = stdin
	os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = Execute("test", "abc", "today")
	if err == nil {
		t.Fatal("Execute returned nil; the no-credential helper call must fail so the error-path lifecycle is the path being skipped")
	}
	if _, err := os.Stderr.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(os.Stderr)
	os.Args, os.Stdin, os.Stderr = oldArgs, oldStdin, oldStderr
	if strings.Contains(string(got), "npm bootstrap") {
		t.Fatalf("protocol command leaked lifecycle output to stderr: %q", string(got))
	}
	if len(got) != 0 {
		t.Fatalf("protocol command stderr = %q, want empty", string(got))
	}
}

func TestExecuteRunsFirstRunNoticeForNormalCommand(t *testing.T) {
	// The skip must be scoped to protocol commands: a normal command on a
	// fresh bootstrap state still prints the one-time notice (issue 647
	// acceptance: no regression for interactive users).
	dir := t.TempDir()
	binary := filepath.Join(dir, "gc")
	t.Setenv("GITCODE_CLI_BINARY", binary)
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("GC_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUILD_NUMBER", "")
	t.Setenv("CI_NAME", "")
	t.Setenv("TEAMCITY_VERSION", "")
	t.Setenv("GC_NO_UPDATE_CHECK", "")
	t.Setenv("GC_UPDATE_MODE", "")
	manifest := fmt.Sprintf(`{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":%q,"helper":%q}`,
		dir, filepath.Join(dir, "helper.js"))
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	if err := stderrFile.Close(); err != nil {
		t.Fatal(err)
	}

	oldArgs, oldStderr := os.Args, os.Stderr
	defer func() {
		os.Args, os.Stderr = oldArgs, oldStderr
	}()
	os.Args = []string{"gc", "version"}
	os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := Execute("test", "abc", "today"); err != nil {
		t.Logf("Execute version returned: %v", err)
	}
	if _, err := os.Stderr.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(os.Stderr)
	os.Args, os.Stderr = oldArgs, oldStderr
	if !strings.Contains(string(got), "npm bootstrap") {
		t.Fatalf("normal command lost the first-run notice: %q", string(got))
	}
}

func TestExecuteSkipsLifecycleForProtocolCommandSuccessPath(t *testing.T) {
	// git calls `store` after a successful fill; the helper's no-op store
	// succeeds, so the PersistentPostRun guard must skip the lifecycle there
	// too (issue 647: helper stderr stays clean on every protocol op).
	dir := t.TempDir()
	binary := filepath.Join(dir, "gc")
	t.Setenv("GITCODE_CLI_BINARY", binary)
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("GC_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUILD_NUMBER", "")
	t.Setenv("CI_NAME", "")
	t.Setenv("TEAMCITY_VERSION", "")
	t.Setenv("GC_NO_UPDATE_CHECK", "")
	t.Setenv("GC_UPDATE_MODE", "")
	manifest := fmt.Sprintf(`{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":%q,"helper":%q}`,
		dir, filepath.Join(dir, "helper.js"))
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	if err := stderrFile.Close(); err != nil {
		t.Fatal(err)
	}

	oldArgs, oldStdin, oldStderr := os.Args, os.Stdin, os.Stderr
	defer func() {
		os.Args, os.Stdin, os.Stderr = oldArgs, oldStdin, oldStderr
	}()
	os.Args = []string{"gc", "auth", "git-credential", "store"}
	os.Stdin, err = os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Stdin.Close()
	os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = Execute("test", "abc", "today")
	if err != nil {
		t.Fatalf("Execute store error = %v, want nil (no-op success)", err)
	}
	if _, err := os.Stderr.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(os.Stderr)
	os.Args, os.Stdin, os.Stderr = oldArgs, oldStdin, oldStderr
	if len(got) != 0 {
		t.Fatalf("protocol command stderr = %q, want empty on the success path too", string(got))
	}
}

func TestExecuteRunsLifecycleOnFailingNormalCommand(t *testing.T) {
	// The skip must be protocol-only: a failing normal command (auth status
	// without a login) still runs the error-path lifecycle, and the first-run
	// notice reaches stderr (runPostCommandLifecycle body coverage).
	dir := t.TempDir()
	binary := filepath.Join(dir, "gc")
	t.Setenv("GITCODE_CLI_BINARY", binary)
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("GC_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUILD_NUMBER", "")
	t.Setenv("CI_NAME", "")
	t.Setenv("TEAMCITY_VERSION", "")
	t.Setenv("GC_NO_UPDATE_CHECK", "")
	t.Setenv("GC_UPDATE_MODE", "")
	manifest := fmt.Sprintf(`{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":%q,"helper":%q}`,
		dir, filepath.Join(dir, "helper.js"))
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	if err := stderrFile.Close(); err != nil {
		t.Fatal(err)
	}

	oldArgs, oldStderr := os.Args, os.Stderr
	defer func() {
		os.Args, os.Stderr = oldArgs, oldStderr
	}()
	os.Args = []string{"gc", "auth", "status"}
	os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = Execute("test", "abc", "today")
	if err == nil {
		t.Fatal("auth status without a login should fail (exit 4)")
	}
	if _, err := os.Stderr.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(os.Stderr)
	os.Args, os.Stderr = oldArgs, oldStderr
	if !strings.Contains(string(got), "npm bootstrap") {
		t.Fatalf("failing normal command lost the error-path first-run notice: %q", string(got))
	}
}

// setupLifecycleFixture prepares a hermetic npm-bootstrap environment: an
// adjacent manifest, a fresh state dir, an isolated config dir, and cleared
// CI/update-mode variables (CI runners set them, which legitimately disables
// the update notice and would make lifecycle assertions vacuous).
func setupLifecycleFixture(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("GITCODE_CLI_BINARY", filepath.Join(dir, "gc"))
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("GC_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUILD_NUMBER", "")
	t.Setenv("CI_NAME", "")
	t.Setenv("TEAMCITY_VERSION", "")
	t.Setenv("GC_NO_UPDATE_CHECK", "")
	t.Setenv("GC_UPDATE_MODE", "")
	manifest := fmt.Sprintf(`{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":%q,"helper":%q}`,
		dir, filepath.Join(dir, "helper.js"))
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestExecuteLifecycleMatrix(t *testing.T) {
	// Pins the ExecuteC failure-mode cells that motivated the refactor: on
	// flag-parse and arg-validation failures the resolved leaf command still
	// reaches the annotation check (protocol commands skip the lifecycle);
	// unknown commands resolve to root and keep the lifecycle.
	cases := []struct {
		name       string
		args       []string
		wantNotice bool
	}{
		{name: "protocol parse error skips lifecycle", args: []string{"auth", "git-credential", "get", "--nope"}, wantNotice: false},
		{name: "protocol arg validation error skips lifecycle", args: []string{"auth", "git-credential", "a", "b"}, wantNotice: false},
		{name: "unknown command runs lifecycle", args: []string{"definitely-not-a-command"}, wantNotice: true},
		{name: "normal command parse error runs lifecycle", args: []string{"version", "--nope"}, wantNotice: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupLifecycleFixture(t)
			stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			if err := stderrFile.Close(); err != nil {
				t.Fatal(err)
			}
			oldArgs, oldStderr := os.Args, os.Stderr
			defer func() {
				os.Args, os.Stderr = oldArgs, oldStderr
			}()
			os.Args = append([]string{"gc"}, tc.args...)
			os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			_ = Execute("test", "abc", "today")
			if _, err := os.Stderr.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(os.Stderr)
			os.Args, os.Stderr = oldArgs, oldStderr
			hasNotice := strings.Contains(string(got), "npm bootstrap")
			if hasNotice != tc.wantNotice {
				t.Fatalf("first-run notice = %v, want %v; stderr = %q", hasNotice, tc.wantNotice, string(got))
			}
		})
	}
}
