package storesync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dixie6502/folio/internal/testutil"
)

// testTime is the fixed clock of every test Syncer.
var testTime = time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)

// Helpers shared with the other packages' tests.
var (
	isolate = testutil.Home
	run     = testutil.Run
	write   = testutil.Write
)

// read returns the content of the file path.
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// newRemote creates a bare repository holding a store with its marker, and
// returns its file:// URL.
func newRemote(t *testing.T, home string) string {
	t.Helper()
	remote := filepath.Join(home, "remote.git")
	run(t, home, "git", "init", "-q", "--bare", "-b", "main", remote)
	run(t, remote, "git", "config", "uploadpack.allowFilter", "true")
	seed := filepath.Join(home, "seed")
	run(t, home, "git", "clone", "-q", remote, seed)
	write(t, filepath.Join(seed, ".folio"), "")
	run(t, seed, "git", "add", ".folio")
	run(t, seed, "git", "commit", "-q", "-m", "store")
	run(t, seed, "git", "push", "-q", "origin", "main")
	return "file://" + remote
}

// machine clones the store from url as machine name would hold it, at
// <home>/<name>/GitHub/owner/notebooks, and returns the store and a Syncer
// for that machine. Extra clone arguments (e.g. --sparse) go in cloneArgs.
func machine(t *testing.T, home, name, url string, cloneArgs ...string) (string, Syncer) {
	t.Helper()
	owner := filepath.Join(home, name, "GitHub", "owner")
	if err := os.MkdirAll(owner, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{"clone", "-q"}, cloneArgs...), url, "notebooks")
	run(t, owner, "git", args...)
	gitleaks, err := exec.LookPath("gitleaks")
	if err != nil {
		t.Fatal("gitleaks is needed to test pushing")
	}
	return filepath.Join(owner, "notebooks"), Syncer{
		Host:       name,
		Gitleaks:   gitleaks,
		StatusFile: filepath.Join(home, name, "status.json"),
		Now:        func() time.Time { return testTime },
	}
}

// mustSync syncs store and fails the test on error.
func mustSync(t *testing.T, syncer Syncer, store string) Outcome {
	t.Helper()
	outcome, err := syncer.Store(store)
	if err != nil {
		t.Fatalf("sync %s: %v", store, err)
	}
	return outcome
}
