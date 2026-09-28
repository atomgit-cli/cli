package installupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadBootstrapManifestAdjacentToBinary(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "gitcode")
	t.Setenv("GITCODE_CLI_BINARY", binary)
	manifestPath := filepath.Join(dir, ".gitcode-install.json")
	data := []byte(`{"distribution":"npm-bootstrap","version":"1.2.3","targetDir":"` + escapedJSON(dir) + `","helper":"` + escapedJSON(filepath.Join(dir, "helper.js")) + `"}`)
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadBootstrapManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.2.3" || manifest.ManifestPath() != manifestPath {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestAfterCommandShowsSummaryButDoesNotScheduleWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITCODE_CLI_BINARY", filepath.Join(dir, "gitcode"))
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	manifest := Manifest{Distribution: "npm-bootstrap", Version: "1.2.3", TargetDir: dir, Helper: filepath.Join(dir, "missing.js")}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	writeState(StatePath(nil), updateState{Summary: &stateSummary{Message: "updated", Shown: false}})
	out := &bytes.Buffer{}
	AfterCommand(nil, out, true, false)
	if out.String() != "updated\n" {
		t.Fatalf("output = %q", out.String())
	}
	if !readState(StatePath(nil)).Summary.Shown {
		t.Fatal("summary should be marked shown")
	}
}

func TestAfterCommandDoesNotOverwriteStateOwnedByUpdater(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITCODE_CLI_BINARY", filepath.Join(dir, "gitcode"))
	t.Setenv("GC_STATE_DIR", filepath.Join(dir, "state"))
	manifest := Manifest{Distribution: "npm-bootstrap", Version: "1.2.3", TargetDir: dir, Helper: filepath.Join(dir, "missing.js")}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	initial := updateState{NextCheck: 123, Summary: &stateSummary{Message: "pending", Shown: false}}
	writeState(StatePath(nil), initial)
	lockPath := StatePath(nil) + ".lock"
	if lock, err := acquireStateLock(lockPath); err != nil {
		t.Fatal(err)
	} else {
		defer lock.Close()
		defer os.Remove(lockPath)
	}

	out := &bytes.Buffer{}
	AfterCommand(nil, out, false, false)
	state := readState(StatePath(nil))
	if out.Len() != 0 || state.NextCheck != initial.NextCheck || state.Summary.Shown {
		t.Fatalf("state was changed while updater lock was held: %#v, output %q", state, out.String())
	}
}

func TestDueAtUsesTwentyFourHourTTL(t *testing.T) {
	now := time.Unix(100, 0)
	if got := DueAt(now); got != now.Add(24*time.Hour).UnixMilli() {
		t.Fatalf("DueAt() = %d", got)
	}
}

func TestCheckFailureDetailPrefersReasonOverRawExit(t *testing.T) { // --json mode prints the error message to stdout.
	if got := checkFailureDetail(`{"status":"error","message":"registry reset"}`, "", errors.New("exit status 1")); got != "registry reset" {
		t.Fatalf("json detail = %q", got)
	}
	// Plain mode prints "update failed: <reason>" to stderr; the prefix must
	// be stripped so it is not double-wrapped by "update check failed:".
	if got := checkFailureDetail("", "update failed: cannot compare 1 and 2\n", errors.New("exit status 1")); got != "cannot compare 1 and 2" {
		t.Fatalf("stderr detail = %q", got)
	}
	// Nothing captured: the raw run error stands.
	if got := checkFailureDetail("", "", errors.New("exit status 1")); got != "exit status 1" {
		t.Fatalf("raw detail = %q", got)
	}
}

func TestTruncateDetailIsRuneSafe(t *testing.T) {
	text := strings.Repeat("汉", 250) // 750 bytes, 250 runes
	got := truncateDetail(text)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	runes := []rune(strings.TrimSuffix(got, "..."))
	if len(runes) != 200 {
		t.Fatalf("expected 200 runes after truncation, got %d", len(runes))
	}
	for _, r := range runes {
		if r == 0xfffd {
			t.Fatal("replacement character leaked into the truncation")
		}
	}
}

func TestResolveUpdateModeFallsBackToConfigOnInvalidEnv(t *testing.T) {
	cases := []struct {
		envValue, cfgValue, want string
	}{
		{"off", "auto", "off"},     // valid env wins
		{"", "off", "off"},         // empty env falls back to config
		{"banana", "off", "off"},   // invalid env falls back to config (was: kept checking)
		{"banana", "", "notify"},   // nothing valid anywhere
		{" NOTIFY ", "", "notify"}, // trimmed and case-folded
		{"", "garbage", "notify"},  // invalid config falls back to the default
	}
	for _, tc := range cases {
		if got := resolveUpdateMode(tc.envValue, tc.cfgValue); got != tc.want {
			t.Fatalf("resolveUpdateMode(%q, %q) = %q, want %q", tc.envValue, tc.cfgValue, got, tc.want)
		}
	}
}

func TestCIEnvironmentCoversVendorsWithoutCIFlag(t *testing.T) {
	for _, name := range []string{"GITHUB_ACTIONS", "BUILD_NUMBER", "CI_NAME", "TEAMCITY_VERSION"} {
		t.Setenv(name, "1")
		if !ciEnvironment() {
			t.Fatalf("%s must be detected as CI", name)
		}
		t.Setenv(name, "")
	}
	t.Setenv("CI", "")
	if ciEnvironment() {
		t.Fatal("no CI variables set must not be detected as CI")
	}
	t.Setenv("CI", "true")
	if !ciEnvironment() {
		t.Fatal("CI=true must be detected")
	}
}

func TestWindowsStateRootMatchesJSPrecedence(t *testing.T) {
	if got := windowsStateRoot(`C:\Users\u\AppData\Local`, `C:\Users\u`); got != filepath.Join(`C:\Users\u\AppData\Local`, "gitcode-cli") {
		t.Fatalf("LOCALAPPDATA wins: %q", got)
	}
	// An empty LOCALAPPDATA must fall back to <home>\AppData\Local instead
	// of splitting the Go and JS halves onto different state files.
	if got := windowsStateRoot("", `C:\Users\u`); got != filepath.Join(`C:\Users\u`, "AppData", "Local", "gitcode-cli") {
		t.Fatalf("home fallback: %q", got)
	}
}

func TestReadStatePreservesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_STATE_DIR", dir)
	path := StatePath(nil)
	corrupt := []byte(`{"nextCheck": "not-a-number"`)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if state := readState(path); state != (updateState{}) {
		t.Fatalf("corrupt state must reset, got %#v", state)
	}
	preserved, err := os.ReadFile(path + ".corrupt")
	if err != nil || !bytes.Equal(preserved, corrupt) {
		t.Fatalf("the corrupt bytes must be preserved beside the state file: %v", err)
	}
}

func TestStatePathScopesPerPackageAndChannel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_STATE_DIR", "")
	if runtime.GOOS == "windows" {
		// stateRoot derives from LOCALAPPDATA only on Windows (mirroring the
		// JS stateDir precedence); point it at the temp dir.
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("XDG_STATE_HOME", dir)
	}
	if got := StatePath(nil); got != filepath.Join(dir, "gitcode-cli", "update-state.json") {
		t.Fatalf("legacy manifest state path = %q", got)
	}
	scoped := filepath.Join(dir, "gitcode-cli", "atomgit-cli", "npm-bootstrap", "update-state.json")
	if got := StatePath(&Manifest{Package: "atomgit-cli"}); got != scoped {
		t.Fatalf("scoped state path = %q, want %q", got, scoped)
	}
	// The explicit override keeps the legacy flat file.
	override := t.TempDir()
	t.Setenv("GC_STATE_DIR", override)
	if got := StatePath(&Manifest{Package: "atomgit-cli"}); got != filepath.Join(override, "update-state.json") {
		t.Fatalf("override state path = %q", got)
	}
}

func TestAfterCommandUsesScopedStatePathFromManifest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITCODE_CLI_BINARY", filepath.Join(dir, "gitcode"))
	t.Setenv("GC_STATE_DIR", "")
	if runtime.GOOS == "windows" {
		// See TestStatePathScopesPerPackageAndChannel: the Windows state root
		// comes from LOCALAPPDATA only.
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("LOCALAPPDATA", "")
		t.Setenv("XDG_STATE_HOME", dir)
	}
	manifest := Manifest{
		Distribution: "npm-bootstrap",
		Version:      "1.2.3",
		TargetDir:    dir,
		Helper:       filepath.Join(dir, "missing.js"),
		Package:      "atomgit-cli",
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	scoped := filepath.Join(dir, "gitcode-cli", "atomgit-cli", "npm-bootstrap", "update-state.json")
	writeState(scoped, updateState{Summary: &stateSummary{Message: "updated", Shown: false}})
	out := &bytes.Buffer{}
	AfterCommand(nil, out, true, false)
	if out.String() != "updated\n" {
		t.Fatalf("output = %q", out.String())
	}
	if !readState(scoped).Summary.Shown {
		t.Fatal("the scoped state file must be the one read and written")
	}
}

// Regression gate: mutateStateLocked rewrites the whole state file from the
// updateState struct, so any field the JS updaters write must be declared
// here or it is silently dropped.
func TestAfterCommandPreservesUpdaterFailureFields(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITCODE_CLI_BINARY", filepath.Join(dir, "gitcode"))
	t.Setenv("GC_STATE_DIR", stateDir)
	manifest := Manifest{Distribution: "npm-bootstrap", Version: "1.2.3", TargetDir: dir, Helper: filepath.Join(dir, "missing.js")}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, ".gitcode-install.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	raw := `{"lastChecked":"2026-09-28T00:00:00.000Z","nextCheck":99999999999999,` +
		`"failureStreak":3,"lastErrorFingerprint":"abc123","permanentError":true,` +
		`"summary":{"message":"Automatic update failed: x.","shown":false}}`
	if err := os.WriteFile(StatePath(nil), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	AfterCommand(nil, out, false, false)
	state := readState(StatePath(nil))
	if state.LastChecked != "2026-09-28T00:00:00.000Z" || state.FailureStreak != 3 ||
		state.LastErrorFingerprint != "abc123" || !state.PermanentError {
		t.Fatalf("updater failure fields were dropped: %#v", state)
	}
	if !state.Summary.Shown || !strings.HasPrefix(out.String(), "Automatic update failed: x.\n") {
		t.Fatalf("pending summary must still be shown first: %q", out.String())
	}
	// The permanent gate must stop before scheduling: no detached-spawn
	// attempt (which would print an "update check skipped" line here,
	// since the manifest helper does not exist).
	if strings.Contains(out.String(), "update check skipped") {
		t.Fatalf("permanent failure must not schedule a background check: %q", out.String())
	}
}

func TestResolveNodeFallsBackWhenRecordedRuntimeIsMissing(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "missing-node")
	if got := resolveNode(recorded); got == recorded {
		t.Fatalf("resolveNode() kept missing recorded runtime %q", got)
	}
}

func TestStartDetachedRunsInOwnSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX session semantics only")
	}
	dir := t.TempDir()
	reportFile := filepath.Join(dir, "child-report")
	// After Setsid the child is a session leader: its process group id equals
	// its own pid. A plain fork keeps the parent's group, where pid != pgid.
	script := fmt.Sprintf("#!/bin/sh\necho \"$$ $(ps -o pgid= -p $$ | tr -d ' ')\" > %s\nsleep 1\n", reportFile)
	node := filepath.Join(dir, "fake-node")
	if err := os.WriteFile(node, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{
		Distribution: "npm-bootstrap",
		Version:      "1.2.3",
		TargetDir:    dir,
		Node:         node,
		Helper:       filepath.Join(dir, "helper.js"),
		path:         filepath.Join(dir, ".gitcode-install.json"),
	}
	if err := StartDetached(manifest, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(reportFile)
		if err == nil {
			// The shell truncates the file before writing, so tolerate
			// partial reads until the full "pid pgid" report lands.
			if fields := strings.Fields(strings.TrimSpace(string(data))); len(fields) == 2 {
				if fields[0] != fields[1] {
					t.Fatalf("detached helper is not a session leader: pid=%s pgid=%s", fields[0], fields[1])
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("detached helper did not report in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCheckNowParsesHelperResult(t *testing.T) {
	script := `const r={status:"current",distribution:"npm-bootstrap",current:"1.2.3",` +
		`latest:"1.2.3",message:"GitCode CLI 1.2.3 is current."};` +
		`process.stdout.write(JSON.stringify(r))`
	node, helper := stubCheckHelper(t, script)
	result, err := CheckNow(stubCheckManifest(t, node, helper))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "current" || result.Latest != "1.2.3" {
		t.Fatalf("unexpected check result: %#v", result)
	}
}

func TestCheckNowReturnsHelperErrorDetail(t *testing.T) {
	script := `process.stdout.write(JSON.stringify({status:"error",` +
		`message:"npm registry check failed: network ECONNRESET"})); process.exit(1);`
	node, helper := stubCheckHelper(t, script)
	_, err := CheckNow(stubCheckManifest(t, node, helper))
	if err == nil {
		t.Fatal("expected the helper's error to propagate")
	}
	if !strings.Contains(err.Error(), "ECONNRESET") {
		t.Fatalf("error must carry the real cause, got %q", err.Error())
	}
}

func TestCheckNowBoundsRunawayHelperDetail(t *testing.T) {
	script := `process.stdout.write(JSON.stringify({status:"error",` +
		`message:"boom ` + strings.Repeat("x", 400) + `"})); process.exit(1);`
	node, helper := stubCheckHelper(t, script)
	_, err := CheckNow(stubCheckManifest(t, node, helper))
	if err == nil {
		t.Fatal("expected the helper's error to propagate")
	}
	if len(err.Error()) > 240 {
		t.Fatalf("detail must be truncated, got %d chars", len(err.Error()))
	}
}

func TestCheckNowReportsUnparseableHelperOutput(t *testing.T) {
	node, helper := stubCheckHelper(t, `process.stderr.write("node: bad helper"); process.exit(1);`)
	_, err := CheckNow(stubCheckManifest(t, node, helper))
	if err == nil {
		t.Fatal("expected an error for unparseable helper output")
	}
	if !strings.Contains(err.Error(), "bad helper") {
		t.Fatalf("error must fall back to helper stderr, got %q", err.Error())
	}
}

func stubCheckHelper(t *testing.T, script string) (string, string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available for helper stubbing")
	}
	helper := filepath.Join(t.TempDir(), "check-helper.js")
	if err := os.WriteFile(helper, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	return node, helper
}

func stubCheckManifest(t *testing.T, node, helper string) *Manifest {
	t.Helper()
	dir := t.TempDir()
	return &Manifest{
		Distribution: "npm-bootstrap",
		Version:      "1.2.3",
		TargetDir:    dir,
		Node:         node,
		Helper:       helper,
		path:         filepath.Join(dir, ".gitcode-install.json"),
	}
}

func TestUpdaterEnvironmentStripsCredentials(t *testing.T) {
	t.Setenv("PATH", "test-path")
	t.Setenv("GC_TOKEN", "gitcode-secret")
	t.Setenv("GITCODE_TOKEN", "legacy-secret")
	t.Setenv("NPM_TOKEN", "npm-secret")
	t.Setenv("NODE_AUTH_TOKEN", "node-secret")
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "cloud-secret")
	seenPath := false
	for _, item := range updaterEnvironment() {
		upper := strings.ToUpper(item)
		if strings.HasPrefix(upper, "PATH=") {
			seenPath = true
		}
		for _, key := range []string{
			"GC_TOKEN=", "GITCODE_TOKEN=", "NPM_TOKEN=", "NODE_AUTH_TOKEN=",
			"GITHUB_TOKEN=", "AWS_SECRET_ACCESS_KEY=",
		} {
			if strings.HasPrefix(upper, key) {
				t.Fatalf("credential leaked to updater environment: %s", key)
			}
		}
	}
	if !seenPath {
		t.Fatal("PATH should be preserved for updater tools")
	}
}

func escapedJSON(value string) string {
	data, _ := json.Marshal(value)
	return string(data[1 : len(data)-1])
}
