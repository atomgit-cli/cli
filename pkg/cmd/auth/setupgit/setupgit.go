// Package setupgit implements the auth setup-git command.
package setupgit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// SetupGitOptions holds the configuration for the auth setup-git command.
type SetupGitOptions struct {
	IO     *iostreams.IOStreams
	Config func() (config.Config, error)

	// Flags
	Hostname    string
	HostnameSet bool

	// runGitConfig executes `git config` with the given args and returns stdout.
	// Injected by tests; defaults to a real exec call.
	runGitConfig func(args ...string) (string, error)
	// resolveExecutable returns the path to the gc executable.
	// Injected by tests; defaults to os.Executable.
	resolveExecutable func() (string, error)
}

// NewCmdSetupGit creates the auth setup-git command.
func NewCmdSetupGit(f *cmdutil.Factory, runF func(*SetupGitOptions) error) *cobra.Command {
	opts := &SetupGitOptions{
		IO:     f.IOStreams,
		Config: f.Config,
	}

	cmd := &cobra.Command{
		Use:   "setup-git",
		Short: "Configure git to use gc as a credential helper",
		Long: heredoc.Doc(`
			Configure git to use gc as a credential helper so that git
			operations against GitCode automatically carry authentication.

			The credential helper entry is appended for the host without
			overwriting existing credential helper configuration. Re-running
			the command is safe: duplicate entries are skipped.

			This is a client-side command: it does not call the GitCode API
			and does not read or print tokens.

			The configured helper points to the gc auth git-credential
			subcommand, which fills the username and token of the account
			logged in via gc auth login for HTTPS git operations.
		`),
		Example: heredoc.Doc(`
			# Configure the credential helper for the default host
			$ gc auth setup-git

			# Configure for a specific hostname
			$ gc auth setup-git --hostname gitcode.com
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.HostnameSet = cmd.Flags().Changed("hostname")
			if runF != nil {
				return runF(opts)
			}
			return setupGitRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Hostname, "hostname", "H", "", "Configure the credential helper for a specific hostname")

	return cmd
}

func setupGitRun(opts *SetupGitOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if opts.Hostname == "" {
		opts.Hostname, _ = cfg.Authentication().DefaultHost()
	}
	opts.Hostname, err = config.NormalizeTrustedHost(opts.Hostname)
	if err != nil {
		return err
	}

	resolveExecutable := opts.resolveExecutable
	if resolveExecutable == nil {
		resolveExecutable = os.Executable
	}
	gcPath, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("failed to resolve gc executable path: %w", err)
	}

	helperValue := fmt.Sprintf("!\"%s\" auth git-credential", gcPath)
	configKey := fmt.Sprintf("credential.https://%s.helper", opts.Hostname)

	runGitConfig := opts.runGitConfig
	if runGitConfig == nil {
		runGitConfig = defaultRunGitConfig
	}

	// Idempotent: skip if the helper entry already exists. `git config
	// --get-all` exits 1 when the key is absent (expected on first run);
	// real exec errors surface at the --add call below.
	existing, _ := runGitConfig("--global", "--get-all", configKey)
	if hasHelperValue(existing, helperValue) {
		fmt.Fprintf(opts.IO.Out, "%s Credential helper already configured for %s\n", opts.IO.ColorScheme().SuccessIcon(), opts.Hostname)
		fmt.Fprintf(opts.IO.Out, "  Verify with: git config --global --get-all %s\n", configKey)
		return nil
	}

	if _, err := runGitConfig("--global", "--add", configKey, helperValue); err != nil {
		return fmt.Errorf("failed to configure git credential helper: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Configured git credential helper for %s\n", opts.IO.ColorScheme().SuccessIcon(), opts.Hostname)
	fmt.Fprintf(opts.IO.Out, "  Verify with: git config --global --get-all %s\n", configKey)
	return nil
}

// hasHelperValue reports whether the helper value is already present in the
// multi-valued `git config --get-all` output.
func hasHelperValue(getAllOutput, helperValue string) bool {
	for _, line := range strings.Split(strings.TrimSpace(getAllOutput), "\n") {
		if strings.TrimSpace(line) == helperValue {
			return true
		}
	}
	return false
}

func defaultRunGitConfig(args ...string) (string, error) {
	cmdArgs := append([]string{"config"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	return string(out), err
}
