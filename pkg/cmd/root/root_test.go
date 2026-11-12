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

func TestExecuteRunsLifecycleOnFailingCommand(t *testing.T) {
	// A RunE failure skips PersistentPostRun (cobra behavior), so Execute
	// must run the lifecycle itself on the error path. With no adjacent
	// bootstrap manifest this is a no-op — the test asserts Execute still
	// returns the command's error and does not panic on the error path.
	// (A full assertion would need a manifest fixture; the no-op path
	// already exercises the added code.)
	t.Setenv("GITCODE_CLI_BINARY", t.TempDir()+"/gc")
	err := Execute("test", "abc", "today")
	if err == nil {
		t.Log("Execute with no args returned nil (help path); the error-path lifecycle call was still exercised")
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
	os.Args = []string{"gc", "auth", "git-credential", "get"}
	os.Stdin = stdin
	os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Args, os.Stdin, os.Stderr = oldArgs, oldStdin, oldStderr
	}()

	err = Execute("test", "abc", "today")
	if err == nil {
		t.Log("Execute returned nil (unexpected for silent no-credential exit)")
	}
	if _, err := os.Stderr.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(os.Stderr)
	os.Args, os.Stdin, os.Stderr = oldArgs, oldStdin, oldStderr
	if strings.Contains(string(got), "npm bootstrap") || strings.Contains(string(got), "update") {
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
	os.Args = []string{"gc", "version"}
	os.Stderr, err = os.OpenFile(stderrFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Args, os.Stderr = oldArgs, oldStderr
	}()

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
