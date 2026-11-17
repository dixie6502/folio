package storesync

import (
	"crypto/rand"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dixie6502/folio/internal/testutil"
)

func TestSyncPushesAndPulls(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	storeA, a := machine(t, home, "a", url)
	storeB, b := machine(t, home, "b", url)
	write(t, filepath.Join(storeA, "project", "TODO.md"), "from a\n")

	if outcome := mustSync(t, a, storeA); !outcome.Committed || !outcome.Pushed {
		t.Fatalf("a: %s", outcome)
	}
	if outcome := mustSync(t, b, storeB); !outcome.Merged {
		t.Fatalf("b: %s", outcome)
	}
	if got := read(t, filepath.Join(storeB, "project", "TODO.md")); got != "from a\n" {
		t.Fatalf("b holds %q", got)
	}
	if outcome := mustSync(t, b, storeB); outcome.String() != "up to date" {
		t.Fatalf("second sync of b: %s", outcome)
	}
}

func TestSyncMergesChangesToDifferentNotes(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	storeA, a := machine(t, home, "a", url)
	storeB, b := machine(t, home, "b", url)
	write(t, filepath.Join(storeA, "project", "a.md"), "a\n")
	write(t, filepath.Join(storeB, "project", "b.md"), "b\n")

	mustSync(t, a, storeA)
	if outcome := mustSync(t, b, storeB); !outcome.Merged || !outcome.Pushed || len(outcome.Conflicts) != 0 {
		t.Fatalf("b: %s", outcome)
	}
	mustSync(t, a, storeA)
	for _, store := range []string{storeA, storeB} {
		for _, note := range []string{"a.md", "b.md"} {
			read(t, filepath.Join(store, "project", note))
		}
	}
}

func TestSyncKeepsBothSidesOfAConflict(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	storeA, a := machine(t, home, "a", url)
	storeB, b := machine(t, home, "b", url)
	todo := filepath.Join("project", "TODO.md")
	write(t, filepath.Join(storeA, todo), "base\n")
	mustSync(t, a, storeA)
	mustSync(t, b, storeB)
	write(t, filepath.Join(storeA, todo), "from a\n")
	write(t, filepath.Join(storeB, todo), "from b\n")

	mustSync(t, a, storeA)
	outcome := mustSync(t, b, storeB)
	conflict := filepath.Join("project", "TODO.conflict-b-20261117T080000Z.md")
	if len(outcome.Conflicts) != 1 || outcome.Conflicts[0] != conflict || !outcome.Pushed {
		t.Fatalf("b: %s", outcome)
	}
	mustSync(t, a, storeA)
	for _, store := range []string{storeA, storeB} {
		if got := read(t, filepath.Join(store, todo)); got != "from a\n" {
			t.Fatalf("%s: TODO.md holds %q, want the upstream's version", store, got)
		}
		if got := read(t, filepath.Join(store, conflict)); got != "from b\n" {
			t.Fatalf("%s: conflict file holds %q, want b's version", store, got)
		}
	}
}

func TestSyncKeepsAChangeOverADeletion(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	storeA, a := machine(t, home, "a", url)
	storeB, b := machine(t, home, "b", url)
	note := filepath.Join(storeA, "project", "note.md")
	write(t, note, "base\n")
	mustSync(t, a, storeA)
	mustSync(t, b, storeB)
	if err := os.Remove(note); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(storeB, "project", "note.md"), "changed by b\n")

	mustSync(t, a, storeA)
	mustSync(t, b, storeB)
	mustSync(t, a, storeA)
	if got := read(t, note); got != "changed by b\n" {
		t.Fatalf("a holds %q, want b's change", got)
	}
}

func TestSyncRetriesWhenAFileChangedDuringTheMerge(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	storeA, a := machine(t, home, "a", url)
	storeB, b := machine(t, home, "b", url)
	write(t, filepath.Join(storeA, "project", "note.md"), "base\n")
	mustSync(t, a, storeA)
	mustSync(t, b, storeB)
	write(t, filepath.Join(storeA, "project", "note.md"), "from a\n")
	mustSync(t, a, storeA)
	run(t, storeB, "git", "fetch", "-q")
	write(t, filepath.Join(storeB, "project", "note.md"), "being edited on b\n") // after b's commit step

	result, err := b.integrate(storeB, filepath.Join(storeB, ".git"))
	if err != nil || !result.retry || result.merged {
		t.Fatalf("got %+v, %v; want a retry", result, err)
	}
	if got := read(t, filepath.Join(storeB, "project", "note.md")); got != "being edited on b\n" {
		t.Fatalf("the edit was overwritten: %q", got)
	}
}

func TestSyncBlocksASecret(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	store, syncer := machine(t, home, "a", url)
	write(t, filepath.Join(store, "project", "leak.md"), "token "+fakeGitHubToken(t)+"\n")

	_, err := syncer.Store(store)
	if err == nil || !strings.Contains(err.Error(), "gitleaks found a secret") {
		t.Fatalf("got %v, want the push blocked", err)
	}
	if run(t, store, "git", "rev-list", "--count", "@{upstream}..HEAD") != "1" {
		t.Fatal("the commit should stay local")
	}
}

func TestSyncWithoutGitleaksDoesNotPush(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	store, syncer := machine(t, home, "a", url)
	syncer.Gitleaks = ""
	write(t, filepath.Join(store, "project", "note.md"), "note\n")

	if _, err := syncer.Store(store); err == nil || !strings.Contains(err.Error(), "gitleaks not installed") {
		t.Fatalf("got %v", err)
	}
}

func TestSyncGrowsTheSparseCheckout(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	full, a := machine(t, home, "a", url)
	for _, name := range []string{"one", "two", ".template"} {
		write(t, filepath.Join(full, name, "note.md"), name+"\n")
	}
	mustSync(t, a, full)
	store, c := machine(t, home, "c", url, "--filter=blob:none", "--sparse")
	owner := filepath.Dir(store)
	present := func(name string) bool {
		_, err := os.Stat(filepath.Join(store, name, "note.md"))
		return err == nil
	}

	if err := os.Mkdir(filepath.Join(owner, "one"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustSync(t, c, store)
	if !present("one") || present("two") || !present(".template") {
		t.Fatalf("want one and .template only, got one=%v two=%v", present("one"), present("two"))
	}

	if err := os.Mkdir(filepath.Join(owner, "two"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(owner, "one")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(store, "three", ".notebook"), "") // a notebook made here by folio init
	write(t, filepath.Join(store, "three", "note.md"), "three\n")
	if outcome := mustSync(t, c, store); !outcome.Pushed {
		t.Fatalf("c: %s", outcome)
	}
	if !present("one") || !present("two") {
		t.Fatal("want one kept (never removed) and two added")
	}
	mustSync(t, a, full)
	read(t, filepath.Join(full, "three", "note.md"))
}

func TestSyncSkipsAStoreItCannotSync(t *testing.T) {
	home := isolate(t)
	plain := filepath.Join(home, "plain")
	noRemote := filepath.Join(home, "noremote")
	for _, dir := range []string{plain, noRemote} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	run(t, noRemote, "git", "init", "-q")
	syncer := Syncer{Host: "a", StatusFile: filepath.Join(home, "status.json"), Now: time.Now}

	for dir, reason := range map[string]string{plain: "not a git repository", noRemote: "no remote"} {
		if outcome, err := syncer.Store(dir); err != nil || outcome.Skipped != reason {
			t.Fatalf("%s: got %s, %v; want skipped: %s", dir, outcome, err, reason)
		}
	}
	if _, err := os.Stat(syncer.StatusFile); err == nil {
		t.Fatal("a skipped store must not be recorded")
	}
}

func TestLockWaitsThenFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	unlock, err := lock(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock(path, 300*time.Millisecond); err == nil {
		t.Fatal("a second lock must fail while the first is held")
	}
	unlock()
	again, err := lock(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

// fakeGitHubToken returns a random string shaped like a GitHub token, built
// at run time so no secret sits in the source.
func fakeGitHubToken(t *testing.T) string {
	t.Helper()
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	token := []byte("ghp_")
	for range 36 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			t.Fatal(err)
		}
		token = append(token, alphabet[n.Int64()])
	}
	return string(token)
}

func TestSyncSettlesAFileAgainstAFolder(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	storeA, a := machine(t, home, "a", url)
	storeB, b := machine(t, home, "b", url)
	write(t, filepath.Join(storeA, "project", "ideas"), "a file on a\n")
	write(t, filepath.Join(storeB, "project", "ideas", "one.md"), "a folder on b\n")

	mustSync(t, a, storeA)
	if outcome := mustSync(t, b, storeB); !outcome.Merged || !outcome.Pushed {
		t.Fatalf("b: %s", outcome)
	}
	mustSync(t, a, storeA)
	if got := read(t, filepath.Join(storeA, "project", "ideas", "one.md")); got != "a folder on b\n" {
		t.Fatalf("a lost b's folder: %q", got)
	}
	matches, _ := filepath.Glob(filepath.Join(storeA, "project", "ideas~*"))
	if len(matches) != 1 || read(t, matches[0]) != "a file on a\n" {
		t.Fatalf("a's file should be kept beside the folder, got %q", matches)
	}
}

func TestSyncSkipsAStoreAnotherSyncHolds(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	store, syncer := machine(t, home, "a", url)
	defer func(wait time.Duration) { lockWait = wait }(lockWait)
	lockWait = 500 * time.Millisecond
	unlock, err := lock(filepath.Join(store, ".git", "folio-sync.lock"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	start := time.Now()
	outcome, err := syncer.Store(store)
	if err != nil || outcome.Skipped != "another sync holds it" {
		t.Fatalf("got %s, %v", outcome, err)
	}
	if elapsed := time.Since(start); elapsed < lockWait {
		t.Fatalf("gave up after %s, want a wait of %s", elapsed, lockWait)
	}
	if _, err := os.Stat(syncer.StatusFile); err == nil {
		t.Fatal("a held store must not be recorded as a failure")
	}
}

func TestSyncLeavesAnEmptyStore(t *testing.T) {
	home := isolate(t)
	store := testutil.Mkdir(t, filepath.Join(home, "GitHub", "owner", "notebooks"))
	run(t, store, "git", "init", "-q", "-b", "main")
	remote := filepath.Join(home, "remote.git")
	run(t, home, "git", "init", "-q", "--bare", "-b", "main", remote)
	run(t, store, "git", "remote", "add", "origin", remote)
	syncer := Syncer{Host: "a", StatusFile: filepath.Join(home, "status.json"), Now: time.Now}

	if outcome, err := syncer.Store(store); err != nil || outcome.Changed() {
		t.Fatalf("got %s, %v; want nothing done", outcome, err)
	}
	problems, err := syncer.Check([]string{store})
	if err != nil || !strings.Contains(strings.Join(problems, "\n"), "no upstream branch yet") {
		t.Fatalf("got %q, %v", problems, err)
	}
}
