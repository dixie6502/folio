// Package notes lists a notebook's notes with their #tags, filters them by a
// query, and creates new ones.
package notes

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Extension is the suffix of a note file, implicit in names and queries.
const Extension = ".md"

// Note is one note file.
type Note struct {
	Path string   // the file
	Name string   // what the switcher shows and matches: a prefix, then the path in its notebook, without Extension
	Tags []string // its #tags, each once, in order of appearance

	search string // Name then Tags, as matched; built by List
}

// searchText is what a query is matched against: the name, then the tags.
func (n Note) searchText() string {
	if n.search == "" {
		return n.Name + " " + strings.Join(n.Tags, " ")
	}
	return n.search
}

// isHidden reports whether a file or folder name is left out of the listing,
// so never given to a new note either.
func isHidden(name string) bool { return strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") }

// List returns the notes of notebook, sorted by path, each named prefix plus
// its path in the notebook. Hidden files and folders are skipped, and so are
// what cannot be read inside the notebook (an unreadable folder, a broken
// link): one bad entry never hides the other notes. A note that cannot be read
// is listed without tags.
func List(notebook, prefix string) ([]Note, error) {
	var list []Note
	err := filepath.WalkDir(notebook, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil && path == notebook:
			return err
		case err != nil:
			return nil // skip it, and a folder's content with it
		case path == notebook:
			return nil
		case isHidden(entry.Name()) && entry.IsDir():
			return filepath.SkipDir
		case isHidden(entry.Name()) || !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), Extension):
			return nil
		}
		data, _ := os.ReadFile(path)
		rel, _ := filepath.Rel(notebook, path)
		note := Note{Path: path, Name: prefix + strings.TrimSuffix(rel, Extension), Tags: tags(data)}
		note.search = note.searchText()
		list = append(list, note)
		return nil
	})
	return list, err
}

// tags returns the #tags of text, each once, in order: a # after the start of
// a line or whitespace (so never a markdown heading "# Title" or a URL
// fragment "foo#bar"), a letter, then letters, digits, _, / or -.
func tags(text []byte) []string {
	var found []string
	for at := 0; ; {
		hash := bytes.IndexByte(text[at:], '#')
		if hash < 0 {
			return found
		}
		start := at + hash
		at = start + 1
		if start > 0 && !strings.ContainsRune(" \t\n\r\f", rune(text[start-1])) {
			continue
		}
		end := at
		for end < len(text) && isTagByte(text[end], end == at) {
			end++
		}
		if end > at {
			if tag := string(text[start:end]); !slices.Contains(found, tag) {
				found = append(found, tag)
			}
			at = end
		}
	}
}

// isTagByte reports whether b may stand in a tag; first means right after #.
func isTagByte(b byte, first bool) bool {
	letter := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
	if first {
		return letter
	}
	return letter || b >= '0' && b <= '9' || b == '_' || b == '/' || b == '-'
}

// Filter returns the notes matching query, best first. Each word of the
// query must appear in a note's name or tags as a contiguous substring;
// matching ignores case unless the query has an upper-case letter. Notes
// whose words match at the start of a name part rank first, then those
// matching earlier, then shorter names; ties keep the list's order. An empty
// query matches every note, in order.
func Filter(list []Note, query string) []Note {
	words := strings.Fields(query)
	if len(words) == 0 {
		return list
	}
	foldCase := !strings.ContainsFunc(query, unicode.IsUpper)
	if foldCase {
		for i, word := range words {
			words[i] = strings.ToLower(word)
		}
	}
	type ranked struct {
		note             Note
		atStart, earlier int
	}
	var matches []ranked
notes:
	for _, note := range list {
		text := note.searchText()
		if foldCase {
			text = strings.ToLower(text)
		}
		match := ranked{note: note}
		for _, word := range words {
			at := strings.Index(text, word)
			if at < 0 {
				continue notes
			}
			if at == 0 || strings.ContainsRune("/-_ .#", rune(text[at-1])) {
				match.atStart++
			}
			match.earlier += at
		}
		matches = append(matches, match)
	}
	slices.SortStableFunc(matches, func(a, b ranked) int {
		if a.atStart != b.atStart {
			return b.atStart - a.atStart
		}
		if a.earlier != b.earlier {
			return a.earlier - b.earlier
		}
		return len(a.note.Name) - len(b.note.Name)
	})
	result := make([]Note, len(matches))
	for i, match := range matches {
		result[i] = match.note
	}
	return result
}

// Create returns the file for a note typed as name in notebook, making its
// folder (the editor writes the file on save; it cannot make the folder):
// name plus Extension (unless typed), or an existing note of list whose path
// differs only in case, so a capitalisation slip never forks a note. A name
// with an empty or hidden part, which the listing would never show, is
// refused.
func Create(notebook, name string, list []Note) (string, error) {
	if notebook == "" {
		return "", errors.New("no notebook to create the note in")
	}
	name = strings.TrimSuffix(name, Extension) + Extension
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || isHidden(part) {
			return "", errors.New("not a listable note path (no part empty or starting with '_' or '.'): " + name)
		}
	}
	path := filepath.Join(notebook, name)
	for _, note := range list {
		if strings.EqualFold(note.Path, path) {
			return note.Path, nil
		}
	}
	return path, os.MkdirAll(filepath.Dir(path), 0o755)
}
