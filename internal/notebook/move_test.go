package notebook

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveFolderAndNotebookInOneStore(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := syncedStore(t, owner)
	oldFolder := repo(t, filepath.Join(owner, "old"))
	notebook := notebookAt(t, filepath.Join(storeDir, "old"))
	write(t, filepath.Join(notebook, "tracked.md"), "tracked\n")
	run(t, storeDir, "git", "add", "-A")
	run(t, storeDir, "git", "commit", "-q", "-m", "notes")
	write(t, filepath.Join(notebook, "untracked.md"), "untracked\n")
	newFolder := filepath.Join(owner, "new")

	done, err := env.Move(oldFolder, newFolder)
	if err != nil || len(done) != 2 {
		t.Fatalf("got %v, %v", done, err)
	}
	moved := filepath.Join(storeDir, "new")
	for _, path := range []string{filepath.Join(newFolder, "README.md"), filepath.Join(moved, "tracked.md"), filepath.Join(moved, "untracked.md")} {
		if !isFile(path) {
			t.Fatalf("%s missing after the move", path)
		}
	}
	if status := run(t, storeDir, "git", "status", "--porcelain"); !strings.Contains(status, "R  old/tracked.md -> new/tracked.md") {
		t.Fatalf("git did not record a rename:\n%s", status)
	}
}

func TestMoveNotebookAfterAManualMove(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	notebookAt(t, filepath.Join(storeDir, "old"))
	newFolder := mkdir(t, filepath.Join(owner, "new"))

	if _, err := env.Move(filepath.Join(owner, "old"), newFolder); err != nil {
		t.Fatal(err)
	}
	if !isNotebook(filepath.Join(storeDir, "new")) || exists(filepath.Join(storeDir, "old")) {
		t.Fatal("notebook not moved")
	}
}

func TestMoveAcrossStores(t *testing.T) {
	env := newTestEnv(t)
	from := filepath.Join(env.Home, "GitHub", "from")
	to := filepath.Join(env.Home, "GitHub", "to")
	notebookAt(t, filepath.Join(store(t, from), "project"))
	storeTo := store(t, to)
	oldFolder := mkdir(t, filepath.Join(from, "project"))

	if _, err := env.Move(oldFolder, filepath.Join(to, "project")); err != nil {
		t.Fatal(err)
	}
	if !isNotebook(filepath.Join(storeTo, "project")) || exists(filepath.Join(from, StoreDir, "project")) {
		t.Fatal("notebook not moved across stores")
	}
}

func TestMoveTrackedSubfolderWithGitMv(t *testing.T) {
	env := newTestEnv(t)
	project := repo(t, filepath.Join(env.Home, "GitHub", "owner", "project"))
	write(t, filepath.Join(project, "docs", "a.md"), "a\n")
	run(t, project, "git", "add", "docs")
	run(t, project, "git", "commit", "-q", "-m", "docs")

	if _, err := env.Move(filepath.Join(project, "docs"), filepath.Join(project, "guide")); err != nil {
		t.Fatal(err)
	}
	if status := run(t, project, "git", "status", "--porcelain"); !strings.Contains(status, "R  docs/a.md -> guide/a.md") {
		t.Fatalf("git did not record a rename:\n%s", status)
	}
}

func TestMoveCarriesALocalNotebook(t *testing.T) {
	env := newTestEnv(t)
	scratch := filepath.Join(env.Home, "scratch")
	notebookAt(t, filepath.Join(scratch, "old", LocalDir))

	if _, err := env.Move(filepath.Join(scratch, "old"), filepath.Join(scratch, "new")); err != nil {
		t.Fatal(err)
	}
	if !isNotebook(filepath.Join(scratch, "new", LocalDir)) {
		t.Fatal("local notebook did not travel with its folder")
	}
}

func TestMoveRefusesWithoutChangingAnything(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	notebook := notebookAt(t, filepath.Join(store(t, owner), "project"))
	project := mkdir(t, filepath.Join(owner, "project"))
	elsewhere := mkdir(t, filepath.Join(env.Home, "GitHub", "nostore"))
	taken := mkdir(t, filepath.Join(owner, "taken"))

	_, err := env.Move(project, filepath.Join(elsewhere, "project"))
	wantError(t, err, "no notebooks/")
	_, err = env.Move(project, taken)
	wantError(t, err, "already exists")
	_, err = env.Move(filepath.Join(owner, "missing"), filepath.Join(owner, "missing2"))
	wantError(t, err, "neither")
	_, err = env.Move(filepath.Join(owner, "missing"), taken)
	wantError(t, err, "no notebook to move")
	if !isDir(project) || !isNotebook(notebook) {
		t.Fatal("a refused move changed something")
	}
}

func TestForget(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	orphan := notebookAt(t, filepath.Join(storeDir, "gone"))
	write(t, filepath.Join(orphan, "a.md"), "a\n")
	notebookAt(t, filepath.Join(storeDir, "here"))
	here := mkdir(t, filepath.Join(owner, "here"))
	gone := filepath.Join(owner, "gone")

	_, err := env.Forget(here, yes)
	wantError(t, err, "only an orphan")
	_, err = env.Forget(filepath.Join(owner, "never"), yes)
	wantError(t, err, "has no notebook")
	var asked string
	_, err = env.Forget(gone, func(question string) bool { asked = question; return false })
	wantError(t, err, "kept")
	if !strings.Contains(asked, "1 note(s)") || !isNotebook(orphan) {
		t.Fatalf("question %q, or the notebook was not kept", asked)
	}
	if _, err := env.Forget(gone, yes); err != nil || exists(orphan) {
		t.Fatalf("forget failed: %v", err)
	}
}
