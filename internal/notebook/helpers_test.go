package notebook

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixie6502/folio/internal/testutil"
)

// Helpers shared with the other packages' tests.
var (
	run   = testutil.Run
	write = testutil.Write
	mkdir = testutil.Mkdir
)

// newTestEnv returns an Env over a fresh, isolated home.
func newTestEnv(t *testing.T) Env {
	t.Helper()
	testutil.Home(t)
	env, err := NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// repo creates a git repository at path with one commit and returns path.
func repo(t *testing.T, path string) string {
	t.Helper()
	mkdir(t, path)
	run(t, path, "git", "init", "-q", "-b", "main")
	write(t, filepath.Join(path, "README.md"), "project\n")
	run(t, path, "git", "add", "README.md")
	run(t, path, "git", "commit", "-q", "-m", "init")
	return path
}

// store creates a local store (no git) in folder parent and returns it.
func store(t *testing.T, parent string) string {
	t.Helper()
	dir := mkdir(t, filepath.Join(parent, StoreDir))
	write(t, filepath.Join(dir, StoreMarker), "")
	return dir
}

// syncedStore creates a store in parent cloned from a fresh bare remote, with
// its marker committed and pushed, and returns it.
func syncedStore(t *testing.T, parent string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	run(t, ".", "git", "init", "-q", "--bare", "-b", "main", remote)
	mkdir(t, parent)
	run(t, parent, "git", "clone", "-q", remote, StoreDir)
	dir := filepath.Join(parent, StoreDir)
	write(t, filepath.Join(dir, StoreMarker), "")
	run(t, dir, "git", "add", StoreMarker)
	run(t, dir, "git", "commit", "-q", "-m", "store")
	run(t, dir, "git", "push", "-q", "-u", "origin", "main")
	return dir
}

// notebookAt creates a notebook (its marker) at path and returns it.
func notebookAt(t *testing.T, path string) string {
	t.Helper()
	write(t, filepath.Join(path, NotebookMarker), "")
	return path
}

// wantPath fails the test unless got equals want and err is nil.
func wantPath(t *testing.T, got string, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// wantError fails the test unless err is set and mentions fragment.
func wantError(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("got error %v, want one mentioning %q", err, fragment)
	}
}

// yes and no answer every question.
func yes(string) bool { return true }
func no(string) bool  { return false }
