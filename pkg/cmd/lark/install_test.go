package lark

import (
	"bytes"
	"io"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/larkcli"
)

func TestInstallRun_Success(t *testing.T) {
	called := false
	streams, _, out, _ := iostreams.Test()
	opts := &installOptions{
		IO: streams,
		installFn: func(stdout, stderr io.Writer) error {
			called = true
			stdout.Write([]byte("installing...\n"))
			return nil
		},
	}
	// FindLarkCLI resolves a path so the "installed but not on PATH" branch
	// is skipped; point env at the running test binary (cross-platform stub,
	// replaces /bin/true which only exists on Linux — issue #499).
	t.Setenv("GC_LARK_CLI_BIN", existingExecutable(t))
	if err := installRun(opts); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !called {
		t.Fatal("installFn not invoked")
	}
	if !bytes.Contains(out.Bytes(), []byte("lark-cli ready")) {
		t.Errorf("out = %q, want ready message", out.String())
	}
}

func TestInstallRun_InstallFails(t *testing.T) {
	t.Parallel()
	streams, _, _, _ := iostreams.Test()
	opts := &installOptions{
		IO: streams,
		installFn: func(stdout, stderr io.Writer) error {
			return cmdutil.NewCLIError(cmdutil.ExitError, "npx failed", nil)
		},
	}
	if err := installRun(opts); err == nil {
		t.Fatal("err = nil, want install failure")
	}
}

func TestNewCmdInstall_Builds(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	if cmd := newCmdInstall(f, nil); cmd == nil {
		t.Fatal("cmd nil")
	}
}

func TestDefaultInstall_ErrorsWhenNpxMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GC_LARK_CLI_BIN", "")
	var out, errOut bytes.Buffer
	if err := defaultInstall(&out, &errOut); err == nil {
		t.Fatal("err = nil, want failure when npx missing")
	}
}

// ensure larkcli.Installer is referenced so the import stays used in tests.
var _ = larkcli.Installer{}
