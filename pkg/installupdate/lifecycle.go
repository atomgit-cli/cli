// Package installupdate manages the npm-bootstrap updater lifecycle.
package installupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gitcode.com/gitcode-cli/cli/pkg/config"
)

const (
	updateTTL       = 24 * time.Hour
	updateLockStale = 15 * time.Minute
	checkDeadline   = 120 * time.Second
)

// Manifest describes an npm-bootstrap installation.
type Manifest struct {
	Distribution string `json:"distribution"`
	Version      string `json:"version"`
	TargetDir    string `json:"targetDir"`
	Node         string `json:"node"`
	NPM          string `json:"npm"`
	Helper       string `json:"helper"`
	// Package is the npm coordinate being bootstrapped. Optional: legacy
	// manifests lack it and fall back to the flat state path, matching the
	// old helper that writes the same flat file.
	Package string `json:"package,omitempty"`
	path    string
}

type updateState struct {
	NextCheck   int64         `json:"nextCheck,omitempty"`
	NoticeShown bool          `json:"noticeShown,omitempty"`
	Summary     *stateSummary `json:"summary,omitempty"`
	// Failure bookkeeping written by the JS updaters (backoff streak,
	// summary deduplication, permanent-failure stop). Every field must be
	// declared here: mutateStateLocked rewrites the whole file from this
	// struct, so an undeclared field would be silently dropped.
	LastChecked          string `json:"lastChecked,omitempty"`
	FailureStreak        int64  `json:"failureStreak,omitempty"`
	LastErrorFingerprint string `json:"lastErrorFingerprint,omitempty"`
	PermanentError       bool   `json:"permanentError,omitempty"`
}

type stateSummary struct {
	Message string `json:"message"`
	Shown   bool   `json:"shown"`
}

// LoadBootstrapManifest returns the manifest adjacent to the running binary.
func LoadBootstrapManifest() (*Manifest, error) {
	binary := os.Getenv("GITCODE_CLI_BINARY")
	if binary == "" {
		var err error
		binary, err = os.Executable()
		if err != nil {
			return nil, err
		}
	}
	path := filepath.Join(filepath.Dir(binary), ".gitcode-install.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Distribution != "npm-bootstrap" || manifest.TargetDir == "" || manifest.Helper == "" {
		return nil, fmt.Errorf("invalid npm-bootstrap manifest")
	}
	manifest.path = path
	return &manifest, nil
}

// ManifestPath returns the source path for the loaded manifest.
func (m *Manifest) ManifestPath() string {
	return m.path
}

// AfterCommand shows pending status and schedules a due bootstrap update.
// Errors are intentionally reported to stderr and never change command status.
func AfterCommand(cfg config.Config, errOut io.Writer, noUpdate, noInteractive bool) {
	manifest, err := LoadBootstrapManifest()
	if err != nil {
		return
	}
	statePath := StatePath(manifest)
	updatesDisabled := disabled(cfg, noUpdate, noInteractive)
	var state updateState
	if !mutateStateLocked(statePath, func(current *updateState) {
		if current.Summary != nil && !current.Summary.Shown {
			fmt.Fprintln(errOut, current.Summary.Message)
			current.Summary.Shown = true
		}
		if !updatesDisabled && !current.NoticeShown {
			fmt.Fprintln(errOut, "GitCode CLI installed by npm bootstrap checks daily for stable updates and notifies without installing them.")
			fmt.Fprintln(errOut, `Run "gitcode update", opt in with "gitcode config set update.mode auto", or disable checks with update.mode off.`)
			current.NoticeShown = true
		}
		state = *current
	}) {
		return
	}
	if updatesDisabled {
		return
	}
	// A permanent failure (broken manifest, missing npm runtime) stops
	// background scheduling until the user repairs the install; the pending
	// summary above is still shown first.
	if state.PermanentError {
		return
	}
	if state.NextCheck > time.Now().UnixMilli() {
		return
	}
	if err := StartDetached(manifest, false); err != nil {
		fmt.Fprintf(errOut, "update check skipped: %v\n", err)
	}
}

func mutateStateLocked(path string, mutate func(*updateState)) bool {
	lockPath := path + ".lock"
	lock, err := acquireStateLock(lockPath)
	if err != nil {
		return false
	}
	defer func() {
		_ = lock.Close()
		_ = os.Remove(lockPath)
	}()
	state := readState(path)
	mutate(&state)
	writeState(path, state)
	return true
}

func acquireStateLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if !os.IsExist(err) {
		return lock, err
	}
	info, statErr := os.Stat(path)
	if statErr != nil || time.Since(info.ModTime()) <= updateLockStale {
		return nil, err
	}
	if removeErr := os.Remove(path); removeErr != nil {
		return nil, removeErr
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

// StatePath returns the npm-bootstrap update state file, scoped per package
// coordinate and channel so parallel installs (npm-global, other
// coordinates) do not share nextCheck or summaries. An explicit
// GC_STATE_DIR keeps the legacy flat file; a legacy manifest without a
// package name falls back to the flat path too, staying consistent with
// the old helper copy that writes the same flat file.
func StatePath(manifest *Manifest) string {
	if dir := os.Getenv("GC_STATE_DIR"); dir != "" {
		return filepath.Join(dir, "update-state.json")
	}
	root := stateRoot()
	if manifest != nil && manifest.Package != "" {
		return filepath.Join(root, manifest.Package, "npm-bootstrap", "update-state.json")
	}
	return filepath.Join(root, "update-state.json")
}

func stateRoot() string {
	if local := os.Getenv("LOCALAPPDATA"); runtime.GOOS == "windows" && local != "" {
		return filepath.Join(local, "gitcode-cli")
	}
	home, _ := os.UserHomeDir()
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "gitcode-cli")
}

// StartDetached runs the copied bootstrap helper after this process exits.
func StartDetached(manifest *Manifest, force bool) error {
	node := resolveNode(manifest.Node)
	args := []string{
		manifest.Helper,
		"--background",
		"--parent-pid", strconv.Itoa(os.Getpid()),
		"--manifest", manifest.ManifestPath(),
	}
	if force {
		args = append(args, "--force")
	}
	cmd := exec.Command(node, args...)
	cmd.Env = updaterEnvironment()
	cmd.SysProcAttr = detachAttrs()
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// RunCheck executes an explicit foreground update check.
func RunCheck(manifest *Manifest, jsonOutput bool, out, errOut io.Writer) error {
	node := resolveNode(manifest.Node)
	args := []string{manifest.Helper, "--check", "--manifest", manifest.ManifestPath()}
	if jsonOutput {
		args = append(args, "--json")
	}
	ctx, cancel := context.WithTimeout(context.Background(), checkDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Env = updaterEnvironment()
	// Capture the streams so a failed check can surface the helper's reason
	// in the returned error (matching CheckNow's wrapping) instead of a bare
	// "exit status 1"; everything captured is still forwarded to the
	// caller's writers.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if out != nil {
		cmd.Stdout = io.MultiWriter(out, &stdout)
	}
	if errOut != nil {
		cmd.Stderr = io.MultiWriter(errOut, &stderr)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update check failed: %s", truncateDetail(checkFailureDetail(stdout.String(), stderr.String(), err)))
	}
	return nil
}

// checkFailureDetail extracts the helper's failure reason: the JSON error
// message first (--json mode prints it to stdout), then the helper's stderr
// line with its own "update failed: " prefix stripped (avoiding double
// wrapping), then the raw run error.
func checkFailureDetail(stdout, stderr string, runErr error) string {
	var parsed struct {
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(stdout), &parsed) == nil && parsed.Message != "" {
		return parsed.Message
	}
	line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(stderr), "update failed:"))
	if line != "" {
		return line
	}
	return runErr.Error()
}

// CheckResult mirrors the bootstrap helper's JSON result for a foreground
// update check.
type CheckResult struct {
	Status       string `json:"status"`
	Distribution string `json:"distribution"`
	Current      string `json:"current"`
	Latest       string `json:"latest"`
	Message      string `json:"message"`
}

// CheckNow runs a foreground update check and parses the helper's result.
// A failed check returns the helper's error message so callers can surface
// the underlying cause instead of a generic failure notice.
func CheckNow(manifest *Manifest) (*CheckResult, error) {
	node := resolveNode(manifest.Node)
	args := []string{manifest.Helper, "--check", "--json", "--manifest", manifest.ManifestPath()}
	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), checkDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, args...)
	cmd.Env = updaterEnvironment()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	var result CheckResult
	parseErr := json.Unmarshal(stdout.Bytes(), &result)
	if parseErr == nil && result.Status != "error" {
		return &result, nil
	}
	detail := ""
	if parseErr == nil {
		detail = result.Message
	}
	if detail == "" {
		detail = strings.TrimSpace(stderr.String())
	}
	if detail == "" {
		switch {
		case runErr != nil:
			detail = runErr.Error()
		case parseErr != nil:
			detail = parseErr.Error()
		}
	}
	return nil, fmt.Errorf("update check failed: %s", truncateDetail(detail))
}

// truncateDetail bounds helper error details to one collapsed line so failure
// summaries and JSON output stay readable regardless of npm stderr volume.
func truncateDetail(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	if text == "" {
		text = "unknown update check failure"
	}
	return text
}

func resolveNode(recorded string) string {
	if recorded != "" {
		if _, err := os.Stat(recorded); err == nil || !filepath.IsAbs(recorded) {
			return recorded
		}
	}
	if current, err := exec.LookPath("node"); err == nil {
		return current
	}
	return "node"
}

func disabled(cfg config.Config, noUpdate, noInteractive bool) bool {
	if noUpdate || noInteractive || truthy(os.Getenv("GC_NO_UPDATE_CHECK")) || truthy(os.Getenv("CI")) {
		return true
	}
	mode := strings.ToLower(os.Getenv("GC_UPDATE_MODE"))
	if mode == "" && cfg != nil {
		mode, _ = cfg.Get("gitcode.com", "update.mode")
	}
	return mode == "off"
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func updaterEnvironment() []string {
	allowed := map[string]struct{}{
		"ALL_PROXY": {}, "APPDATA": {}, "COMSPEC": {}, "GC_CONFIG_DIR": {},
		"GC_STATE_DIR": {}, "GC_UPDATE_MODE": {}, "HOME": {}, "HTTP_PROXY": {},
		"HTTPS_PROXY": {}, "LANG": {}, "LC_ALL": {}, "LOCALAPPDATA": {},
		"NODE_EXTRA_CA_CERTS": {}, "NO_PROXY": {}, "PATH": {}, "PATHEXT": {}, "SSL_CERT_DIR": {},
		"SSL_CERT_FILE": {}, "SYSTEMROOT": {}, "TEMP": {}, "TMP": {},
		"USERPROFILE": {}, "WINDIR": {}, "XDG_CONFIG_HOME": {}, "XDG_STATE_HOME": {},
	}
	clean := make([]string, 0, len(allowed)+1)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, ok := allowed[strings.ToUpper(key)]; ok {
			clean = append(clean, item)
		}
	}
	return append(clean, "GC_NO_UPDATE_CHECK=1")
}

func readState(path string) updateState {
	data, err := os.ReadFile(path)
	if err != nil {
		return updateState{}
	}
	var state updateState
	if json.Unmarshal(data, &state) != nil {
		return updateState{}
	}
	return state
}

func writeState(path string, state updateState) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".update-state-*")
	if err != nil {
		return
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return
	}
	if err := temp.Close(); err != nil {
		return
	}
	if os.Rename(tempPath, path) == nil {
		return
	}
	_ = os.Remove(path)
	_ = os.Rename(tempPath, path)
}

// DueAt returns the next automatic-check time used by tests and diagnostics.
func DueAt(now time.Time) int64 {
	return now.Add(updateTTL).UnixMilli()
}
