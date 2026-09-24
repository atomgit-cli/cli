// Package update implements explicit installation-channel updates.
package update

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/installupdate"
)

type result struct {
	Status       string `json:"status"`
	Distribution string `json:"distribution"`
	Current      string `json:"current"`
	Latest       string `json:"latest"`
	Message      string `json:"message"`
}

// NewCmdUpdate creates the update command.
func NewCmdUpdate(_ *cmdutil.Factory) *cobra.Command {
	var checkOnly bool
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for or apply an installation-channel update",
		Long: `Update GitCode CLI through the channel that owns the current installation.

Global npm wrappers update the installed npm coordinate (atomgit-cli,
@atomgit-cli/cli, or @gitcode-cli/cli) directly. npm-bootstrap installs
schedule an atomic replacement after the current process exits. Other package
managers remain user-controlled and are never invoked or removed implicitly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := installupdate.LoadBootstrapManifest()
			if err == nil {
				if checkOnly {
					return installupdate.RunCheck(manifest, jsonOutput, cmd.OutOrStdout(), cmd.ErrOrStderr())
				}
				if err := installupdate.StartDetached(manifest, true); err != nil {
					return err
				}
				return writeResult(cmd, jsonOutput, result{
					Status:       "scheduled",
					Distribution: "npm-bootstrap",
					Current:      manifest.Version,
					Message:      "Update scheduled; the new stable version will be used on the next launch.",
				})
			}

			distribution := detectDistribution()
			message := managerMessage(distribution)
			return writeResult(cmd, jsonOutput, result{Status: "manual", Distribution: distribution, Message: message})
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for an update without installing it")
	cmdutil.AddJSONFlag(cmd, &jsonOutput)
	return cmd
}

// detectDistribution resolves the installation channel from the process
// environment and the running binary's location so channel-specific upgrade
// guidance (Homebrew, deb/rpm, npm) reaches users who do not run through an
// npm wrapper.
func detectDistribution() string {
	binary := os.Getenv("GITCODE_CLI_BINARY")
	if binary == "" {
		if resolved, err := os.Executable(); err == nil {
			binary = resolved
		}
	}
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[strings.ToUpper(key)] = value
		}
	}
	return installupdate.DetectDistribution(env, binary)
}

func managerMessage(distribution string) string {
	switch distribution {
	case "pypi":
		return "This installation is managed by Python; upgrade it explicitly with pipx or pip in its environment."
	case "homebrew":
		return "This installation is managed by Homebrew; run brew upgrade gc."
	case "deb", "rpm", "system-package":
		return "This installation is managed by the operating-system package manager; use apt, dnf, or rpm explicitly."
	case "npm":
		return "Run update through the npm gitcode wrapper so it can safely replace its bundled binary."
	default:
		return "Download and verify a newer release archive, or rebuild from the desired source tag."
	}
}

func writeResult(cmd *cobra.Command, jsonOutput bool, value result) error {
	if jsonOutput {
		return cmdutil.WriteJSON(cmd.OutOrStdout(), value)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), value.Message)
	return err
}
