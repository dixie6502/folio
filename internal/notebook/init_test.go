package notebook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitInStoreWithGlobalTemplate(t *testing.T) {
	env := newTestEnv(t)
	write(t, filepath.Join(env.Template, ".wikijump"), "")
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	project := repo(t, filepath.Join(owner, "project"))

	got, err := env.Init(project, false, nil)
	want := filepath.Join(storeDir, "project")
	wantPath(t, got, err, want)
	for _, file := range []string{NotebookMarker, ".wikijump"} {
		if !isFile(filepath.Join(want, file)) {
			t.Fatalf("%s missing from the new notebook", file)
		}
	}
	got, err = env.Init(mkdir(t, filepath.Join(project, "src")), false, nil) // idempotent, from a subfolder
	wantPath(t, got, err, want)
}

func TestInitPrefersTheStoreTemplate(t *testing.T) {
	env := newTestEnv(t)
	write(t, filepath.Join(env.Template, "global.md"), "")
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	write(t, filepath.Join(storeDir, TemplateDir, "sub", "store.md"), "store\n")
	project := repo(t, filepath.Join(owner, "project"))

	notebook, err := env.Init(project, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !isFile(filepath.Join(notebook, "sub", "store.md")) || exists(filepath.Join(notebook, "global.md")) {
		t.Fatal("want the store template only")
	}
}

func TestInitWithoutStoreNeedsLocalOrYes(t *testing.T) {
	env := newTestEnv(t)
	topic := mkdir(t, filepath.Join(env.Home, "scratch", "topic"))
	want := filepath.Join(topic, LocalDir)

	_, err := env.Init(topic, false, nil)
	wantError(t, err, "folio init --local")
	_, err = env.Init(topic, false, no)
	wantError(t, err, "no notebook created")
	if exists(want) {
		t.Fatal("a refused init created the notebook")
	}
	got, err := env.Init(topic, false, yes)
	wantPath(t, got, err, want)
}

func TestInitLocal(t *testing.T) {
	env := newTestEnv(t)
	write(t, filepath.Join(env.Template, ".wikijump"), "")
	topic := mkdir(t, filepath.Join(env.Home, "scratch", "topic"))

	got, err := env.Init(topic, true, nil)
	wantPath(t, got, err, filepath.Join(topic, LocalDir))
	if !isFile(filepath.Join(got, ".wikijump")) {
		t.Fatal("template missing from the local notebook")
	}
}

func TestInitAsksAboutARepositoryWithoutStore(t *testing.T) {
	env := newTestEnv(t)
	project := repo(t, filepath.Join(env.Home, "GitHub", "owner", "project"))
	var asked string

	got, err := env.Init(project, false, func(question string) bool { asked = question; return true })
	wantPath(t, got, err, filepath.Join(project, LocalDir))
	if !strings.Contains(asked, "inside the repository") {
		t.Fatalf("question %q does not warn about the repository", asked)
	}
}

func TestInitRefuses(t *testing.T) {
	env := newTestEnv(t)
	owner := filepath.Join(env.Home, "GitHub", "owner")
	storeDir := store(t, owner)
	mkdir(t, filepath.Join(storeDir, "taken"))
	taken := mkdir(t, filepath.Join(owner, "taken"))

	_, err := env.Init(env.Home, true, nil)
	wantError(t, err, "~ itself")
	_, err = env.Init(storeDir, true, nil)
	wantError(t, err, "inside the store")
	_, err = env.Init(taken, false, nil)
	wantError(t, err, "not a notebook")
	if entries, _ := os.ReadDir(storeDir); len(entries) != 2 {
		t.Fatalf("store holds %d entries, want the marker and taken/ only", len(entries))
	}
}
