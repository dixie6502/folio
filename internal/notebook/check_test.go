package notebook

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCleanStore(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := syncedStore(t, owner)
	notebookAt(t, filepath.Join(storeDir, "project"))
	mkdir(t, filepath.Join(owner, "project"))
	write(t, filepath.Join(storeDir, "README.md"), "a file at the store root is fine\n")
	run(t, storeDir, "git", "add", "-A")
	run(t, storeDir, "git", "commit", "-q", "-m", "notes")
	run(t, storeDir, "git", "push", "-q")

	if problems := env.Check(); len(problems) != 0 {
		t.Fatalf("want no problems, got %q", problems)
	}
}

func TestCheckReportsProblems(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	synced := syncedStore(t, owner)
	notebook := notebookAt(t, filepath.Join(synced, "gone"))
	notebookAt(t, filepath.Join(notebook, "inner"))
	write(t, filepath.Join(notebook, "TODO.conflict-host-2026.md"), "")
	mkdir(t, filepath.Join(synced, "stray"))
	run(t, synced, "git", "add", "-A")
	run(t, synced, "git", "commit", "-q", "-m", "unpushed")
	store(t, filepath.Join(env.Home, "GitHub", "local"))
	write(t, filepath.Join(env.Home, "GitHub", "other", "elsewhere", StoreMarker), "")

	got := strings.Join(env.Check(), "\n")
	for _, want := range []string{
		"gone: orphan",
		"folio forget " + filepath.Join(owner, "gone"),
		"inner: notebook inside the notebook",
		"TODO.conflict-host-2026.md: sync conflict",
		"stray: stray folder",
		"1 unpushed commit(s)",
		filepath.Join("local", StoreDir) + ": not a git repository",
		"store marker outside a notebooks/ folder",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}

func TestCheckReportsNoRemoteAndNoUpstream(t *testing.T) {
	env := newTestEnv(t)
	noRemote := store(t, filepath.Join(env.Home, "GitHub", "a"))
	run(t, noRemote, "git", "init", "-q")
	noUpstream := store(t, filepath.Join(env.Home, "GitHub", "b"))
	run(t, noUpstream, "git", "init", "-q")
	run(t, noUpstream, "git", "remote", "add", "origin", filepath.Join(env.Home, "nowhere.git"))

	got := strings.Join(env.Check(), "\n")
	for _, want := range []string{noRemote + ": no remote", noUpstream + ": no upstream branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}
