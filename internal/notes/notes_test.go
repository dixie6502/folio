package notes

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dixie6502/folio/internal/testutil"
)

// names returns the names of list.
func names(list []Note) []string {
	var out []string
	for _, note := range list {
		out = append(out, note.Name)
	}
	return out
}

func TestListSkipsHiddenAndUnderscoredAndReadsTags(t *testing.T) {
	notebook := t.TempDir()
	testutil.Write(t, filepath.Join(notebook, "b.md"), "# Title\nsee #moc and #idea, url x.org/a#frag #moc\n")
	testutil.Write(t, filepath.Join(notebook, "a", "deep.md"), "#first line tag\n")
	testutil.Write(t, filepath.Join(notebook, "tab\tname.md"), "")
	for _, hidden := range []string{"_draft.md", ".hidden.md", "_folder/x.md", ".git/y.md", "notes.txt"} {
		testutil.Write(t, filepath.Join(notebook, hidden), "")
	}

	list, err := List(notebook, "pre/")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(list), []string{"pre/a/deep", "pre/b", "pre/tab\tname"}; !slices.Equal(got, want) {
		t.Fatalf("names %q, want %q", got, want)
	}
	if got, want := list[1].Tags, []string{"#moc", "#idea"}; !slices.Equal(got, want) {
		t.Fatalf("tags %q, want %q (no heading, no URL fragment, each once)", got, want)
	}
	if got := list[0].Tags; !slices.Equal(got, []string{"#first"}) {
		t.Fatalf("tags %q, want a tag at the start of a line", got)
	}
}

func TestFilter(t *testing.T) {
	list := []Note{
		{Name: "work/meeting-notes"},
		{Name: "ideas", Tags: []string{"#moc"}},
		{Name: "Notes"},
		{Name: "mocha recipe"},
	}
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"", []string{"work/meeting-notes", "ideas", "Notes", "mocha recipe"}},
		{"notes", []string{"Notes", "work/meeting-notes"}}, // a name part's start, then earlier
		{"Notes", []string{"Notes"}},                       // upper case: case matters
		{"#moc", []string{"ideas"}},                        // tags are searched
		{"moc", []string{"mocha recipe", "ideas"}},
		{"meet work", []string{"work/meeting-notes"}}, // every word, any order
		{"mtg", nil}, // exact, not fuzzy
	} {
		if got := names(Filter(list, test.query)); !slices.Equal(got, test.want) {
			t.Errorf("Filter(%q) = %q, want %q", test.query, got, test.want)
		}
	}
}

func TestCreate(t *testing.T) {
	notebook := t.TempDir()
	existing := []Note{{Path: filepath.Join(notebook, "Ideas.md"), Name: "Ideas"}}

	for typed, want := range map[string]string{
		"idea":         filepath.Join(notebook, "idea.md"),
		"work/idea.md": filepath.Join(notebook, "work", "idea.md"),
		"ideas":        filepath.Join(notebook, "Ideas.md"), // reuses the note differing in case
	} {
		got, err := Create(notebook, typed, existing)
		if err != nil || got != want {
			t.Errorf("Create(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
	if info, err := os.Stat(filepath.Join(notebook, "work")); err != nil || !info.IsDir() {
		t.Error("Create must make the note's folder")
	}
	for _, typed := range []string{"_draft", ".hidden", "a//b", "../escape", "/abs", "dir/"} {
		if _, err := Create(notebook, typed, nil); err == nil {
			t.Errorf("Create(%q) accepted a path the listing would never show", typed)
		}
	}
	if _, err := Create("", "idea", nil); err == nil {
		t.Error("Create without a notebook must fail")
	}
}

func TestTags(t *testing.T) {
	got := tags([]byte("#start mid#no\t#tab\n#2no #a-b/c_d #a-b/c_d ## #"))
	if want := []string{"#start", "#tab", "#a-b/c_d"}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestListSkipsWhatItCannotRead(t *testing.T) {
	notebook := t.TempDir()
	testutil.Write(t, filepath.Join(notebook, "good.md"), "#ok\n")
	testutil.Mkdir(t, filepath.Join(notebook, "folder"))
	for link, target := range map[string]string{"broken.md": "nowhere", "dir.md": "folder"} {
		if err := os.Symlink(target, filepath.Join(notebook, link)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List(notebook, "")
	if err != nil || len(list) != 1 || list[0].Name != "good" {
		t.Fatalf("got %+v, %v", list, err)
	}
}
