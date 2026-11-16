package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The old behavior under test (issue 672): the read-modify-write paths held
// no cross-process lock, so two concurrent logouts both reported success
// while one deletion was silently lost — the live-command reproduction on
// origin/main hit this in 50/50 rounds (each round ended with exactly one
// surviving user although both exits were 0 with "✓ Cleared").
func TestConcurrentLogoutsLoseNoUpdates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)

	seed := func() {
		auth := New().Authentication()
		for _, u := range []string{"alice", "bob"} {
			if _, err := auth.Login("gitcode.com", u, "token-"+u, "https", false); err != nil {
				t.Fatalf("seed Login(%s) error = %v", u, err)
			}
		}
	}

	for round := 0; round < 20; round++ {
		seed()
		if got := New().Authentication().LoginUsers("gitcode.com"); len(got) != 2 {
			t.Fatalf("round %d: seed left %v, want [alice bob] (guards a vacuous pass)", round, got)
		}
		var wg sync.WaitGroup
		for _, u := range []string{"alice", "bob"} {
			wg.Add(1)
			go func(user string) {
				defer wg.Done()
				if err := New().Authentication().Logout("gitcode.com", user); err != nil {
					t.Errorf("round %d: Logout(%s) error = %v", round, user, err)
				}
			}(u)
		}
		wg.Wait()

		users := New().Authentication().LoginUsers("gitcode.com")
		if len(users) != 0 {
			t.Fatalf("round %d: concurrent logouts left %v — a deletion was lost", round, users)
		}
	}
}

// logout‖switch matrix (issue acceptance): a logout of one user racing a
// switch to the other must never leave a dangling active_user pointing at
// the removed account, and the remaining user must survive.
func TestConcurrentLogoutSwitchMatrix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)

	for round := 0; round < 20; round++ {
		// Login bob first, then alice: active_user ends as alice, so the
		// racing logout of alice exercises the active-user reassignment
		// path (the previous seeding left active=bob and never touched it).
		auth := New().Authentication()
		for _, u := range []string{"bob", "alice"} {
			if _, err := auth.Login("gitcode.com", u, "token-"+u, "https", false); err != nil {
				t.Fatalf("seed Login(%s) error = %v", u, err)
			}
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := New().Authentication().Logout("gitcode.com", "alice"); err != nil {
				t.Errorf("round %d: Logout(alice) error = %v", round, err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := New().Authentication().SwitchUser("gitcode.com", "bob"); err != nil {
				t.Errorf("round %d: SwitchUser(bob) error = %v", round, err)
			}
		}()
		wg.Wait()

		users := New().Authentication().LoginUsers("gitcode.com")
		if len(users) != 1 || users[0] != "bob" {
			t.Fatalf("round %d: users = %v, want exactly [bob] (logout of alice must not remove bob)", round, users)
		}
		user, _ := New().Authentication().StoredCredential("gitcode.com")
		if user != "bob" {
			t.Fatalf("round %d: active user = %q, want bob (never a dangling reference to the removed alice)", round, user)
		}
	}
}

// A logout of a user that is not stored must not rewrite auth.json at all
// (issue 672: the unconditional no-op rewrite both widened the race window
// and churned the file for nothing).
func TestLogoutOfAbsentUserDoesNotRewrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)

	auth := New().Authentication()
	if _, err := auth.Login("gitcode.com", "alice", "token", "https", false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "auth.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}

	if err := auth.Logout("gitcode.com", "ghost"); err != nil {
		t.Fatalf("Logout(ghost) error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(past) {
		t.Fatal("logout of an absent user rewrote auth.json — the no-op rewrite must be skipped")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("logout of an absent user changed auth.json content")
	}
}

// Acquiring the write lock must sweep stale .gc-tmp-* leftovers (crash or
// failed-cleanup residue holding credential payloads) while keeping
// anything younger than the age gate (an in-flight temp of a
// version-skew writer that predates the lock).
func TestLockSweepsStaleTempFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)

	stale := filepath.Join(dir, ".gc-tmp-stale")
	if err := os.WriteFile(stale, []byte("old payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(dir, ".gc-tmp-fresh")
	if err := os.WriteFile(fresh, []byte("in-flight"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Near-boundary pair pins the age gate itself (a drift of the
	// tempSweepAge var to either side flips one of these).
	justOld := filepath.Join(dir, ".gc-tmp-justold")
	if err := os.WriteFile(justOld, []byte("just past the gate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(justOld, time.Now().Add(-tempSweepAge-time.Second), time.Now().Add(-tempSweepAge-time.Second)); err != nil {
		t.Fatal(err)
	}
	justNew := filepath.Join(dir, ".gc-tmp-justnew")
	if err := os.WriteFile(justNew, []byte("just inside the gate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(justNew, time.Now().Add(-tempSweepAge+time.Second), time.Now().Add(-tempSweepAge+time.Second)); err != nil {
		t.Fatal(err)
	}

	if _, err := New().Authentication().Login("gitcode.com", "alice", "token", "https", false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp not swept: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh temp must be kept by the age gate: %v", err)
	}
	if _, err := os.Stat(justOld); !os.IsNotExist(err) {
		t.Fatalf("temp just past the gate (%s) must be swept: %v", tempSweepAge, err)
	}
	if _, err := os.Stat(justNew); err != nil {
		t.Fatalf("temp just inside the gate must be kept: %v", err)
	}
}

// A second acquisition of an already-held lock must fail loudly after the
// bounded timeout instead of blocking forever (a stuck gc process must not
// wedge every other invocation).
func TestLockTimeoutIsLoud(t *testing.T) {
	dir := t.TempDir()

	release, err := lockConfigDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	old := lockTimeout
	lockTimeout = 50 * time.Millisecond
	defer func() { lockTimeout = old }()

	_, err = lockConfigDir(dir)
	if err == nil {
		t.Fatal("second lock acquisition should time out")
	}
	if !strings.Contains(err.Error(), "lock") {
		t.Fatalf("error = %q, want it to name the lock", err.Error())
	}
}
