// Package config provides configuration management for gitcode-cli
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const configStateVersion = 1

var allowedConfigKeys = map[string]struct{}{
	"browser":      {},
	"editor":       {},
	"pager":        {},
	"update.mode":  {},
	"default_repo": {},
}

// Config interface defines configuration operations
type Config interface {
	// Get retrieves a configuration value
	Get(host, key string) (string, error)
	// Set stores a configuration value
	Set(host, key, value string) error

	// GitProtocol returns the preferred git protocol
	GitProtocol(host string) ConfigEntry
	// Editor returns the preferred editor
	Editor(host string) ConfigEntry
	// Browser returns the preferred browser
	Browser(host string) ConfigEntry
	// Pager returns the preferred pager
	Pager(host string) ConfigEntry

	// Authentication returns authentication configuration
	Authentication() AuthConfig

	// Write persists the configuration
	Write() error
}

// ConfigEntry represents a configuration value with its source
type ConfigEntry struct {
	Value  string
	Source string // "environment", "config", "default"
}

// AuthConfig interface defines authentication operations
type AuthConfig interface {
	// ActiveToken returns the active token for a host
	ActiveToken(hostname string) (string, string)
	// StoredToken returns the stored token for a host without environment overrides
	StoredToken(hostname string) (string, string)
	// HasActiveToken checks if a token exists for a host
	HasActiveToken(hostname string) bool
	// ActiveUser returns the active user for a host
	ActiveUser(hostname string) (string, error)
	// Hosts returns all configured hosts
	Hosts() []string
	// DefaultHost returns the default host
	DefaultHost() (string, string)
	// Login creates or updates authentication for a host
	Login(hostname, username, token, gitProtocol string, secureStorage bool) (bool, error)
	// Logout removes authentication for a host
	Logout(hostname, username string) error
	// SwitchUser switches the active user for a host
	SwitchUser(hostname, user string) error
}

// config implements Config interface
type config struct {
	configDir string
}

type configState struct {
	Version int                          `json:"version"`
	Hosts   map[string]map[string]string `json:"hosts,omitempty"`
}

// New creates a new Config
func New() Config {
	return &config{
		configDir: configDir(),
	}
}

// configDir returns the configuration directory
func configDir() string {
	if dir := os.Getenv("GC_CONFIG_DIR"); dir != "" {
		return dir
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "~"
	}

	return filepath.Join(homeDir, ".config", "gc")
}

// Get retrieves a configuration value
func (c *config) Get(host, key string) (string, error) {
	normalizedKey, err := normalizeConfigKey(key)
	if err != nil {
		return "", err
	}

	// Check environment variable first
	envKey := "GC_" + strings.ToUpper(strings.ReplaceAll(normalizedKey, ".", "_"))
	if val := os.Getenv(envKey); val != "" {
		return val, nil
	}

	state, err := c.readConfigState()
	if err != nil {
		return "", err
	}
	values := state.host(host)
	if values == nil {
		return "", nil
	}
	return values[normalizedKey], nil
}

// FileGet retrieves a configuration value from the config file only,
// skipping the environment-variable override that Get applies. Callers that
// need to distinguish the configured value from an (possibly invalid)
// environment override use this — e.g. the updater falls back to the
// configured update.mode when GC_UPDATE_MODE holds an invalid value, which
// Get cannot express since it returns the environment text verbatim.
func (c *config) FileGet(host, key string) (string, error) {
	normalizedKey, err := normalizeConfigKey(key)
	if err != nil {
		return "", err
	}
	state, err := c.readConfigState()
	if err != nil {
		return "", err
	}
	values := state.host(host)
	if values == nil {
		return "", nil
	}
	return values[normalizedKey], nil
}

// Set stores a configuration value
func (c *config) Set(host, key, value string) error {
	normalizedKey, err := normalizeConfigKey(key)
	if err != nil {
		return err
	}

	state, err := c.readConfigState()
	if err != nil {
		return err
	}
	values := state.ensureHost(host)
	if value == "" {
		delete(values, normalizedKey)
	} else {
		values[normalizedKey] = value
	}
	return c.writeConfigState(state)
}

// GitProtocol returns the preferred git protocol
func (c *config) GitProtocol(host string) ConfigEntry {
	if val := os.Getenv("GC_GIT_PROTOCOL"); val != "" {
		return ConfigEntry{Value: val, Source: "environment"}
	}

	state, err := c.readAuthState()
	if err == nil {
		if user := state.activeUser(host); user != nil && user.GitProtocol != "" {
			return ConfigEntry{Value: user.GitProtocol, Source: "config"}
		}
	}
	return ConfigEntry{Value: "https", Source: "default"}
}

// Editor returns the preferred editor
func (c *config) Editor(host string) ConfigEntry {
	if val := os.Getenv("GC_EDITOR"); val != "" {
		return ConfigEntry{Value: val, Source: "environment"}
	}
	if val := os.Getenv("EDITOR"); val != "" {
		return ConfigEntry{Value: val, Source: "environment"}
	}
	if val, err := c.Get(host, "editor"); err == nil && val != "" {
		return ConfigEntry{Value: val, Source: "config"}
	}
	return ConfigEntry{Value: "vim", Source: "default"}
}

// Browser returns the preferred browser
func (c *config) Browser(host string) ConfigEntry {
	if val := os.Getenv("GC_BROWSER"); val != "" {
		return ConfigEntry{Value: val, Source: "environment"}
	}
	if val, err := c.Get(host, "browser"); err == nil && val != "" {
		return ConfigEntry{Value: val, Source: "config"}
	}
	return ConfigEntry{Value: "", Source: "default"}
}

// Pager returns the preferred pager
func (c *config) Pager(host string) ConfigEntry {
	if val := os.Getenv("GC_PAGER"); val != "" {
		return ConfigEntry{Value: val, Source: "environment"}
	}
	if val := os.Getenv("PAGER"); val != "" {
		return ConfigEntry{Value: val, Source: "environment"}
	}
	if val, err := c.Get(host, "pager"); err == nil && val != "" {
		return ConfigEntry{Value: val, Source: "config"}
	}
	return ConfigEntry{Value: "less", Source: "default"}
}

// Authentication returns authentication configuration
func (c *config) Authentication() AuthConfig {
	return &authConfig{config: c}
}

// Write persists the configuration
func (c *config) Write() error {
	// Ensure config directory exists
	if err := os.MkdirAll(c.configDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(c.configDir, 0o700); err != nil {
		return err
	}
	state, err := c.readConfigState()
	if err != nil {
		return err
	}
	if err := c.writeConfigState(state); err != nil {
		return err
	}
	return nil
}

func (c *config) configStatePath() string {
	return filepath.Join(c.configDir, "config.json")
}

func (c *config) readConfigState() (*configState, error) {
	data, err := os.ReadFile(c.configStatePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &configState{
				Version: configStateVersion,
				Hosts:   map[string]map[string]string{},
			}, nil
		}
		return nil, err
	}

	var state configState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Version == 0 {
		state.Version = configStateVersion
	}
	if state.Hosts == nil {
		state.Hosts = map[string]map[string]string{}
	}
	return &state, nil
}

func (c *config) writeConfigState(state *configState) error {
	if state == nil {
		state = &configState{}
	}
	state.Version = configStateVersion
	if state.Hosts == nil {
		state.Hosts = map[string]map[string]string{}
	}
	if err := os.MkdirAll(c.configDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(c.configDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return secureWriteFile(c.configStatePath(), data, 0o600)
}

func (s *configState) host(hostname string) map[string]string {
	if hostname == "" {
		hostname = "gitcode.com"
	}
	return s.Hosts[hostname]
}

func (s *configState) ensureHost(hostname string) map[string]string {
	if hostname == "" {
		hostname = "gitcode.com"
	}
	if s.Hosts == nil {
		s.Hosts = map[string]map[string]string{}
	}
	values := s.Hosts[hostname]
	if values == nil {
		values = map[string]string{}
		s.Hosts[hostname] = values
	}
	return values
}

func normalizeConfigKey(key string) (string, error) {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	if normalizedKey == "" {
		return "", errors.New("config key is required")
	}
	if _, ok := allowedConfigKeys[normalizedKey]; !ok {
		return "", errors.New("unsupported config key")
	}
	return normalizedKey, nil
}

// NormalizeTrustedHost validates and normalizes a GitCode hostname.
func NormalizeTrustedHost(host string) (string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "gitcode.com", nil
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, "/\\@:") {
		return "", errors.New("invalid host: use a trusted hostname such as gitcode.com")
	}
	for _, r := range host {
		if r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return "", errors.New("invalid host: use a trusted hostname such as gitcode.com")
	}
	return host, nil
}
