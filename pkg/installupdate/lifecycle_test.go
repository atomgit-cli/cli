package installupdate

import (
	"bytes"
	"encoding/json"
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
	writeState(StatePath(), updateState{Summary: &stateSummary{Message: "updated", Shown: false}})
	out := &bytes.Buffer{}
	AfterCommand(nil, out, true, false)
	if out.String() != "updated\n" {
		t.Fatalf("output = %q", out.String())
	}
	if !readState(StatePath()).Summary.Shown {
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
	writeState(StatePath(), initial)
	lockPath := StatePath() + ".lock"
	if lock, err := acquireStateLock(lockPath); err != nil {
		t.Fatal(err)
	} else {
		defer lock.Close()
		defer os.Remove(lockPath)
	}

	out := &bytes.Buffer{}
	AfterCommand(nil, out, false, false)
	state := readState(StatePath())
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
			fields := strings.Fields(strings.TrimSpace(string(data)))
			if len(fields) != 2 {
				t.Fatalf("unexpected child report %q", string(data))
			}
			if fields[0] != fields[1] {
				t.Fatalf("detached helper is not a session leader: pid=%s pgid=%s", fields[0], fields[1])
			}
			return
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
