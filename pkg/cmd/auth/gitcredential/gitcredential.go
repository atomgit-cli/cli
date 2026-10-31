// Package gitcredential implements Git's credential helper protocol.
package gitcredential

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
)

const maxRequestBytes = 64 * 1024

// NewCmdGitCredential creates the command used by auth setup-git.
func NewCmdGitCredential(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:           "git-credential <get|store|erase>",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Serve Git credential requests using the current CLI login",
		Long: "Read the Git credential protocol from stdin. Only HTTPS requests receive credentials. " +
			"Store and erase are no-ops; manage the CLI login with auth login/logout.",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"get", "store", "erase"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "get" {
				return nil
			}
			return credentialRun(f)
		},
	}
}

func credentialRun(f *cmdutil.Factory) error {
	fields, err := readRequest(f.IOStreams.In)
	if err != nil {
		return err
	}
	host := fields["host"]
	normalized, err := config.NormalizeTrustedHost(host)
	if fields["protocol"] != "https" || host == "" || err != nil || normalized != strings.ToLower(host) {
		return nil
	}
	cfg, err := f.Config()
	if err != nil {
		return errors.New("failed to read credential configuration")
	}
	auth := cfg.Authentication()
	token, _ := auth.StoredToken(normalized)
	if normalized == "gitcode.com" {
		token, _ = auth.ActiveToken(normalized)
	}
	if token == "" {
		return nil
	}
	username, err := auth.ActiveUser(normalized)
	if err != nil {
		return errors.New("failed to read credential user")
	}
	if username == "" {
		username = "oauth2"
	}
	if strings.ContainsAny(username+token, "\r\n\x00") {
		return errors.New("invalid credential value")
	}
	_, err = fmt.Fprintf(f.IOStreams.Out, "username=%s\npassword=%s\n\n", username, token)
	if err != nil {
		return errors.New("failed to write credential response")
	}
	return nil
}

func readRequest(input io.Reader) (map[string]string, error) {
	fields := make(map[string]string)
	scanner := bufio.NewScanner(input)
	total := 0
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		total += advance
		if total > maxRequestBytes {
			return 0, nil, errors.New("credential request too large")
		}
		return advance, token, err
	})
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || strings.ContainsRune(line, '\x00') {
			return nil, errors.New("invalid credential request")
		}
		// Ignore extensions (including repeated wwwauth[] fields).
		if key != "host" && key != "protocol" {
			continue
		}
		if _, exists := fields[key]; exists {
			return nil, errors.New("duplicate credential field")
		}
		fields[key] = value
	}
	if scanner.Err() != nil {
		return nil, errors.New("failed to read credential request")
	}
	return fields, nil
}
