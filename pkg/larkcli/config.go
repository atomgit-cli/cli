package larkcli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gitcode.com/gitcode-cli/cli/pkg/config"
)

// larkConfig is the persisted lark integration state stored in lark.json.
// It is intentionally separate from pkg/config's host-scoped config.json
// because a Feishu chat id is not tied to a GitCode host.
type larkConfig struct {
	DefaultChatID string `json:"default_chat_id,omitempty"`
}

// errNoConfigDir is returned when the config directory cannot be resolved.
// Resolution never falls back to a cwd-relative path (issue 675's twin fix
// in pkg/config).
var errNoConfigDir = errors.New("cannot resolve the config directory: HOME is not set; set HOME or GC_CONFIG_DIR")

// ConfigPath returns the lark config file path (~/.config/gc/lark.json or
// $GC_CONFIG_DIR/lark.json). It returns an empty string when no config
// location can be resolved (no GC_CONFIG_DIR and no home directory); callers
// must treat that as "no configuration" and must never create files for it.
func ConfigPath() string {
	dir, err := configDirChecked()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "lark.json")
}

// DefaultChatID resolves the default Feishu chat id from, in priority order:
// the GC_LARK_DEFAULT_CHAT_ID environment variable, then the persisted
// lark.json. Returns an empty string when nothing is configured.
func DefaultChatID() string {
	if v := strings.TrimSpace(os.Getenv(EnvDefaultChat)); v != "" {
		return v
	}
	cfg, err := readConfig()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.DefaultChatID)
}

// SaveDefaultChat persists the default chat id to lark.json.
func SaveDefaultChat(chatID string) error {
	chatID = strings.TrimSpace(chatID)
	cfg, _ := readConfig()
	cfg.DefaultChatID = chatID
	return writeConfig(cfg)
}

// ClearDefaultChat removes the persisted default chat id.
func ClearDefaultChat() error {
	cfg, _ := readConfig()
	cfg.DefaultChatID = ""
	return writeConfig(cfg)
}

// configDirChecked returns the config directory: GC_CONFIG_DIR first, then
// the home directory. It fails closed when neither is available: the
// previous "~" fallback was a relative path (Go does not expand a literal
// tilde), so writes created the tree in whatever directory the command ran
// in (issue 675). Mirrors pkg/config's configDirChecked.
func configDirChecked() (string, error) {
	if dir := os.Getenv("GC_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errNoConfigDir
	}
	return filepath.Join(home, ".config", "gc"), nil
}

func readConfig() (larkConfig, error) {
	path := ConfigPath()
	if path == "" {
		// No resolvable config location: reads see no configuration and must
		// not trust a cwd-relative path or legacy "~" trees (issue 675).
		return larkConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return larkConfig{}, nil
		}
		return larkConfig{}, err
	}
	var cfg larkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return larkConfig{}, err
	}
	return cfg, nil
}

func writeConfig(cfg larkConfig) error {
	path := ConfigPath()
	if path == "" {
		// No resolvable config location: fail closed with guidance instead of
		// creating a "~/.config/gc" tree in the current directory (issue 675).
		return errNoConfigDir
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return config.SecureWriteFile(path, data, 0o600)
}
