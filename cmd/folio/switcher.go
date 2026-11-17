package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dixie6502/folio/internal/notebook"
	"github.com/dixie6502/folio/internal/notes"
	"github.com/dixie6502/folio/internal/picker"
)

// switchArgs are the switcher's arguments.
type switchArgs struct {
	all    bool        // --all: every store's notes, never creating
	filter bool        // --filter: print the matches
	mode   picker.Mode // what the picker may create
	query  string      // the query to start with
}

// switcher runs the switcher: over the current notebook, every store's
// notebooks (--all), or as a filter printing the matches (--filter).
func switcher(env notebook.Env, cwd string, args []string, stdout, stderr io.Writer) int {
	parsed, err := parseSwitchArgs(args)
	if err != nil {
		return fail(stderr, err)
	}
	var list []notes.Note
	var current string // the notebook a new note goes in; none for --all
	if parsed.all {
		list, err = everyStoreNote(env)
	} else if current, err = env.Find(cwd); err == nil {
		list, err = notes.List(current, "")
	}
	if err != nil {
		return fail(stderr, err)
	}
	if parsed.filter {
		for _, note := range notes.Filter(list, parsed.query) {
			fmt.Fprintln(stdout, note.Name)
		}
		return 0
	}

	result, err := picker.Run(list, parsed.mode, parsed.query)
	if err != nil {
		return fail(stderr, err)
	}
	var path string
	switch result.Action {
	case picker.Cancel:
		return 0
	case picker.Open:
		path = result.Note.Path
	case picker.Create:
		if path, err = notes.Create(current, result.Query, list); err != nil {
			return fail(stderr, err)
		}
	}
	if err := edit(path); err != nil {
		return fail(stderr, err)
	}
	return 0
}

// parseSwitchArgs reads the switcher's arguments: --all or --filter first,
// then -c and at most one query; after --, the query even if it looks like a
// flag or a command.
func parseSwitchArgs(args []string) (switchArgs, error) {
	var parsed switchArgs
	if len(args) > 0 && (args[0] == "--all" || args[0] == "--filter") {
		parsed.all, parsed.filter, args = args[0] == "--all", args[0] == "--filter", args[1:]
	}
	parsed.mode = picker.CreateOnCtrlN
	if parsed.all || parsed.filter {
		parsed.mode = picker.OpenOnly
	}
	flags, afterDashes, queries := args, []string(nil), []string(nil)
	if i := slices.Index(args, "--"); i >= 0 {
		flags, afterDashes = args[:i], args[i+1:]
	}
	for _, arg := range flags {
		switch {
		case arg == "-c" || arg == "--create-on-enter":
			if parsed.mode == picker.OpenOnly {
				return parsed, errors.New("--all and --filter never create: drop -c")
			}
			parsed.mode = picker.CreateOnEnter
		case strings.HasPrefix(arg, "-"):
			return parsed, errors.New("unknown flag: " + arg)
		default:
			queries = append(queries, arg)
		}
	}
	queries = append(queries, afterDashes...)
	if len(queries) > 1 {
		return parsed, errors.New("unexpected arg: " + queries[1])
	}
	if len(queries) == 1 {
		parsed.query = queries[0]
	}
	return parsed, nil
}

// everyStoreNote lists the notes of every store's notebooks, each named
// <owner>/<notebook>/<note>, or <host>/<owner>/<notebook>/<note> when the
// owner has stores under more than one host.
func everyStoreNote(env notebook.Env) ([]notes.Note, error) {
	stores, _ := env.Stores()
	hostsOf := map[string]int{}
	for _, store := range stores {
		hostsOf[notebook.Owner(store)]++
	}
	var list []notes.Note
	found := false
	for _, store := range stores {
		prefix := notebook.Owner(store) + "/"
		if hostsOf[notebook.Owner(store)] > 1 {
			prefix = filepath.Base(filepath.Dir(filepath.Dir(store))) + "/" + prefix
		}
		for _, dir := range notebook.Notebooks(store) {
			found = true
			inNotebook, err := notes.List(dir, prefix+filepath.Base(dir)+"/")
			if err != nil {
				return nil, err
			}
			list = append(list, inNotebook...)
		}
	}
	if !found {
		return nil, errors.New("no notebook in any store")
	}
	return list, nil
}

// edit opens path in $EDITOR (which may hold flags, e.g. "code -w"), else
// vim, on the terminal: like the picker, it works whatever stdin and stdout
// are.
func edit(path string) error {
	editor := strings.Fields(os.Getenv("EDITOR"))
	if len(editor) == 0 {
		editor = []string{"vim"}
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("no terminal for the editor: %w", err)
	}
	defer func() { _ = tty.Close() }()
	cmd := exec.Command(editor[0], append(editor[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, os.Stderr
	return cmd.Run()
}
