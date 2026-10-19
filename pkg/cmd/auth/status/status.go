// Package status implements the auth status command
package status

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/envtoken"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type StatusOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	Config     func() (config.Config, error)

	// Flags
	Hostname    string
	HostnameSet bool
	ShowToken   bool
	JSON        bool
}

// AuthStatus represents the JSON output structure for auth status
type AuthStatus struct {
	Hostname    string `json:"hostname"`
	LoggedIn    bool   `json:"logged_in"`
	Username    string `json:"username,omitempty"`
	TokenSource string `json:"token_source,omitempty"`
	TokenValid  bool   `json:"token_valid,omitempty"`
	GitProtocol string `json:"git_protocol,omitempty"`
	Token       string `json:"token,omitempty"`
}

// NewCmdStatus creates the status command
func NewCmdStatus(f *cmdutil.Factory, runF func(*StatusOptions) error) *cobra.Command {
	opts := &StatusOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config:     f.Config,
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "View authentication status",
		Long: heredoc.Doc(`
			View information about your authentication status.

			When no hostname is specified, checks for token from:
			1. GC_TOKEN environment variable
			2. GITCODE_TOKEN environment variable
			3. Stored credentials from local config (~/.config/gc/auth.json)
		`),
		Example: heredoc.Doc(`
			# Check authentication status
			$ gc auth status
			gitcode.com
			  ✓ Logged in as username (GC_TOKEN)
			  ✓ Git operations protocol: https

			# Output as JSON
			$ gc auth status --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.HostnameSet = cmd.Flags().Changed("hostname")
			if runF != nil {
				return runF(opts)
			}
			return statusRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Hostname, "hostname", "H", "", "Check a specific hostname")
	cmd.Flags().BoolVar(&opts.ShowToken, "show-token", false, "Display the full auth token")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Output as JSON")

	return cmd
}

func statusRun(opts *StatusOptions) error {
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
		return err
	}

	cs := opts.IO.ColorScheme()

	token, tokenSource := authCfg.ActiveToken(opts.Hostname)
	if opts.HostnameSet || opts.Hostname != "gitcode.com" {
		token, tokenSource = authCfg.StoredToken(opts.Hostname)
	}

	// Build status for JSON output
	status := AuthStatus{
		Hostname: opts.Hostname,
	}

	if token == "" {
		status.LoggedIn = false

		if opts.JSON {
			if err := cmdutil.WriteJSON(opts.IO.Out, status); err != nil {
				return err
			}
			return cmdutil.NewAuthError(fmt.Sprintf("not logged in to %s", opts.Hostname))
		}

		fmt.Fprintf(opts.IO.ErrOut, "%s\n", opts.Hostname)
		fmt.Fprintf(opts.IO.ErrOut, "  %s Not logged in\n", cs.Red("✗"))
		fmt.Fprintf(opts.IO.ErrOut, "\n")
		fmt.Fprintf(opts.IO.ErrOut, "To authenticate, run: gc auth login\n")
		return cmdutil.NewAuthError(fmt.Sprintf("not logged in to %s", opts.Hostname))
	}

	status.LoggedIn = true
	status.TokenSource = tokenSource

	// Verify token
	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	user, err := api.VerifyToken(httpClient, opts.Hostname, token)
	if err != nil {
		status.TokenValid = false

		var apiErr *api.APIError
		isAuthError := false
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case 401, 403:
				isAuthError = true
			default:
				isAuthError = false
			}
		}

		if opts.JSON {
			if err := cmdutil.WriteJSON(opts.IO.Out, status); err != nil {
				return err
			}
			if isAuthError {
				return cmdutil.NewAuthError(fmt.Sprintf("token for %s is invalid or expired", opts.Hostname))
			}
			return fmt.Errorf("failed to verify token for %s: %w", opts.Hostname, err)
		}

		fmt.Fprintf(opts.IO.ErrOut, "%s\n", opts.Hostname)
		if isAuthError {
			fmt.Fprintf(opts.IO.ErrOut, "  %s Token is invalid or expired\n", cs.Red("✗"))
			fmt.Fprintf(opts.IO.ErrOut, "\n")
			// A token coming from the environment keeps overriding any local
			// login, so suggesting a plain re-login would be a dead end. Point
			// at the copy-paste remediation plans instead (issue 613).
			if tokenSource == "GC_TOKEN" || tokenSource == "GITCODE_TOKEN" {
				fmt.Fprint(opts.IO.ErrOut, envtoken.Warning(envtoken.Options{
					EnvVars:    envtoken.SetEnvVars(os.LookupEnv),
					Shell:      os.Getenv("SHELL"),
					GOOS:       runtime.GOOS,
					LoginSaved: false,
				}))
			} else {
				fmt.Fprintf(opts.IO.ErrOut, "To re-authenticate, run: gc auth login\n")
			}
			return cmdutil.NewAuthError(fmt.Sprintf("token for %s is invalid or expired", opts.Hostname))
		}
		fmt.Fprintf(opts.IO.ErrOut, "  %s Token verification failed: %v\n", cs.Red("✗"), err)
		return fmt.Errorf("failed to verify token for %s: %w", opts.Hostname, err)
	}

	status.TokenValid = true
	status.Username = user.Login
	status.GitProtocol = cfg.GitProtocol(opts.Hostname).Value

	if opts.ShowToken {
		if err := cmdutil.ConfirmTokenDisclosure(opts.IO, opts.Hostname); err != nil {
			return err
		}
		status.Token = token
	}

	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, status)
	}

	// Display logged in status
	fmt.Fprintf(opts.IO.Out, "%s\n", opts.Hostname)
	fmt.Fprintf(opts.IO.Out, "  %s Logged in as %s (%s)\n", cs.Green("✓"), user.Login, tokenSource)
	fmt.Fprintf(opts.IO.Out, "  %s Git operations protocol: %s\n", cs.Green("✓"), cfg.GitProtocol(opts.Hostname).Value)

	if opts.ShowToken {
		fmt.Fprintf(opts.IO.ErrOut, "%s Warning: displaying authentication token. Do not share this output.\n", cs.Yellow("!"))
		fmt.Fprintf(opts.IO.Out, "  %s Token: %s\n", cs.Green("✓"), token)
	}

	return nil
}
