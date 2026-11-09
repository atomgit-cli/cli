// Package logout implements the auth logout command
package logout

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/envtoken"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type LogoutOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	Config     func() (config.Config, error)

	// Flags
	Hostname string
	Username string
	Yes      bool
}

// NewCmdLogout creates the logout command
func NewCmdLogout(f *cmdutil.Factory, runF func(*LogoutOptions) error) *cobra.Command {
	opts := &LogoutOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config:     f.Config,
	}

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of a GitCode account",
		Long: heredoc.Doc(`
			Log out of a GitCode account.

			This removes the stored authentication token.
			Hostnames are normalized and validated before logout.

			On the default host, token environment variables are not removed.
			The command prints current-session cleanup guidance on stderr.

				Non-interactive mode: Requires --yes to skip confirmation.
		`),
		Example: heredoc.Doc(`
			# Interactive logout with confirmation
			$ gc auth logout
			! This will log out of gitcode.com
			Type the hostname to confirm: gitcode.com
			✓ Cleared stored authentication for gitcode.com

			# Logout from a specific hostname
			$ gc auth logout --hostname gitcode.com

			# Skip confirmation prompt
			$ gc auth logout --yes
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return logoutRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Hostname, "hostname", "H", "", "Hostname of the GitCode instance to log out of (hostname only; normalized and validated)")
	cmd.Flags().StringVarP(&opts.Username, "username", "u", "", "The username to log out of")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "Skip confirmation prompt")

	return cmd
}

func logoutRun(opts *LogoutOptions) error {
	// Set default hostname
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	authCfg := cfg.Authentication()
	if opts.Hostname == "" {
		opts.Hostname, _ = authCfg.DefaultHost()
	}
	opts.Hostname, err = config.NormalizeTrustedHost(opts.Hostname)
	if err != nil {
		return cmdutil.NewUsageError(err.Error())
	}

	cs := opts.IO.ColorScheme()

	// Require confirmation before logout
	if err := cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: opts.Hostname,
		Prompt:   fmt.Sprintf("! This will log out of %s\nType the hostname to confirm: ", opts.Hostname),
	}); err != nil {
		return err
	}

	if err := authCfg.Logout(opts.Hostname, opts.Username); err != nil {
		return fmt.Errorf("failed to clear stored authentication: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Cleared stored authentication for %s\n", cs.Green("✓"), opts.Hostname)
	if opts.Hostname == "gitcode.com" {
		envVars := envtoken.SetEnvVars(os.LookupEnv)
		if len(envVars) > 0 {
			fmt.Fprint(opts.IO.ErrOut, "\n"+envtoken.ShortWarning(envtoken.Options{
				EnvVars: envVars,
				GOOS:    runtime.GOOS,
			}))
		}
	}

	return nil
}
