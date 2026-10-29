// Package version implements the version command
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

// VersionInfo represents the JSON output for version command
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Built   string `json:"built,omitempty"`
	URL     string `json:"url,omitempty"`
}

// NewCmdVersion creates the version command.
func NewCmdVersion(ver, commit, date string, commandName ...string) *cobra.Command {
	var jsonOutput bool
	displayName := "gc"
	if len(commandName) > 0 && commandName[0] != "" {
		displayName = commandName[0]
	}

	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Print %s version", displayName),
		Long:  fmt.Sprintf("Print the version information of %s.", displayName),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if jsonOutput {
				info := VersionInfo{
					Version: ver,
					Commit:  commit,
					Built:   date,
					URL:     "https://gitcode.com/atomgit-cli/cli",
				}
				return cmdutil.WriteJSON(out, info)
			}

			fmt.Fprintf(out, "%s version %s\n", displayName, ver)
			fmt.Fprintf(out, "  commit: %s\n", commit)
			fmt.Fprintf(out, "  built:  %s\n", date)
			fmt.Fprintln(out, "https://gitcode.com/atomgit-cli/cli")
			return nil
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOutput)

	return cmd
}
