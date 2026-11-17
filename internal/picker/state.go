// Package picker is folio's switcher: a list of notes filtered as you type,
// drawn inline in the terminal.
package picker

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/dixie6502/folio/internal/notes"
	"github.com/mattn/go-runewidth"
)

// Mode decides what the picker may create.
type Mode int

// The picker's modes.
const (
	OpenOnly      Mode = iota // open a note, never create (--all)
	CreateOnCtrlN             // Ctrl-N creates the typed name (the default)
	CreateOnEnter             // Enter creates when nothing matches (-c)
)

// Action is what the user chose.
type Action int

// The picker's outcomes.
const (
	Cancel Action = iota // nothing
	Open                 // open Result.Note
	Create               // create a note named Result.Query
)

// Result is the user's choice.
type Result struct {
	Action Action
	Note   notes.Note // for Open
	Query  string     // the query, trimmed, for Create
}

// Drawing: the longest name column, so one long name cannot push every tag
// off-screen, and the colors.
const (
	maxNameWidth = 50
	dim          = "\x1b[2m"
	tagColor     = "\x1b[2;36m"
	bold         = "\x1b[1m"
	reset        = "\x1b[0m"
)

// state is the picker between two keys.
type state struct {
	all       []notes.Note
	mode      Mode
	query     string
	matches   []notes.Note // all, filtered by query
	nameWidth int          // the name column: the widest match's name, capped
	cursor    int          // the selected match
	offset    int          // the first match shown
}

// newState returns the picker's state over list, with query typed.
func newState(list []notes.Note, mode Mode, query string) *state {
	s := &state{all: list, mode: mode}
	s.setQuery(query)
	return s
}

// setQuery changes the query, filters again and selects the best match.
func (s *state) setQuery(query string) {
	s.query, s.matches, s.cursor, s.offset = query, notes.Filter(s.all, query), 0, 0
	s.nameWidth = 0
	for _, note := range s.matches {
		if s.nameWidth = max(s.nameWidth, runewidth.StringWidth(note.Name)); s.nameWidth >= maxNameWidth {
			s.nameWidth = maxNameWidth
			break
		}
	}
}

// hasQuery reports whether something other than spaces is typed.
func (s *state) hasQuery() bool { return strings.TrimSpace(s.query) != "" }

// handle applies key; done reports that the picker is finished with result.
func (s *state) handle(key Key) (done bool, result Result) {
	switch key.Kind {
	case KeyRune:
		s.setQuery(s.query + key.Text)
	case KeyPaste:
		s.setQuery(s.query + pastable(key.Text))
	case KeyBackspace:
		if s.query != "" {
			runes := []rune(s.query)
			s.setQuery(string(runes[:len(runes)-1]))
		}
	case KeyClear:
		s.setQuery("")
	case KeyDeleteWord:
		trimmed := strings.TrimRightFunc(s.query, unicode.IsSpace)
		cut := strings.LastIndexFunc(trimmed, unicode.IsSpace)
		s.setQuery(trimmed[:cut+1])
	case KeyUp:
		s.cursor = max(s.cursor-1, 0)
	case KeyDown:
		s.cursor = min(s.cursor+1, max(len(s.matches)-1, 0))
	case KeyCreate:
		if s.mode != CreateOnCtrlN {
			return s.handle(Key{Kind: KeyDown}) // Ctrl-N is down where it does not create
		}
		if s.hasQuery() {
			return true, Result{Action: Create, Query: strings.TrimSpace(s.query)}
		}
	case KeyEnter:
		switch {
		case len(s.matches) > 0:
			return true, Result{Action: Open, Note: s.matches[s.cursor]}
		case s.mode == CreateOnEnter && s.hasQuery():
			return true, Result{Action: Create, Query: strings.TrimSpace(s.query)}
		}
	case KeyCancel:
		return true, Result{Action: Cancel}
	}
	return false, Result{}
}

// pastable returns pasted text fit for the query: on one line, without
// control characters (which the prompt would echo to the terminal).
func pastable(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

// header names the keys of the picker's mode.
func (s *state) header() string {
	switch s.mode {
	case CreateOnCtrlN:
		return "enter: open   ctrl-n: create typed name"
	case CreateOnEnter:
		return "enter: open/create"
	default:
		return "enter: open"
	}
}

// height returns how many lines the picker takes in a terminal of rows: up
// to 40% of it, fewer when the notes need fewer, never under 3.
func (s *state) height(rows int) int {
	return max(3, min(rows*2/5, len(s.all)+2))
}

// lines draws the picker as exactly height lines of at most width columns:
// the prompt, a count with the keys, then the matches, scrolled to show the
// selected one. It returns the column of the cursor on the prompt line.
func (s *state) lines(width, height int) ([]string, int) {
	prompt := "> " + s.query
	info := fmt.Sprintf("  %d/%d  %s", len(s.matches), len(s.all), s.header())
	out := []string{
		runewidth.Truncate(prompt, width, ""),
		dim + runewidth.Truncate(info, width, "") + reset,
	}
	rows := height - len(out)
	if s.cursor < s.offset {
		s.offset = s.cursor
	} else if s.cursor >= s.offset+rows {
		s.offset = s.cursor - rows + 1
	}
	for i := s.offset; i < s.offset+rows; i++ {
		line := ""
		if i < len(s.matches) {
			line = matchLine(s.matches[i], i == s.cursor, s.nameWidth, width)
		}
		out = append(out, line)
	}
	return out, min(runewidth.StringWidth(prompt), width-1)
}

// matchLine draws one match: a marker when selected, the name padded to
// nameWidth, then its tags, cut to width. Tabs in a name show as spaces.
func matchLine(note notes.Note, selected bool, nameWidth, width int) string {
	marker := "  "
	if selected {
		marker = bold + "> "
	}
	room := width - 2
	name := strings.ReplaceAll(note.Name, "\t", " ")
	name = runewidth.FillRight(runewidth.Truncate(name, nameWidth, "…"), nameWidth)
	name = runewidth.Truncate(name, room, "")
	line := marker + name
	if room -= runewidth.StringWidth(name); len(note.Tags) > 0 && room > 2 {
		line += reset + tagColor + runewidth.Truncate("  "+strings.Join(note.Tags, " "), room, "")
	}
	return line + reset
}
