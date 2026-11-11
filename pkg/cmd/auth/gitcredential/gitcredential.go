// Package gitcredential implements the auth git-credential command.
package gitcredential

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// GitCredentialOptions holds the configuration for the auth git-credential command.
type GitCredentialOptions struct {
	IO     *iostreams.IOStreams
	Config func() (config.Config, error)

	// Operation is the git credential helper operation passed by git
	// ("get", "store" or "erase"). Empty means "get".
	Operation string
}

// NewCmdGitCredential creates the auth git-credential command. It speaks the
// git credential helper protocol (https://git-scm.com/docs/gitcredentials)
// and is normally invoked by git itself via the helper entry written by
// gc auth setup-git.
func NewCmdGitCredential(f *cmdutil.Factory, runF func(*GitCredentialOptions) error) *cobra.Command {
	opts := &GitCredentialOptions{
		IO:     f.IOStreams,
		Config: f.Config,
	}

	cmd := &cobra.Command{
		Use:   "git-credential",
		Short: "Provide stored GitCode credentials to git (helper protocol)",
		Long: heredoc.Doc(`
			Implement the git credential helper protocol so that git operations
			over HTTPS against GitCode use the credentials of the account logged
			in via gc auth login.

			git invokes this command with an operation argument (get, store or
			erase) and a key=value request on stdin. Only "get" is meaningful:
			when the request host matches the logged-in account, the username
			and the access token are written to stdout in the credential
			protocol format. store and erase are accepted as no-ops because gc
			does not cache credentials beyond the login state.

			When no credentials match, the command exits silently with a
			non-zero status so git can try the next credential helper. Use
			gc auth status to inspect the login state.

			Credentials are served only from the account stored in the active
			configuration directory: GC_CONFIG_DIR isolates the configuration
			(useful for multiple accounts on one machine), while the
			GC_TOKEN/GITCODE_TOKEN environment overrides intentionally do not
			apply, so the helper's answer is fully determined by the login
			state of the selected configuration directory.
		`),
		Example: heredoc.Doc(`
			# Inspect what git would receive for the logged-in account
			$ printf 'protocol=https\nhost=gitcode.com\n\n' | gc auth git-credential

			# Configure git to use gc as a credential helper
			$ gc auth setup-git
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Operation = args[0]
			}
			if runF != nil {
				return runF(opts)
			}
			return gitCredentialRun(opts)
		},
	}

	return cmd
}

func gitCredentialRun(opts *GitCredentialOptions) error {
	switch strings.ToLower(strings.TrimSpace(opts.Operation)) {
	case "", "get":
		return getCredential(opts)
	case "store", "erase":
		// git calls store/erase via "git credential approve/reject" after
		// authentication. gc does not cache credentials beyond the login
		// state, so these operations are no-ops; stdin is drained so the
		// writer can finish without a broken pipe.
		drain(opts.IO.In)
		return nil
	default:
		return cmdutil.NewUsageError(fmt.Sprintf("unsupported credential operation %q (supported: get, store, erase)", opts.Operation))
	}
}

// credentialRequest is the parsed key=value request from git.
type credentialRequest struct {
	protocol string
	host     string
	username string
}

// getCredential reads the request, matches it against the stored login
// state, and writes the credential protocol response to stdout.
func getCredential(opts *GitCredentialOptions) error {
	req, err := parseRequest(opts.IO.In)
	if err != nil {
		return err
	}

	if req.host == "" {
		return silentNoCredential("")
	}

	cfg, err := opts.Config()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	authCfg := cfg.Authentication()

	// The helper serves the stored login state only (StoredToken, not
	// ActiveToken): environment token overrides would break the
	// GC_CONFIG_DIR isolation contract documented in the command help.
	username, err := authCfg.ActiveUser(req.host)
	if err != nil || username == "" {
		return silentNoCredential(req.host)
	}
	token, _ := authCfg.StoredToken(req.host)
	if token == "" {
		return silentNoCredential(req.host)
	}
	if req.username != "" && req.username != username {
		return silentNoCredential(req.host)
	}

	// Credential values are only ever written to stdout in the protocol
	// format; never to stderr, logs, or error messages.
	fmt.Fprintf(opts.IO.Out, "username=%s\n", username)
	fmt.Fprintf(opts.IO.Out, "password=%s\n", token)
	return nil
}

// silentNoCredential reports "no credentials for this request" the way the
// credential helper protocol expects: no output at all and a non-zero exit
// code, so git continues with the next helper. The message never contains
// credential values; it only surfaces in logs and tests.
func silentNoCredential(host string) error {
	if host == "" {
		return cmdutil.NewSilentError(cmdutil.ExitError, "no host in credential request")
	}
	return cmdutil.NewSilentError(cmdutil.ExitError, "no stored credential for "+host)
}

// parseRequest reads key=value lines until a blank line or EOF, tolerating
// unknown keys. A url= key carries protocol and host when git does not split
// them into separate keys.
func parseRequest(r io.Reader) (*credentialRequest, error) {
	req := &credentialRequest{}
	var rawURL string
	if r != nil {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				break
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch key {
			case "protocol":
				req.protocol = value
			case "host":
				req.host = value
			case "username":
				req.username = value
			case "url":
				rawURL = value
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read credential request: %w", err)
		}
	}

	if req.host == "" && rawURL != "" {
		if u, err := url.Parse(rawURL); err == nil {
			if req.protocol == "" {
				req.protocol = u.Scheme
			}
			req.host = u.Hostname()
		}
	}

	req.host = strings.ToLower(strings.TrimSpace(req.host))
	req.username = strings.TrimSpace(req.username)
	return req, nil
}

// drain consumes and discards the request stream.
func drain(r io.Reader) {
	if r == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r)
}
