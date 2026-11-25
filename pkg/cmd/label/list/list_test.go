package list

import (
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdList(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdList(f, func(opts *ListOptions) error {
		return nil
	})

	if cmd == nil {
		t.Fatal("NewCmdList returned nil")
	}
	if cmd.Use != "list" {
		t.Errorf("Expected Use 'list', got %q", cmd.Use)
	}
}

func TestNewCmdListJSONFlag(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	var gotJSON bool

	cmd := NewCmdList(f, func(opts *ListOptions) error {
		gotJSON = opts.JSON
		return nil
	})
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !gotJSON {
		t.Fatal("expected --json flag to set opts.JSON")
	}
}

func TestNewCmdListLimitFlagExists(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdList(f, nil)

	if flag := cmd.Flags().Lookup("limit"); flag == nil {
		t.Fatal("expected limit flag to exist")
	}
}
