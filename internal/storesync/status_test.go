package storesync

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dixie6502/folio/internal/testutil"
)

func TestStatusRecordsAndCheckReports(t *testing.T) {
	home := isolate(t)
	url := newRemote(t, home)
	store, syncer := machine(t, home, "a", url)

	checkAt := func(at time.Time) string {
		t.Helper()
		checker := syncer
		checker.Now = func() time.Time { return at }
		problems, err := checker.Check([]string{store})
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(problems, "\n")
	}

	if got := checkAt(testTime); !strings.Contains(got, "never synced") {
		t.Fatalf("before any sync: %q", got)
	}
	mustSync(t, syncer, store)
	all, err := loadStatus(syncer.StatusFile)
	if err != nil || !all[store].LastSuccess.Equal(testTime) {
		t.Fatalf("status %v, %v", all, err)
	}
	if got := checkAt(testTime.Add(time.Minute)); got != "" {
		t.Fatalf("fresh sync: %q", got)
	}
	if got := checkAt(testTime.Add(2 * time.Hour)); !strings.Contains(got, "over an hour ago") {
		t.Fatalf("stale sync: %q", got)
	}

	later := syncer
	later.Now = func() time.Time { return testTime.Add(time.Minute) }
	if err := later.record(store, errors.New("push failed")); err != nil {
		t.Fatal(err)
	}
	if got := checkAt(testTime.Add(2 * time.Minute)); !strings.Contains(got, "last sync failed") || !strings.Contains(got, "push failed") {
		t.Fatalf("failed sync: %q", got)
	}
}

func TestCheckReportsStoresThatCannotSync(t *testing.T) {
	home := isolate(t)
	plain := testutil.Mkdir(t, filepath.Join(home, "plain"))
	noRemote := testutil.Mkdir(t, filepath.Join(home, "noremote"))
	run(t, noRemote, "git", "init", "-q")
	url := newRemote(t, home)
	unpushed, syncer := machine(t, home, "a", url)
	write(t, filepath.Join(unpushed, "project", "note.md"), "note\n")
	write(t, filepath.Join(unpushed, "project", "TODO.conflict-b-20261117T080000Z.md"), "b\n")
	run(t, unpushed, "git", "add", "-A")
	run(t, unpushed, "git", "commit", "-q", "-m", "local")

	problems, err := syncer.Check([]string{plain, noRemote, unpushed})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(problems, "\n")
	for _, want := range []string{
		plain + ": not a git repository, not synced",
		noRemote + ": no remote, not synced",
		unpushed + ": 1 unpushed commit(s)",
		"TODO.conflict-b-20261117T080000Z.md: sync conflict",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}

func TestDaemonTiming(t *testing.T) {
	changed := testTime
	if isQuiet(watch{status: "", changedAt: changed}, changed.Add(time.Hour)) {
		t.Error("a clean store is never due")
	}
	if isQuiet(watch{status: " M a.md", changedAt: changed}, changed.Add(quietPeriod-time.Second)) {
		t.Error("changes still settling are not due")
	}
	if !isQuiet(watch{status: " M a.md", changedAt: changed}, changed.Add(quietPeriod)) {
		t.Error("changes rested for the quiet period are due")
	}
	if hasResumed(changed, changed.Add(pollEvery)) || !hasResumed(changed, changed.Add(time.Hour)) {
		t.Error("only a wall-clock jump counts as a resume")
	}
}
