// Package view implements the actions plugin view command.
package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// ViewOptions configures the actions plugin view command.
type ViewOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	PluginName string

	JSON  bool
	Files string
}

// NewCmdView creates the actions plugin view command.
func NewCmdView(f *cmdutil.Factory, runF func(*ViewOptions) error) *cobra.Command {
	opts := &ViewOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "view <plugin-name>",
		Aliases: []string{"show"},
		Short:   "View an Actions plugin detail and README",
		Long: heredoc.Doc(`
			View the metadata, versions and Markdown README of a specific
			official GitCode Actions plugin.

			By default the README of each version in vision_content is printed.
			Use --json to output the full API response with all fields preserved.
		`),
		Example: heredoc.Doc(`
			# View a plugin's README
			$ gc actions plugin view checkout -R owner/repo

			# Use 'show' alias
			$ gc actions plugin show checkout -R owner/repo

			# Output as JSON
			$ gc actions plugin view checkout -R owner/repo --json

			# Write output to a file (avoids shell encoding issues)
			$ gc actions plugin view checkout -R owner/repo --json --files detail.json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			opts.PluginName = strings.TrimSpace(args[0])
			if opts.PluginName == "" {
				return cmdutil.NewUsageError("plugin name cannot be empty")
			}
			if runF != nil {
				return runF(opts)
			}
			return viewRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo, defaults to current repo or gitcode-cli/cli)")
	cmd.Flags().StringVar(&opts.Files, "files", "", "Write output to file (UTF-8 bytes, avoids shell encoding issues)")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func viewRun(opts *ViewOptions) error {
	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
	if err != nil {
		return err
	}

	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		repository = "gitcode-cli/cli"
	}
	owner, repo, err := cmdutil.ParseRepo(repository)
	if err != nil {
		return err
	}

	raw, err := api.ViewActionsPlugin(client, owner+"/"+repo, opts.PluginName)
	if err != nil {
		return fmt.Errorf("failed to view actions plugin: %w", err)
	}

	var buf bytes.Buffer
	if opts.JSON {
		if err := cmdutil.WriteJSON(&buf, json.RawMessage(raw)); err != nil {
			return err
		}
	} else {
		var detail api.ActionsPluginDetail
		if err := json.Unmarshal(raw, &detail); err != nil {
			return fmt.Errorf("failed to parse plugin detail: %w", err)
		}
		printPluginDetail(&buf, opts.IO, &detail, opts.PluginName)
	}

	return writeResult(opts, buf.Bytes())
}

func writeResult(opts *ViewOptions, content []byte) error {
	if opts.Files != "" {
		if err := os.WriteFile(opts.Files, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", opts.Files, err)
		}
		fmt.Fprintf(opts.IO.ErrOut, "Wrote %d bytes to %s\n", len(content), opts.Files)
		return nil
	}
	_, err := opts.IO.Out.Write(content)
	return err
}

func printPluginDetail(buf *bytes.Buffer, io *iostreams.IOStreams, d *api.ActionsPluginDetail, requestedName string) {
	cs := io.ColorScheme()
	name := d.Name
	if name == "" {
		name = requestedName
	}
	fmt.Fprintf(buf, "%s\t%s\n", cs.Bold("Plugin:"), name)
	fmt.Fprintf(buf, "%s\t%s\n", "Display Name:", orValue(d.DisplayName))
	fmt.Fprintf(buf, "%s\t%s\n", "Description:", orValue(d.Description))
	fmt.Fprintf(buf, "%s\t%d\n", "Versions:", len(d.VisionContent))
	fmt.Fprintln(buf)

	for _, v := range d.VisionContent {
		label := v.Version
		if label == "" {
			label = "unknown"
		}
		fmt.Fprintf(buf, "%s\n", cs.Bold(fmt.Sprintf("== Version %s ==", label)))
		readme := v.Readme
		if readme == "" {
			fmt.Fprintf(buf, "(no README available)\n\n")
			continue
		}
		fmt.Fprintf(buf, "%s\n\n", readme)
	}
}

func orValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}
