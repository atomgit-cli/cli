// Package config implements user-visible CLI configuration commands.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

const defaultHost = "gitcode.com"

type configResult struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type configListItem struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// NewCmdConfig creates the config command.
func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <command>",
		Short: "Read and write GitCode CLI configuration",
		Long: heredoc.Doc(`
			Read and write GitCode CLI configuration.

			Configuration lives in the config directory (default
			~/.config/gc). Set GC_CONFIG_DIR to read and write a different
			directory, e.g. to keep a per-account configuration.
		`),
	}
	cmd.AddCommand(newCmdGet(f))
	cmd.AddCommand(newCmdSet(f))
	cmd.AddCommand(newCmdList(f))
	cmd.AddCommand(newCmdClearCache(f))
	return cmd
}

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Read a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(strings.TrimSpace(args[0]))
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			value, err := cfg.Get(defaultHost, key)
			if err != nil {
				return err
			}
			if key == "update.mode" && value == "" {
				value = "notify"
			}
			result := configResult{Key: key, Value: value}
			if jsonOutput {
				return cmdutil.WriteJSON(cmd.OutOrStdout(), result)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), value)
			return err
		},
	}
	cmdutil.AddJSONFlag(cmd, &jsonOutput)
	return cmd
}

func newCmdSet(f *cmdutil.Factory) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Store a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(strings.TrimSpace(args[0]))
			value := strings.TrimSpace(args[1])
			if key == "update.mode" {
				value = strings.ToLower(value)
			}
			if err := validateValue(key, value); err != nil {
				return cmdutil.NewUsageError(err.Error())
			}
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			if err := cfg.Set(defaultHost, key, value); err != nil {
				return err
			}
			result := configResult{Key: key, Value: value}
			if jsonOutput {
				return cmdutil.WriteJSON(cmd.OutOrStdout(), result)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Set %s=%s\n", key, value)
			return err
		},
	}
	cmdutil.AddJSONFlag(cmd, &jsonOutput)
	return cmd
}

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long: heredoc.Doc(`
			List all configuration keys, their current values, and the
			source of each value (environment, config, or default).

			Use --json for machine-readable output.
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			items, err := listConfig(cfg)
			if err != nil {
				return err
			}
			if jsonOutput {
				return cmdutil.WriteJSON(cmd.OutOrStdout(), items)
			}
			printConfigList(cmd.OutOrStdout(), items)
			return nil
		},
	}
	cmdutil.AddJSONFlag(cmd, &jsonOutput)
	return cmd
}

func newCmdClearCache(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-cache",
		Short: "Clear CLI cache",
		Long: heredoc.Doc(`
			Clear files from the CLI cache directory (~/.config/gc/cache/)
			if present. This does not affect authentication or configuration
			files (auth.json, config.json, lark.json).
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clearCache(f.IOStreams)
		},
	}
	return cmd
}

func listConfig(cfg config.Config) ([]configListItem, error) {
	keys := []string{"browser", "editor", "pager", "update.mode", "default_repo"}
	sort.Strings(keys)
	items := make([]configListItem, 0, len(keys))
	for _, key := range keys {
		value, source := resolveConfigValue(cfg, key)
		items = append(items, configListItem{Key: key, Value: value, Source: source})
	}
	return items, nil
}

func resolveConfigValue(cfg config.Config, key string) (string, string) {
	envKey := "GC_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	if envVal := os.Getenv(envKey); envVal != "" {
		return envVal, "environment"
	}
	value, err := cfg.Get(defaultHost, key)
	if err != nil {
		return "", "default"
	}
	if value != "" {
		return value, "config"
	}
	if key == "update.mode" {
		return "notify", "default"
	}
	if key == "editor" {
		return "vim", "default"
	}
	if key == "pager" {
		return "less", "default"
	}
	return "", "default"
}

func printConfigList(out interface{ Write([]byte) (int, error) }, items []configListItem) {
	for _, item := range items {
		fmt.Fprintf(out, "%s=%s", item.Key, item.Value)
		if item.Source != "config" {
			fmt.Fprintf(out, "  (%s)", item.Source)
		}
		fmt.Fprintln(out)
	}
}

func clearCache(io *iostreams.IOStreams) error {
	cacheDir := filepath.Join(configDir(), "cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(io.Out, "No cache to clear.")
			return nil
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}
	cleared := 0
	for _, entry := range entries {
		path := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(io.ErrOut, "Warning: failed to remove %s: %v\n", entry.Name(), err)
			continue
		}
		cleared++
	}
	fmt.Fprintf(io.Out, "Cleared %d cache entr%s.\n", cleared, pluralEntry(cleared))
	return nil
}

func pluralEntry(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func configDir() string {
	if dir := os.Getenv("GC_CONFIG_DIR"); dir != "" {
		return dir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".config", "gc")
	}
	return filepath.Join(homeDir, ".config", "gc")
}

func validateValue(key, value string) error {
	if key != "update.mode" {
		return nil
	}
	switch value {
	case "auto", "notify", "off":
		return nil
	default:
		return errors.New("update.mode must be auto, notify, or off")
	}
}
