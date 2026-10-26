// Package login implements the auth login command
package login

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/browser"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth/envtoken"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type LoginOptions struct {
	IO          *iostreams.IOStreams
	HttpClient  func() (*http.Client, error)
	Config      func() (config.Config, error)
	OpenBrowser func(string) error

	// Flags
	Hostname    string
	WithToken   bool
	GitProtocol string
	Web         bool

	// Internal state populated from stdin or interactive input.
	Token string

	// inputReader caches a single buffered reader over IO.In so the token
	// prompt and the save-confirmation gate share one buffer; separate
	// readers would lose pasted-ahead input between them.
	inputReader *bufio.Reader
}

// NewCmdLogin creates the login command
func NewCmdLogin(f *cmdutil.Factory, runF func(*LoginOptions) error) *cobra.Command {
	opts := &LoginOptions{
		IO:          f.IOStreams,
		HttpClient:  f.HttpClient,
		Config:      f.Config,
		OpenBrowser: browser.Open,
	}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a GitCode account",
		Long: heredoc.Doc(`
			Authenticate with a GitCode account.

			By default, gc prompts for a token in an interactive terminal.
			In non-interactive environments, use --with-token to read the token
			from standard input.

			Never place a token literal on the command line: it leaks to shell
			history and process lists, and never store a token in a plaintext
			file. Prefer interactive login, --web, or the GC_TOKEN environment
			variable (injected from your platform's secrets); when using
			--with-token, pipe from a secret manager.
		`),
		Example: heredoc.Doc(`
			# Start interactive login (recommended; use a private, unrecorded terminal)
			$ gc auth login

			# Login via browser device flow
			$ gc auth login --web

			# Login to a specific host
			$ gc auth login --hostname gitcode.com

			# Non-interactive (CI): inject the token via the GC_TOKEN environment
			# variable from your platform's secrets; gc authenticates automatically,
			# no login command needed.
			#   $ export GC_TOKEN="$GC_TOKEN_FROM_SECRETS"

			# Read a token from stdin with --with-token. Never echo/cat a token
			# literal (it leaks to shell history or disk); pipe from a secret
			# manager instead.
			$ <print-token-from-secret-manager> | gc auth login --with-token
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Web && opts.WithToken {
				return cmdutil.NewUsageError("--web and --with-token cannot be used together")
			}
			if opts.Web {
				if err := validateWebHostname(opts); err != nil {
					return err
				}
			}

			if runF != nil {
				return runF(opts)
			}

			// Handle --with-token
			if opts.WithToken {
				return loginWithToken(opts)
			}

			if !opts.IO.CanPrompt() {
				return cmdutil.NewUsageError("interactive login requires a TTY; use --with-token")
			}

			if opts.Web {
				return loginWithWeb(opts)
			}

			// Interactive login
			return loginInteractive(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Hostname, "hostname", "H", "", "The hostname of the GitCode instance to authenticate with")
	cmd.Flags().BoolVar(&opts.WithToken, "with-token", false, "Read token from standard input")
	cmd.Flags().StringVarP(&opts.GitProtocol, "git-protocol", "p", "https", "The Git protocol to use for operations (https/ssh)")
	cmdutil.SetFlagEnumOrWarn(cmd, "git-protocol", "https", "ssh")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open GitCode.com in a browser to authenticate")

	return cmd
}

func loginWithToken(opts *LoginOptions) error {
	// Read token from stdin
	reader := newInputReader(opts)
	token, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read token from stdin: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return cmdutil.NewUsageError("no token provided on stdin")
	}

	opts.Token = token
	return loginWithTokenFlag(opts)
}

func loginWithTokenFlag(opts *LoginOptions) error {
	if opts.Token == "" {
		return cmdutil.NewUsageError("no token provided")
	}

	// Set default hostname
	hostname, err := config.NormalizeTrustedHost(opts.Hostname)
	if err != nil {
		return err
	}
	opts.Hostname = hostname

	// Verify the token
	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	user, err := api.VerifyToken(httpClient, opts.Hostname, opts.Token)
	if err != nil {
		return fmt.Errorf("failed to verify token: %w", err)
	}

	// Environment tokens only override the default host, so the gate and the
	// guidance below apply exclusively to gitcode.com logins (issue 613).
	envVars := envtoken.SetEnvVars(os.LookupEnv)
	envOverrides := opts.Hostname == "gitcode.com" && len(envVars) > 0
	if envOverrides && opts.IO.CanPrompt() {
		prompt := fmt.Sprintf(
			"⚠ 环境变量 %s 已设置，优先级高于本地登录配置；本次登录在该环境不会生效\n仍要将本次登录保存到本地配置吗？(y/N): ",
			strings.Join(envVars, "、"),
		)
		keep, err := cmdutil.ConfirmYesNo(cmdutil.ConfirmYesNoOptions{
			IO:     opts.IO,
			Reader: newInputReader(opts),
			Prompt: prompt,
		})
		if err != nil {
			return err
		}
		if !keep {
			fmt.Fprintf(opts.IO.ErrOut, "已跳过保存登录配置；如需改用本地登录，请先处理环境变量后重新运行 gc auth login。\n")
			return nil
		}
	}

	return persistLogin(opts, user.Login, envVars, envOverrides)
}

// persistLogin stores the verified login and prints the success summary plus,
// when an environment variable overrides it, the remediation guidance.
func persistLogin(opts *LoginOptions, username string, envVars []string, envOverrides bool) error {
	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	_, err = cfg.Authentication().Login(opts.Hostname, username, opts.Token, opts.GitProtocol, false)
	if err != nil {
		return fmt.Errorf("failed to store authentication: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "%s Logged in as %s\n", opts.IO.ColorScheme().Green("✓"), username)
	fmt.Fprintf(opts.IO.Out, "  Host: %s\n", opts.Hostname)
	fmt.Fprintf(opts.IO.Out, "  Git protocol: %s\n", opts.GitProtocol)

	if envOverrides {
		fmt.Fprintf(opts.IO.ErrOut, "\n%s", envtoken.Warning(envtoken.Options{
			EnvVars:    envVars,
			Shell:      os.Getenv("SHELL"),
			GOOS:       runtime.GOOS,
			LoginSaved: true,
		}))
	}

	return nil
}

func loginWithWeb(opts *LoginOptions) error {
	if err := validateWebHostname(opts); err != nil {
		return err
	}

	const loginURL = "https://gitcode.com/setting/token-classic/create"
	fmt.Fprintf(opts.IO.Out, "Opening %s in your browser.\n", loginURL)
	if err := opts.OpenBrowser(loginURL); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "After generating a token in the browser, paste it below.\n")
	return loginInteractive(opts)
}

func validateWebHostname(opts *LoginOptions) error {
	hostname, err := config.NormalizeTrustedHost(opts.Hostname)
	if err != nil {
		return err
	}
	opts.Hostname = hostname

	if opts.Hostname != "gitcode.com" {
		return cmdutil.NewUsageError("--web only supports gitcode.com; use auth login --hostname <host> for custom hosts")
	}
	return nil
}

func loginInteractive(opts *LoginOptions) error {
	// Set default hostname
	hostname, err := config.NormalizeTrustedHost(opts.Hostname)
	if err != nil {
		return err
	}
	opts.Hostname = hostname

	cs := opts.IO.ColorScheme()

	fmt.Fprintf(opts.IO.Out, "\n")
	fmt.Fprintf(opts.IO.Out, "%s Authenticate with GitCode\n", cs.Bold("Tip:"))
	fmt.Fprintf(opts.IO.Out, "\n")

	// Prompt for token
	fmt.Fprintf(opts.IO.Out, "? Paste your authentication token: ")
	reader := newInputReader(opts)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return cmdutil.NewUsageError("no token provided")
	}

	opts.Token = token
	return loginWithTokenFlag(opts)
}

// newInputReader returns the shared buffered reader over the login input,
// created once per LoginOptions so consecutive prompts do not lose buffered
// input to each other.
func newInputReader(opts *LoginOptions) *bufio.Reader {
	if opts.inputReader == nil {
		if opts.IO != nil && opts.IO.In != nil {
			opts.inputReader = bufio.NewReader(opts.IO.In)
		} else {
			opts.inputReader = bufio.NewReader(os.Stdin)
		}
	}
	return opts.inputReader
}
