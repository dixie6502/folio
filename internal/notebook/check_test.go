package notebook

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCleanStore(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	notebookAt(t, filepath.Join(storeDir, "project"))
	mkdir(t, filepath.Join(owner, "project"))
	write(t, filepath.Join(storeDir, "README.md"), "a file at the store root is fine\n")

	if problems := env.Check(); len(problems) != 0 {
		t.Fatalf("want no problems, got %q", problems)
	}
}

func TestCheckReportsProblems(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	notebook := notebookAt(t, filepath.Join(storeDir, "gone"))
	notebookAt(t, filepath.Join(notebook, "inner"))
	mkdir(t, filepath.Join(storeDir, "stray"))
	write(t, filepath.Join(env.Home, "GitHub", "other", "elsewhere", StoreMarker), "")

	got := strings.Join(env.Check(), "\n")
	for _, want := range []string{
		"gone: orphan",
		"folio forget " + filepath.Join(owner, "gone"),
		"inner: notebook inside the notebook",
		"stray: stray folder",
		"store marker outside a notebooks/ folder",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}
