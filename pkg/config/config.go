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
	// Set stores a configuration value. Returns ErrNoConfigDir when the
	// config directory cannot be resolved (no GC_CONFIG_DIR and no home
	// directory); the value is never written to a cwd-relative path.
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

	// Write persists the configuration. Returns ErrNoConfigDir when the
	// config directory cannot be resolved.
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
	// StoredToken returns the stored token for a host without environment
	// overrides. Callers that also need the username must use
	// StoredCredential instead (one consistent snapshot) rather than
	// pairing this with ActiveUser (two separate reads).
	StoredToken(hostname string) (string, string)
	// StoredCredential returns the active username and the stored token for
	// a host from a single read of the auth state, so the pair always comes
	// from one consistent snapshot; no environment overrides. The pair is
	// either complete or empty: any incomplete state (missing host, user,
	// or token, or a failed read; a missing or dangling active user counts as incomplete) returns ("", ""). Callers that need both
	// values must prefer this over ActiveUser+StoredToken (two separate
	// reads can straddle a concurrent auth.json rewrite).
	StoredCredential(hostname string) (username, token string)
	// HasActiveToken checks if a token exists for a host
	HasActiveToken(hostname string) bool
	// ActiveUser returns the active user for a host
	ActiveUser(hostname string) (string, error)
	// Hosts returns all configured hosts
	Hosts() []string
	// DefaultHost returns the default host
	DefaultHost() (string, string)
	// Login creates or updates authentication for a host. Returns
	// ErrNoConfigDir when the config directory cannot be resolved; the
	// stored state is never silently skipped.
	Login(hostname, username, token, gitProtocol string, secureStorage bool) (bool, error)
	// Logout removes authentication for a host. Reads an unresolvable
	// directory as an empty state, so a logout of a host that is not
	// stored is a successful no-op.
	Logout(hostname, username string) error
	// SwitchUser switches the active user for a host. Reads an
	// unresolvable directory as an empty state, so switching on a host
	// that is not stored fails with "host not found" before any write is
	// attempted; this method never returns ErrNoConfigDir.
	SwitchUser(hostname, user string) error
	// LoginUsers returns the sorted usernames that have stored tokens for a
	// host (empty when the host has no stored accounts). Interactive
	// account switching and not-found hints need the list (issue 646).
	LoginUsers(hostname string) []string
}

// config implements Config interface
type config struct {
	// configDir is the resolved configuration directory. The empty string
	// means no location could be resolved (no GC_CONFIG_DIR, no home
	// directory): reads then behave as an empty state and writes fail with
	// guidance instead of creating a "~/.config/gc" tree in the current
	// directory (the pre-fix code wrote auth.json there; issue 675).
	configDir string
}

// ErrNoConfigDir is returned by every write path when the config directory
// cannot be resolved (no GC_CONFIG_DIR and no home directory). Resolution
// never falls back to a cwd-relative path; callers surface this error so a
// login can never report success without persisting anything. Sibling
// packages that persist under the same directory (pkg/larkcli) reuse this
// sentinel instead of duplicating the guidance text. The message names the
// home variable per platform: os.UserHomeDir reads HOME on Unix and
// USERPROFILE on Windows, so the Windows parenthetical keeps the suggested
// remediation effective on both platforms.
var ErrNoConfigDir = errors.New("cannot resolve the config directory: HOME is not set; set HOME (USERPROFILE on Windows) or GC_CONFIG_DIR")

type configState struct {
	Version int                          `json:"version"`
	Hosts   map[string]map[string]string `json:"hosts,omitempty"`
}

// New creates a new Config. When the config directory cannot be resolved
// (no GC_CONFIG_DIR and no home directory), the returned Config treats
// reads as an empty state and fails writes with ErrNoConfigDir — it never
// falls back to a cwd-relative path.
func New() Config {
	dir, _ := configDirChecked()
	return &config{
		configDir: dir,
	}
}

// configDirChecked returns the configuration directory: GC_CONFIG_DIR first,
// then the home directory. It fails closed when neither is available: the
// previous "~" fallback produced a RELATIVE path (Go does not expand a
// literal tilde), so every write created a "~/.config/gc" tree in whatever
// directory the command ran in, and auth.json landed there with credentials
// inside (issue 675). The error names both variables that would fix it.
func configDirChecked() (string, error) {
	if dir := os.Getenv("GC_CONFIG_DIR"); dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", ErrNoConfigDir
	}

	return filepath.Join(home, ".config", "gc"), nil
}

// ResolveConfigDir returns the configuration directory with the same
// fail-closed resolution New() applies: GC_CONFIG_DIR first, then the home
// directory, error when neither is available. Sibling command code that
// needs the location (e.g. cache management) must resolve through here
// instead of re-implementing the lookup: a local copy of the lookup with a
// literal "~" fallback let `gc config clear-cache` read and delete planted
// files from a cwd-relative tree in HOME-less environments (found in review
// of the issue 675 fix).
func ResolveConfigDir() (string, error) {
	return configDirChecked()
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

// Set stores a configuration value. The read-modify-write runs under the
// config-dir cross-process lock (issue 672).
func (c *config) Set(host, key, value string) error {
	normalizedKey, err := normalizeConfigKey(key)
	if err != nil {
		return err
	}
	if c.configDir == "" {
		// Fail closed before locking (issue 672 ordering requirement: the
		// lock file lives in the config dir).
		return ErrNoConfigDir
	}
	release, err := lockConfigDir(c.configDir)
	if err != nil {
		return err
	}
	defer release()

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

// Write persists the configuration. The read-modify-write runs under the
// config-dir cross-process lock, which also ensures the directory exists
// (issue 672 hoisted the per-write MkdirAll/Chmod to lock acquisition).
func (c *config) Write() error {
	if c.configDir == "" {
		return ErrNoConfigDir
	}
	release, err := lockConfigDir(c.configDir)
	if err != nil {
		return err
	}
	defer release()

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
	if c.configDir == "" {
		// No resolvable config location: reads see an empty state. They must
		// not fall back to a cwd-relative path (issue 675).
		return &configState{
			Version: configStateVersion,
			Hosts:   map[string]map[string]string{},
		}, nil
	}
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
	if c.configDir == "" {
		// No resolvable config location: fail closed with guidance instead of
		// creating a "~/.config/gc" tree in the current directory (issue 675).
		return ErrNoConfigDir
	}
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
