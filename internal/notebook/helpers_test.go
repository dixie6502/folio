package notebook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newTestEnv returns an Env over a fresh temporary home, with git isolated
// from the user's configuration.
func newTestEnv(t *testing.T) Env {
	t.Helper()
	clearGitEnv(t)
	home := physical(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
	env, err := NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// clearGitEnv unsets every inherited GIT_* variable (a git hook exports
// GIT_INDEX_FILE and others, which would redirect the tests' git commands),
// restoring them when the test ends.
func clearGitEnv(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		if key, _, _ := strings.Cut(entry, "="); strings.HasPrefix(key, "GIT_") {
			t.Setenv(key, "")
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// run runs a command in dir and fails the test on error.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// mkdir creates the folder path (and its parents) and returns it.
func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// write creates the file path with content, making its parents.
func write(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
