// Package root implements the root command for gc.
package root

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	actionscmd "gitcode.com/gitcode-cli/cli/pkg/cmd/actions"
	apicmd "gitcode.com/gitcode-cli/cli/pkg/cmd/api"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/auth"
	browsecmd "gitcode.com/gitcode-cli/cli/pkg/cmd/browse"
	commitcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/commit"
	configcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/config"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/discussions"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/doctor"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/help"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/issue"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/label"
	larkcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/lark"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/milestone"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/pr"
	precommitcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/precommit"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/release"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/repo"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/schema"
	searchcmd "gitcode.com/gitcode-cli/cli/pkg/cmd/search"
	sshkeycmd "gitcode.com/gitcode-cli/cli/pkg/cmd/sshkey"
	updatecmd "gitcode.com/gitcode-cli/cli/pkg/cmd/update"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/user"
	"gitcode.com/gitcode-cli/cli/pkg/cmd/version"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/installupdate"
)

const commandNameEnv = "GITCODE_CLI_COMMAND_NAME"

// Execute runs the root command.
func Execute(ver, commit, date string) error {
	f := cmdutil.NewFactory()
	rootCmd := NewRootCmd(ver, commit, date, f)
	err := rootCmd.Execute()
	// cobra skips PersistentPostRun when RunE returns an error, so a failing
	// command would skip the update lifecycle entirely (pending summaries,
	// first-run notice, due background check). The npm wrapper runs its
	// lifecycle on every exit code; run ours here to match. AfterCommand
	// exits early without an adjacent bootstrap manifest, so this is a
	// no-op for non-bootstrap channels.
	if err != nil {
		runPostCommandLifecycle(rootCmd, f)
	}
	return err
}

// runPostCommandLifecycle mirrors the PersistentPostRun hook for the error
// path (flag parsing has already happened, so the flag reads are safe).
func runPostCommandLifecycle(cmd *cobra.Command, f *cmdutil.Factory) {
	noUpdate, _ := cmd.Flags().GetBool("no-update-check")
	noInteractive, _ := cmd.Flags().GetBool("no-interactive")
	cfg, _ := f.Config()
	installupdate.AfterCommand(cfg, f.IOStreams.ErrOut, noUpdate, noInteractive)
}

// NewRootCmd creates the root command.
func NewRootCmd(ver, commit, date string, f *cmdutil.Factory) *cobra.Command {
	commandName := resolveCommandName()

	cmd := &cobra.Command{
		Use:           commandName,
		Short:         "GitCode CLI - Command line tool for GitCode",
		Long:          rootLong(commandName),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if noInteractive, _ := cmd.Flags().GetBool("no-interactive"); noInteractive {
				f.IOStreams.SetNoInteractive(true)
			}
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			noUpdate, _ := cmd.Flags().GetBool("no-update-check")
			noInteractive, _ := cmd.Flags().GetBool("no-interactive")
			cfg, _ := f.Config()
			installupdate.AfterCommand(cfg, f.IOStreams.ErrOut, noUpdate, noInteractive)
		},
	}

	cmd.PersistentFlags().Bool("no-interactive", false, "Disable interactive prompts; forces --yes or fails on confirmations (for agents/scripts)")
	cmd.PersistentFlags().Bool("no-update-check", false, "Disable the npm update check for this invocation")

	// Add subcommands.
	cmd.AddCommand(version.NewCmdVersion(ver, commit, date, commandName))
	cmd.AddCommand(actionscmd.NewCmdActions(f))
	cmd.AddCommand(apicmd.NewCmdAPI(f, nil))
	cmd.AddCommand(auth.NewCmdAuth(f))
	cmd.AddCommand(browsecmd.NewCmdBrowse(f, nil))
	cmd.AddCommand(repo.NewCmdRepo(f))
	cmd.AddCommand(issue.NewCmdIssue(f))
	cmd.AddCommand(pr.NewCmdPR(f))
	cmd.AddCommand(commitcmd.NewCmdCommit(f))
	cmd.AddCommand(configcmd.NewCmdConfig(f))
	cmd.AddCommand(discussions.NewCmdDiscussions(f))
	cmd.AddCommand(doctor.NewCmdDoctor(f, ver, commit, date))
	cmd.AddCommand(precommitcmd.NewCmdPrecommit(f))
	cmd.AddCommand(label.NewCmdLabel(f))
	cmd.AddCommand(milestone.NewCmdMilestone(f))
	cmd.AddCommand(release.NewCmdRelease(f))
	cmd.AddCommand(schema.NewCmdSchema(cmd))
	cmd.AddCommand(searchcmd.NewCmdSearch(f))
	cmd.AddCommand(sshkeycmd.NewCmdSSHKey(f))
	cmd.AddCommand(user.NewCmdUser(f))
	cmd.AddCommand(updatecmd.NewCmdUpdate(f))
	cmd.AddCommand(larkcmd.NewCmdLark(f))

	rewriteExamples(cmd, commandName)

	// Set custom help command with search and discovery features.
	cmd.SetHelpCommand(help.NewCmdHelp(cmd))

	return cmd
}

func resolveCommandName() string {
	if name := normalizedCommandName(os.Getenv(commandNameEnv)); name != "" {
		return name
	}

	executable := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
	if name := normalizedCommandName(executable); name != "" {
		return name
	}

	if runtime.GOOS == "windows" {
		return "gitcode"
	}
	return "gc"
}

func normalizedCommandName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "gc":
		return "gc"
	case "gitcode":
		return "gitcode"
	default:
		return ""
	}
}

func rootLong(commandName string) string {
	long := fmt.Sprintf(`%s is a command line tool for GitCode.

It provides convenient access to GitCode features including:
  - Authentication management (auth login, auth status)
  - Repository operations (repo clone, repo create)
  - Issue management (issue create, issue list)
  - Pull Request management (pr create, pr review)`, commandName)

	if runtime.GOOS == "windows" && commandName == "gc" {
		long += `

Windows PowerShell defines "gc" as an alias for Get-Content.
If "gc" is intercepted by PowerShell, use "gitcode", "gc.exe", or
"python -m gc_cli" instead.`
	}

	return long
}

func rewriteExamples(cmd *cobra.Command, rootName string) {
	cmd.Example = rewriteCommandReferences(cmd.Example, rootName)
	for _, child := range cmd.Commands() {
		rewriteExamples(child, rootName)
	}
}

func rewriteCommandReferences(text, rootName string) string {
	if text == "" || rootName == "" || rootName == "gc" {
		return text
	}

	replacer := strings.NewReplacer(
		"$ gc ", "$ "+rootName+" ",
		"| gc ", "| "+rootName+" ",
		"Use \"gc ", "Use \""+rootName+" ",
	)
	return replacer.Replace(text)
}
