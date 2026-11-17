package notebook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindStoreNotebookFromRepoAndSubfolder(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	project := repo(t, filepath.Join(owner, "project"))
	want := notebookAt(t, filepath.Join(store(t, owner), "project"))

	got, err := env.Find(project)
	wantPath(t, got, err, want)
	got, err = env.Find(mkdir(t, filepath.Join(project, "src", "deep")))
	wantPath(t, got, err, want)
}

func TestFindSharesTheNotebookWithAWorktree(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	project := repo(t, filepath.Join(owner, "project"))
	want := notebookAt(t, filepath.Join(store(t, owner), "project"))
	inside := filepath.Join(project, ".claude", "worktrees", "feature")
	outside := filepath.Join(env.Home, "elsewhere", "feature")
	run(t, project, "git", "worktree", "add", "-q", "-b", "inside", inside)
	run(t, project, "git", "worktree", "add", "-q", "-b", "outside", outside)

	for _, worktree := range []string{inside, outside} {
		got, err := env.Find(worktree)
		wantPath(t, got, err, want)
	}
}

func TestFindLocalNotebookFromSubfolder(t *testing.T) {
	env := newTestEnv(t)
	topic := mkdir(t, filepath.Join(env.Home, "scratch", "topic"))
	want := notebookAt(t, filepath.Join(topic, LocalDir))

	got, err := env.Find(mkdir(t, filepath.Join(topic, "a", "b")))
	wantPath(t, got, err, want)
	got, err = env.Find(want)
	wantPath(t, got, err, want)
}

func TestFindInsideAStoreNotebook(t *testing.T) {
	env := newTestEnv(t)
	want := notebookAt(t, filepath.Join(store(t, filepath.Join(env.Home, "GitHub", "owner")), "project"))

	got, err := env.Find(mkdir(t, filepath.Join(want, "todo")))
	wantPath(t, got, err, want)
	_, err = env.Find(filepath.Dir(want))
	wantError(t, err, "not in a notebook")
}

func TestFindNoNotebook(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	store(t, owner)
	project := repo(t, filepath.Join(owner, "project"))
	notebookAt(t, filepath.Join(env.Home, LocalDir)) // ~/notes never serves as a default

	for _, dir := range []string{project, mkdir(t, filepath.Join(env.Home, "scratch")), env.Home} {
		if _, err := env.Find(dir); !errors.Is(err, ErrNoNotebook) {
			t.Fatalf("Find(%s): got %v, want ErrNoNotebook", dir, err)
		}
	}
}

func TestFindFollowsSymlinks(t *testing.T) {
	env := newTestEnv(t)
	topic := mkdir(t, filepath.Join(env.Home, "scratch", "topic"))
	want := notebookAt(t, filepath.Join(topic, LocalDir))
	link := filepath.Join(env.Home, "link")
	if err := os.Symlink(topic, link); err != nil {
		t.Fatal(err)
	}

	got, err := env.Find(link)
	wantPath(t, got, err, want)
}

func TestPhysicalKeepsAMissingTail(t *testing.T) {
	env := newTestEnv(t)
	link := filepath.Join(env.Home, "link")
	target := mkdir(t, filepath.Join(env.Home, "target"))
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	got := physical(filepath.Join(link, "missing", "file"))
	if want := filepath.Join(target, "missing", "file"); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestGitErrorCarriesMessage(t *testing.T) {
	env := newTestEnv(t)
	_, err := git(env.Home, "rev-parse", "--show-toplevel")
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("got %v, want git's message", err)
	}
}

func TestStoreOnlyInAnOwnerFolder(t *testing.T) {
	env := newTestEnv(t)
	scratch := filepath.Join(env.Home, "scratch")
	store(t, scratch) // ~/scratch/notebooks: one level too shallow
	topic := mkdir(t, filepath.Join(scratch, "topic"))
	store(t, filepath.Join(env.Home, ".hidden", "owner")) // never listed

	if _, err := env.Init(topic, false, nil); err == nil || !strings.Contains(err.Error(), "--local") {
		t.Fatalf("a shallow notebooks/ must not count as a store, got %v", err)
	}
	if stores, misplaced := env.Stores(); len(stores)+len(misplaced) != 0 {
		t.Fatalf("got stores %q, misplaced %q; want none", stores, misplaced)
	}
}
