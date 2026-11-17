package picker

import (
	"slices"
	"strings"
	"testing"

	"github.com/dixie6502/folio/internal/notes"
	"github.com/mattn/go-runewidth"
)

func TestDecoder(t *testing.T) {
	var d decoder
	got := d.feed([]byte("aé\x1b[A\x1b[B\x0e\r\x7f\x15\x17\x03\x1b[1;5C\x1bOA"))
	want := []Key{
		{Kind: KeyRune, Text: "a"}, {Kind: KeyRune, Text: "é"}, {Kind: KeyUp}, {Kind: KeyDown},
		{Kind: KeyCreate}, {Kind: KeyEnter}, {Kind: KeyBackspace}, {Kind: KeyClear},
		{Kind: KeyDeleteWord}, {Kind: KeyCancel}, {Kind: KeyUp}, // the Ctrl-Right sequence is skipped
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := d.feed([]byte("\x1b")); !slices.Equal(got, []Key{{Kind: KeyCancel}}) {
		t.Fatalf("a lone Esc: got %v", got)
	}
}

func TestDecoderAcrossReads(t *testing.T) {
	var d decoder
	e := []byte("é")
	if got := d.feed(e[:1]); len(got) != 0 {
		t.Fatalf("half a character decoded: %v", got)
	}
	if got := d.feed(e[1:]); !slices.Equal(got, []Key{{Kind: KeyRune, Text: "é"}}) {
		t.Fatalf("got %v", got)
	}
	if got := d.feed([]byte("\x1b[200~line one\nline")); len(got) != 0 {
		t.Fatalf("an unfinished paste decoded: %v", got)
	}
	got := d.feed([]byte(" two\x1b[20"))
	got = append(got, d.feed([]byte("1~x"))...)
	want := []Key{{Kind: KeyPaste, Text: "line one\nline two"}, {Kind: KeyRune, Text: "x"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// sample is a list of notes for the state tests.
var sample = []notes.Note{{Name: "alpha"}, {Name: "beta", Tags: []string{"#moc"}}, {Name: "gamma"}}

// press applies keys to s and returns whether it finished, and with what.
func press(s *state, keys ...Key) (bool, Result) {
	for _, key := range keys {
		if done, result := s.handle(key); done {
			return true, result
		}
	}
	return false, Result{}
}

// typed returns the keys typing text.
func typed(text string) []Key {
	var keys []Key
	for _, r := range text {
		keys = append(keys, Key{Kind: KeyRune, Text: string(r)})
	}
	return keys
}

func TestStateOpensTheSelectedMatch(t *testing.T) {
	s := newState(sample, CreateOnCtrlN, "")
	done, result := press(s, Key{Kind: KeyDown}, Key{Kind: KeyDown}, Key{Kind: KeyDown}, Key{Kind: KeyUp}, Key{Kind: KeyEnter})
	if !done || result.Action != Open || result.Note.Name != "beta" {
		t.Fatalf("got %v %+v", done, result)
	}
	s = newState(sample, CreateOnCtrlN, "")
	if done, result := press(s, append(typed("gam"), Key{Kind: KeyEnter})...); !done || result.Note.Name != "gamma" {
		t.Fatalf("typing filters: got %+v", result)
	}
}

func TestStateCreateKeysPerMode(t *testing.T) {
	keys := append(typed("new note"), Key{Kind: KeyEnter})
	if done, _ := press(newState(sample, CreateOnCtrlN, ""), keys...); done {
		t.Fatal("enter on no match must not create without -c")
	}
	if done, result := press(newState(sample, CreateOnEnter, ""), keys...); !done || result.Action != Create || result.Query != "new note" {
		t.Fatalf("-c: got %+v", result)
	}
	if done, result := press(newState(sample, CreateOnCtrlN, "alp"), Key{Kind: KeyCreate}); !done || result.Action != Create || result.Query != "alp" {
		t.Fatalf("ctrl-n creates even with a match: got %+v", result)
	}
	s := newState(sample, OpenOnly, "")
	if done, _ := press(s, Key{Kind: KeyCreate}); done || s.cursor != 1 {
		t.Fatal("ctrl-n is down where it does not create")
	}
	if done, _ := press(newState(sample, CreateOnCtrlN, ""), Key{Kind: KeyCreate}); done {
		t.Fatal("ctrl-n with nothing typed creates nothing")
	}
	if done, result := press(newState(sample, CreateOnCtrlN, ""), Key{Kind: KeyCancel}); !done || result.Action != Cancel {
		t.Fatal("esc cancels")
	}
}

func TestStateEditsTheQuery(t *testing.T) {
	s := newState(sample, CreateOnCtrlN, "")
	press(s, typed("one two")...)
	press(s, Key{Kind: KeyDeleteWord})
	if s.query != "one " {
		t.Fatalf("ctrl-w: %q", s.query)
	}
	press(s, Key{Kind: KeyBackspace}, Key{Kind: KeyPaste, Text: "x\ny"})
	if s.query != "onex y" {
		t.Fatalf("backspace then paste: %q", s.query)
	}
	press(s, Key{Kind: KeyClear})
	if s.query != "" || len(s.matches) != len(sample) {
		t.Fatalf("ctrl-u: %q, %d matches", s.query, len(s.matches))
	}
}

func TestLinesFitAndScroll(t *testing.T) {
	var many []notes.Note
	for _, name := range []string{"one", "two", "three", "four", "five", "a very long note name that goes on"} {
		many = append(many, notes.Note{Name: name, Tags: []string{"#tag"}})
	}
	s := newState(many, CreateOnCtrlN, "")
	press(s, Key{Kind: KeyDown}, Key{Kind: KeyDown}, Key{Kind: KeyDown}, Key{Kind: KeyDown})
	lines, column := s.lines(20, 4)
	if len(lines) != 4 || column != 2 {
		t.Fatalf("got %d lines, cursor at %d", len(lines), column)
	}
	if !strings.Contains(lines[3], "five") || !strings.Contains(lines[3], "> ") {
		t.Fatalf("the selected match must be shown, marked: %q", lines)
	}
	for _, line := range lines {
		if width := runewidth.StringWidth(stripColors(line)); width > 20 {
			t.Fatalf("line %q is %d columns wide", line, width)
		}
	}
	if s.height(100) != len(many)+2 || s.height(10) != 4 || s.height(2) != 3 {
		t.Fatalf("heights %d %d %d", s.height(100), s.height(10), s.height(2))
	}
}

// stripColors removes the color sequences from line.
func stripColors(line string) string {
	var out strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == '\x1b' {
			for i < len(line) && line[i] != 'm' {
				i++
			}
			continue
		}
		out.WriteByte(line[i])
	}
	return out.String()
}

func TestPasteAndCreateAreSanitized(t *testing.T) {
	s := newState(sample, CreateOnCtrlN, "")
	press(s, Key{Kind: KeyPaste, Text: "a\x1b[2Jb\x07 \tc"})
	if s.query != "a[2Jb c" {
		t.Fatalf("control characters must not reach the query: %q", s.query)
	}
	if done, result := press(newState(sample, CreateOnCtrlN, "  idea "), Key{Kind: KeyCreate}); !done || result.Query != "idea" {
		t.Fatalf("create takes the trimmed query: %+v", result)
	}

	var d decoder
	got := d.feed([]byte("\x1b[20"))
	got = append(got, d.feed([]byte("0~one\rtwo\x1b[201~"))...)
	if want := []Key{{Kind: KeyPaste, Text: "one\rtwo"}}; !slices.Equal(got, want) {
		t.Fatalf("a paste marker split across reads: got %v", got)
	}
}
