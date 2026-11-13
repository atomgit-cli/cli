package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
)

const authStateVersion = 1

// authConfig implements AuthConfig interface
type authConfig struct {
	config *config
}

type authState struct {
	Version     int                   `json:"version"`
	DefaultHost string                `json:"default_host,omitempty"`
	Hosts       map[string]*hostState `json:"hosts,omitempty"`
}

type hostState struct {
	ActiveUser string                `json:"active_user,omitempty"`
	Users      map[string]*userState `json:"users,omitempty"`
}

type userState struct {
	Token       string `json:"token"`
	GitProtocol string `json:"git_protocol,omitempty"`
}

// ActiveToken returns the active token for a host
func (a *authConfig) ActiveToken(hostname string) (string, string) {
	if token := os.Getenv("GC_TOKEN"); token != "" {
		return token, "GC_TOKEN"
	}
	if token := os.Getenv("GITCODE_TOKEN"); token != "" {
		return token, "GITCODE_TOKEN"
	}

	return a.StoredToken(hostname)
}

// StoredToken returns the stored token for a host without environment overrides.
func (a *authConfig) StoredToken(hostname string) (string, string) {
	state, err := a.config.readAuthState()
	if err != nil {
		return "", ""
	}
	user := state.activeUser(hostname)
	if user == nil || user.Token == "" {
		return "", ""
	}
	return user.Token, "config"
}

// StoredCredential returns the active username and stored token for a host
// from a single read of the auth state. ActiveUser and StoredToken are two
// separate reads of the same file that can straddle a concurrent rewrite
// (login or user switch in another process) and yield a mismatched pair;
// consumers that need both values require one consistent snapshot (issue
// 649). The pair is either complete or empty: any incomplete state returns
// ("", "").
func (a *authConfig) StoredCredential(hostname string) (string, string) {
	state, err := a.config.readAuthState()
	if err != nil {
		return "", ""
	}
	host := state.host(hostname)
	if host == nil || host.ActiveUser == "" {
		return "", ""
	}
	user := host.Users[host.ActiveUser]
	if user == nil || user.Token == "" {
		return "", ""
	}
	return host.ActiveUser, user.Token
}

// HasActiveToken checks if a token exists for a host
func (a *authConfig) HasActiveToken(hostname string) bool {
	token, _ := a.ActiveToken(hostname)
	return token != ""
}

// ActiveUser returns the active user for a host
func (a *authConfig) ActiveUser(hostname string) (string, error) {
	state, err := a.config.readAuthState()
	if err != nil {
		return "", err
	}

	host := state.host(hostname)
	if host == nil {
		return "", nil
	}
	return host.ActiveUser, nil
}

// Hosts returns all configured hosts
func (a *authConfig) Hosts() []string {
	state, err := a.config.readAuthState()
	if err != nil {
		return []string{"gitcode.com"}
	}

	hosts := map[string]struct{}{"gitcode.com": {}}
	if host := getEnvHost(); host != "" {
		hosts[host] = struct{}{}
	}
	for host := range state.Hosts {
		hosts[host] = struct{}{}
	}

	result := make([]string, 0, len(hosts))
	for host := range hosts {
		result = append(result, host)
	}
	sort.Strings(result)
	return result
}

// DefaultHost returns the default host
func (a *authConfig) DefaultHost() (string, string) {
	if host := getEnvHost(); host != "" {
		return host, "environment"
	}

	state, err := a.config.readAuthState()
	if err == nil && state.DefaultHost != "" {
		return state.DefaultHost, "config"
	}
	return "gitcode.com", "default"
}

// Login creates or updates authentication for a host
func (a *authConfig) Login(hostname, username, token, gitProtocol string, secureStorage bool) (bool, error) {
	if hostname == "" {
		hostname = "gitcode.com"
	}
	if username == "" || token == "" {
		return false, errors.New("username and token are required")
	}
	if secureStorage {
		return false, errors.New("secure storage is not supported yet")
	}
	if gitProtocol == "" {
		gitProtocol = "https"
	}

	state, err := a.config.readAuthState()
	if err != nil {
		return false, err
	}

	host := state.ensureHost(hostname)
	user := host.Users[username]
	changed := state.DefaultHost != hostname || host.ActiveUser != username
	if user == nil {
		user = &userState{}
		host.Users[username] = user
		changed = true
	}
	if user.Token != token || user.GitProtocol != gitProtocol {
		changed = true
	}

	user.Token = token
	user.GitProtocol = gitProtocol
	host.ActiveUser = username
	state.DefaultHost = hostname

	if !changed {
		return false, nil
	}

	return true, a.config.writeAuthState(state)
}

// Logout removes authentication for a host
func (a *authConfig) Logout(hostname, username string) error {
	if hostname == "" {
		hostname = "gitcode.com"
	}

	state, err := a.config.readAuthState()
	if err != nil {
		return err
	}

	host := state.host(hostname)
	if host == nil {
		return nil
	}

	if username == "" {
		username = host.ActiveUser
	}
	if username == "" {
		return nil
	}

	delete(host.Users, username)
	if len(host.Users) == 0 {
		delete(state.Hosts, hostname)
		if state.DefaultHost == hostname {
			state.DefaultHost = ""
		}
		return a.config.writeAuthState(state)
	}

	if host.ActiveUser == username {
		host.ActiveUser = firstUsername(host.Users)
	}

	return a.config.writeAuthState(state)
}

// SwitchUser switches the active user for a host
func (a *authConfig) SwitchUser(hostname, user string) error {
	if hostname == "" {
		hostname = "gitcode.com"
	}

	state, err := a.config.readAuthState()
	if err != nil {
		return err
	}

	host := state.host(hostname)
	if host == nil {
		return errors.New("host not found")
	}
	if _, ok := host.Users[user]; !ok {
		return errors.New("user not found")
	}

	host.ActiveUser = user
	// Deliberately not touching state.DefaultHost: switching re-selects an
	// account on one host and must not move the default host that bare
	// commands target (unlike Login, whose intent is "make this my primary
	// host"). SwitchUser had no production callers before auth switch, so
	// this pins the scoped semantics as the method's contract.
	return a.config.writeAuthState(state)
}

// LoginUsers returns the sorted usernames that have stored tokens for a
// host. The interactive account switch and the not-found hints need the
// list (issue 646); SwitchUser itself only validates membership.
// Read errors are swallowed (nil) like the sibling StoredToken/Hosts
// readers: an unreadable auth.json surfaces to the caller as "no stored
// accounts", which keeps the command usable at the exit-code level; the
// tradeoff is recorded here deliberately.
func (a *authConfig) LoginUsers(hostname string) []string {
	state, err := a.config.readAuthState()
	if err != nil {
		return nil
	}
	host := state.host(hostname)
	if host == nil {
		return nil
	}
	users := make([]string, 0, len(host.Users))
	for name := range host.Users {
		users = append(users, name)
	}
	sort.Strings(users)
	return users
}

func (c *config) authStatePath() string {
	return filepath.Join(c.configDir, "auth.json")
}

func (c *config) readAuthState() (*authState, error) {
	path := c.authStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &authState{
				Version: authStateVersion,
				Hosts:   map[string]*hostState{},
			}, nil
		}
		return nil, err
	}

	var state authState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Version == 0 {
		state.Version = authStateVersion
	}
	if state.Hosts == nil {
		state.Hosts = map[string]*hostState{}
	}
	for _, host := range state.Hosts {
		if host.Users == nil {
			host.Users = map[string]*userState{}
		}
	}
	return &state, nil
}

func (c *config) writeAuthState(state *authState) error {
	if state == nil {
		state = &authState{
			Version: authStateVersion,
			Hosts:   map[string]*hostState{},
		}
	}
	if state.Hosts == nil {
		state.Hosts = map[string]*hostState{}
	}
	state.Version = authStateVersion

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
	return secureWriteFile(c.authStatePath(), data, 0o600)
}

func (s *authState) host(hostname string) *hostState {
	if hostname == "" {
		hostname = s.DefaultHost
	}
	if hostname == "" {
		hostname = "gitcode.com"
	}
	return s.Hosts[hostname]
}

func (s *authState) ensureHost(hostname string) *hostState {
	if s.Hosts == nil {
		s.Hosts = map[string]*hostState{}
	}
	host := s.host(hostname)
	if host != nil {
		return host
	}
	host = &hostState{Users: map[string]*userState{}}
	s.Hosts[hostname] = host
	return host
}

func (s *authState) activeUser(hostname string) *userState {
	host := s.host(hostname)
	if host == nil || host.ActiveUser == "" {
		return nil
	}
	return host.Users[host.ActiveUser]
}

func firstUsername(users map[string]*userState) string {
	names := make([]string, 0, len(users))
	for name := range users {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func getEnvHost() string {
	return os.Getenv("GC_HOST")
}
