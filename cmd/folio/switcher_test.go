package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixie6502/folio/internal/picker"
	"github.com/dixie6502/folio/internal/testutil"
)

func TestParseSwitchArgs(t *testing.T) {
	for _, test := range []struct {
		args  []string
		want  switchArgs
		error string
	}{
		{nil, switchArgs{mode: picker.CreateOnCtrlN}, ""},
		{[]string{"idea"}, switchArgs{mode: picker.CreateOnCtrlN, query: "idea"}, ""},
		{[]string{"-c", "idea"}, switchArgs{mode: picker.CreateOnEnter, query: "idea"}, ""},
		{[]string{"--", "init"}, switchArgs{mode: picker.CreateOnCtrlN, query: "init"}, ""},
		{[]string{"-c", "--", "-draft"}, switchArgs{mode: picker.CreateOnEnter, query: "-draft"}, ""},
		{[]string{"--all", "x"}, switchArgs{all: true, mode: picker.OpenOnly, query: "x"}, ""},
		{[]string{"--filter", "x"}, switchArgs{filter: true, mode: picker.OpenOnly, query: "x"}, ""},
		{[]string{"--all", "-c"}, switchArgs{}, "never create"},
		{[]string{"a", "b"}, switchArgs{}, "unexpected arg: b"},
		{[]string{"a", "--", "b"}, switchArgs{}, "unexpected arg: b"},
		{[]string{"-x"}, switchArgs{}, "unknown flag: -x"},
	} {
		got, err := parseSwitchArgs(test.args)
		switch {
		case test.error != "":
			if err == nil || !strings.Contains(err.Error(), test.error) {
				t.Errorf("%q: got %v, want an error with %q", test.args, err, test.error)
			}
		case err != nil || got != test.want:
			t.Errorf("%q: got %+v, %v; want %+v", test.args, got, err, test.want)
		}
	}
}

func TestFilterPrintsTheCurrentNotebooksMatches(t *testing.T) {
	topic := filepath.Join(tempHome(t), "scratch", "topic")
	testutil.Write(t, filepath.Join(topic, "notes", ".notebook"), "")
	testutil.Write(t, filepath.Join(topic, "notes", "work", "meeting.md"), "#moc\n")
	testutil.Write(t, filepath.Join(topic, "notes", "ideas.md"), "")

	if code, stdout, _ := folio(t, topic, "", "--filter", "#moc"); code != 0 || stdout != "work/meeting\n" {
		t.Fatalf("filter: %d %q", code, stdout)
	}
	if code, stdout, _ := folio(t, topic, "", "--filter"); code != 0 || stdout != "ideas\nwork/meeting\n" {
		t.Fatalf("filter, no query: %d %q", code, stdout)
	}
	if code, _, stderr := folio(t, filepath.Dir(topic), "", "--filter", "x"); code != 1 || !strings.Contains(stderr, "folio init") {
		t.Fatalf("no notebook: %d %q", code, stderr)
	}
}

func TestEveryStoreNoteIsNamedByOwnerAndNotebook(t *testing.T) {
	home := tempHome(t)
	store := filepath.Join(home, "GitHub", "owner", "notebooks")
	testutil.Write(t, filepath.Join(store, ".folio"), "")
	testutil.Write(t, filepath.Join(store, "repo", ".notebook"), "")
	testutil.Write(t, filepath.Join(store, "repo", "TODO.md"), "")

	env := mustEnv(t)
	list, err := everyStoreNote(env)
	if err != nil || len(list) != 1 || list[0].Name != "owner/repo/TODO" {
		t.Fatalf("got %+v, %v", list, err)
	}
}

func TestEveryStoreNoteNamesTheHostOfAnOwnerOnTwoHosts(t *testing.T) {
	home := tempHome(t)
	for _, host := range []string{"GitHub", "Forgejo"} {
		store := filepath.Join(home, host, "owner", "notebooks")
		testutil.Write(t, filepath.Join(store, ".folio"), "")
		testutil.Write(t, filepath.Join(store, "repo", ".notebook"), "")
		testutil.Write(t, filepath.Join(store, "repo", "TODO.md"), "")
	}

	list, err := everyStoreNote(mustEnv(t))
	if err != nil || len(list) != 2 || list[0].Name == list[1].Name || !strings.HasSuffix(list[0].Name, "/owner/repo/TODO") {
		t.Fatalf("got %+v, %v", list, err)
	}
}
