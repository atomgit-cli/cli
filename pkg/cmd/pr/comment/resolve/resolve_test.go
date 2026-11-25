package resolve

import (
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

func TestNewCmdResolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		args         []string
		wantErr      bool
		wantPRNum    int
		wantDiscID   string
		wantResolved bool
	}{
		{
			name:         "resolve with PR number and discussion ID",
			args:         []string{"123", "d1", "-R", "owner/repo"},
			wantPRNum:    123,
			wantDiscID:   "d1",
			wantResolved: true,
		},
		{
			name:         "resolve without repo flag",
			args:         []string{"42", "disc_abc"},
			wantPRNum:    42,
			wantDiscID:   "disc_abc",
			wantResolved: true,
		},
		{
			name:    "missing discussion ID",
			args:    []string{"123"},
			wantErr: true,
		},
		{
			name:    "missing all args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc", "d1"},
			wantErr: true,
		},
		{
			name:    "extra args",
			args:    []string{"123", "d1", "extra"},
			wantErr: true,
		},
		{
			name:    "negative PR number (cobra interprets -1 as flag)",
			args:    []string{"-1", "d1"},
			wantErr: true, // Cobra rejects "-1" as unknown shorthand flag
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			var gotOpts *ResolveOptions
			cmd := NewCmdResolve(f, func(opts *ResolveOptions) error {
				gotOpts = opts
				return nil
			})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if gotOpts.Number != tt.wantPRNum {
				t.Errorf("Number = %d, want %d", gotOpts.Number, tt.wantPRNum)
			}
			if gotOpts.DiscussionID != tt.wantDiscID {
				t.Errorf("DiscussionID = %q, want %q", gotOpts.DiscussionID, tt.wantDiscID)
			}
			if gotOpts.Resolved != tt.wantResolved {
				t.Errorf("Resolved = %v, want %v (resolve should set Resolved=true)", gotOpts.Resolved, tt.wantResolved)
			}
		})
	}
}

func TestNewCmdUnresolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		args         []string
		wantErr      bool
		wantPRNum    int
		wantDiscID   string
		wantResolved bool
	}{
		{
			name:         "unresolve with PR number and discussion ID",
			args:         []string{"123", "d1", "-R", "owner/repo"},
			wantPRNum:    123,
			wantDiscID:   "d1",
			wantResolved: false,
		},
		{
			name:         "unresolve without repo flag",
			args:         []string{"42", "disc_abc"},
			wantPRNum:    42,
			wantDiscID:   "disc_abc",
			wantResolved: false,
		},
		{
			name:    "missing discussion ID",
			args:    []string{"123"},
			wantErr: true,
		},
		{
			name:    "missing all args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid PR number",
			args:    []string{"abc", "d1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			var gotOpts *ResolveOptions
			cmd := NewCmdUnresolve(f, func(opts *ResolveOptions) error {
				gotOpts = opts
				return nil
			})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if gotOpts.Number != tt.wantPRNum {
				t.Errorf("Number = %d, want %d", gotOpts.Number, tt.wantPRNum)
			}
			if gotOpts.DiscussionID != tt.wantDiscID {
				t.Errorf("DiscussionID = %q, want %q", gotOpts.DiscussionID, tt.wantDiscID)
			}
			if gotOpts.Resolved != tt.wantResolved {
				t.Errorf("Resolved = %v, want %v (unresolve should set Resolved=false)", gotOpts.Resolved, tt.wantResolved)
			}
		})
	}
}

func TestResolveRunMissingRepo(t *testing.T) {
	t.Setenv("GC_TOKEN", "dummy-token")
	f := cmdutil.TestFactory()
	opts := &ResolveOptions{
		IO:           f.IOStreams,
		HttpClient:   f.HttpClient,
		BaseRepo:     func() (string, error) { return "", nil },
		Number:       123,
		DiscussionID: "d1",
		Resolved:     true,
	}

	err := resolveRun(opts)
	if err == nil {
		t.Error("resolveRun() should fail with empty repository")
	}
	if !strings.Contains(err.Error(), "no repository") {
		t.Errorf("error = %v, want containing 'no repository'", err)
	}
}
