package storesync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixie6502/folio/internal/notebook"
)

// sparseMachine syncs notebooks one and two from a full machine, then clones
// the store sparse on machine c, which checks out neither, and returns c's
// store and an Env over c's home.
func sparseMachine(t *testing.T) (string, notebook.Env) {
	t.Helper()
	home := isolate(t)
	url := newRemote(t, home)
	full, a := machine(t, home, "a", url)
	for _, name := range []string{"one", "two"} {
		write(t, filepath.Join(full, name, notebook.NotebookMarker), "")
		write(t, filepath.Join(full, name, "note.md"), name+"\n")
	}
	mustSync(t, a, full)
	store, _ := machine(t, home, "c", url, "--filter=blob:none", "--sparse")
	return store, notebook.Env{Home: filepath.Join(home, "c")}
}

// wantRename fails unless git records note.md's rename from one notebook to
// the other.
func wantRename(t *testing.T, store, from, to string) {
	t.Helper()
	want := "R  " + from + "/note.md -> " + to + "/note.md"
	if status := run(t, store, "git", "status", "--porcelain"); !strings.Contains(status, want) {
		t.Fatalf("want %q, got:\n%s", want, status)
	}
}

func TestMoveIntoANotebookOutsideTheSparseCheckout(t *testing.T) {
	store, env := sparseMachine(t)
	owner := filepath.Dir(store)
	mkdir(t, filepath.Join(owner, "one"))
	if err := CheckOut(filepath.Join(store, "one")); err != nil {
		t.Fatal(err)
	}

	if _, err := env.Move(filepath.Join(owner, "one"), filepath.Join(owner, "renamed"), CheckOut); err != nil {
		t.Fatal(err)
	}
	wantRename(t, store, "one", "renamed")
}

func TestMoveANotebookMissingFromTheSparseCheckout(t *testing.T) {
	store, env := sparseMachine(t)
	owner := filepath.Dir(store)
	renamed := mkdir(t, filepath.Join(owner, "renamed")) // two/ was moved by hand

	if _, err := env.Move(filepath.Join(owner, "two"), renamed, CheckOut); err != nil {
		t.Fatal(err)
	}
	wantRename(t, store, "two", "renamed")
	if _, err := os.Stat(filepath.Join(store, "one")); err == nil {
		t.Fatal("checked out a notebook the move did not touch")
	}
}

func TestMoveRefusesANotebookOutsideTheSparseCheckout(t *testing.T) {
	store, env := sparseMachine(t)
	owner := filepath.Dir(store)
	mkdir(t, filepath.Join(owner, "one"))
	if err := CheckOut(filepath.Join(store, "one")); err != nil {
		t.Fatal(err)
	}

	_, err := env.Move(filepath.Join(owner, "one"), filepath.Join(owner, "two"), CheckOut)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("want an already exists refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(owner, "one")); err != nil {
		t.Fatal("a refused move moved the folder")
	}
}

func TestCheckOutAcceptsANameLikeAnOption(t *testing.T) {
	store, _ := sparseMachine(t)
	if err := CheckOut(filepath.Join(store, "-draft")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckOutLeavesAFullStoreAlone(t *testing.T) {
	home := isolate(t)
	store, _ := machine(t, home, "a", newRemote(t, home))
	if err := CheckOut(filepath.Join(store, "one")); err != nil {
		t.Fatal(err)
	}
	if sparse := run(t, store, "git", "config", "--bool", "--default", "false", "core.sparseCheckout"); sparse != "false" {
		t.Fatalf("a full store became sparse: %s", sparse)
	}
}
