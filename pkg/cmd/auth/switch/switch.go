// Package switchcmd implements the auth switch command.
package switchcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type SwitchOptions struct {
	IO     *iostreams.IOStreams
	Config func() (config.Config, error)

	// Arguments
	Username string

	// Flags
	Hostname string
	JSON     bool
}

// switchJSON is the stable --json output shape. Field order is part of the
// output contract: hostname, active_user.
type switchJSON struct {
	Hostname   string `json:"hostname"`
	ActiveUser string `json:"active_user"`
}

// NewCmdSwitch creates the auth switch command.
func NewCmdSwitch(f *cmdutil.Factory, runF func(*SwitchOptions) error) *cobra.Command {
	opts := &SwitchOptions{
		IO:     f.IOStreams,
		Config: f.Config,
	}

	cmd := &cobra.Command{
		Use:   "switch [<username>]",
		Short: "Switch the active account for a host",
		Long: heredoc.Doc(`
			Switch the active account on a host between locally stored
			logins. Each host keeps one active_user; all authenticated
			commands use that account's token afterwards.

			With no argument, an interactive prompt lists the stored
			accounts for the host (the current one is marked). In
			non-interactive contexts pass the username explicitly.

			Switching is a local operation: it rewrites active_user in the
			config directory's auth.json and never calls the API. Tokens
			provided by GC_TOKEN/GITCODE_TOKEN environment variables keep
			overriding any switched account (see docs/AUTH.md).
		`),
		Example: heredoc.Doc(`
			# Switch to a stored account on the default host
			$ gc auth switch second-account

			# Switch interactively (lists the stored accounts)
			$ gc auth switch

			# Switch on a specific host
			$ gc auth switch deploy-bot --hostname gitcode.com

			# Machine-readable output
			$ gc auth switch second-account --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Username = args[0]
			}
			if runF != nil {
				return runF(opts)
			}
			return switchRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Hostname, "hostname", "H", "", "Switch the account on a specific hostname")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func switchRun(opts *SwitchOptions) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	authCfg := cfg.Authentication()

	hostname := opts.Hostname
	if hostname == "" {
		if resolved, _ := authCfg.DefaultHost(); resolved != "" {
			hostname = resolved
		}
	}
	hostname, err = config.NormalizeTrustedHost(hostname)
	if err != nil {
		return err
	}

	users := authCfg.LoginUsers(hostname)
	if len(users) == 0 {
		return cmdutil.NewNotFoundError(fmt.Sprintf("no stored accounts for %s; run gc auth login to add one", hostname), nil)
	}
	// Stored account names are echoed in listings, prompts, and error
	// hints; a corrupted auth.json must not inject terminal escape
	// sequences through them (git-credential applies the same idea to its
	// protocol output).
	for _, name := range users {
		if cmdutil.HasTerminalUnsafeChars(name) {
			return fmt.Errorf("corrupted login state for %s: stored account name contains control characters; inspect auth.json", hostname)
		}
	}

	username := opts.Username
	if username == "" {
		if !opts.IO.CanPrompt() {
			return cmdutil.NewUsageError("interactive switch requires a TTY; pass a username (gc auth switch <username>)")
		}
		username, err = promptSwitch(opts.IO, authCfg, hostname, users)
		if err != nil {
			return err
		}
	}

	if !slices.Contains(users, username) {
		return cmdutil.NewNotFoundError(fmt.Sprintf("account %q is not stored for %s; available: %s", username, hostname, strings.Join(users, ", ")), nil)
	}

	if err := authCfg.SwitchUser(hostname, username); err != nil {
		return fmt.Errorf("failed to switch account: %w", err)
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, switchJSON{
			Hostname:   hostname,
			ActiveUser: username,
		})
	}

	cs := opts.IO.ColorScheme()
	fmt.Fprintf(opts.IO.Out, "%s Switched to %s on %s\n", cs.Green("✓"), cs.Bold(username), hostname)
	return nil
}

// promptSwitch lists the stored accounts with the current one marked and
// reads one selection, accepting either the number or the exact username.
func promptSwitch(streams *iostreams.IOStreams, authCfg config.AuthConfig, hostname string, users []string) (string, error) {
	current, _ := authCfg.ActiveUser(hostname)
	cs := streams.ColorScheme()

	// The prompt block goes to stderr so machine-readable stdout (e.g.
	// --json) stays pure while the interactive selection stays visible.
	fmt.Fprintf(streams.ErrOut, "? Switch account on %s:\n", hostname)
	for i, name := range users {
		marker := ""
		if name == current {
			marker = cs.Gray(" (current)")
		}
		fmt.Fprintf(streams.ErrOut, "  %d. %s%s\n", i+1, name, marker)
	}
	fmt.Fprintf(streams.ErrOut, "Choose an account [1-%d]: ", len(users))

	line, err := bufio.NewReader(streams.In).ReadString('\n')
	// Surface read errors even when partial input was received (#361
	// lesson); a clean EOF with a trailing partial line is still a valid
	// selection (no trailing newline).
	if err != nil && !errors.Is(err, io.EOF) {
		return "", cmdutil.NewUsageError(fmt.Sprintf("failed to read selection: %v", err))
	}
	choice := strings.TrimSpace(line)

	if n, convErr := strconv.Atoi(choice); convErr == nil {
		if n >= 1 && n <= len(users) {
			return users[n-1], nil
		}
	}
	if slices.Contains(users, choice) {
		return choice, nil
	}
	return "", cmdutil.NewUsageError(fmt.Sprintf("invalid selection %q; choose 1-%d or one of: %s", choice, len(users), strings.Join(users, ", ")))
}
