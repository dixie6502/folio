# note

A notational-velocity switcher over a flat folder of plain-markdown notes.
No notebooks, no nesting: every note lives directly in `$NOTE_HOME`, and
structure comes from `#tags` you define inside the notes (e.g. `#moc` for a
map of content). Edit with vim + the wikijump.vim plugin.

## Usage

```
note [-c] [<query>]
```

Filter as you type; **Enter** opens the highlighted match in `$EDITOR`,
**Ctrl-N** creates a note named by your query (`.md` is added
automatically). Near-matches stay visible, so a typo never silently forks
the namespace. Creating reuses an existing note whose name differs only in
case.

Pass `-c`/`--create-on-enter` to let **Enter** also create when nothing
matches (fast capture); off by default. Matching is exact (each word a
contiguous substring), not fuzzy.

Bare `note` (no query) opens the switcher over every note.

### Tags

Every `#word` tag in a note is shown next to its name and is searchable:
type `#moc` to list only notes tagged `#moc`. Matching is by substring, so
`#moc` also matches `#mocha`. Markdown headings (`# Title`) and URL
fragments (`foo#bar`) are not tags.

## Install

Symlink `note` into your `$PATH`. For wikijump.vim, create its root marker
once: `touch "${NOTE_HOME:-$HOME/notes}/.wikijump"`.

Requires [`fzf`](https://github.com/junegunn/fzf) and
[`ripgrep`](https://github.com/BurntSushi/ripgrep).

## Environment

- `NOTE_HOME` — notes folder (default: `~/notes`). Only `*.md` files at its
  top level are read; subfolders and `_`-prefixed files are ignored.
- `EDITOR` — editor for opening/creating notes (default: `vim`; may
  include flags, e.g. `code -w`).

## Plan

Migrate `note` to a Go binary — motivated only by shedding the `fzf`/`rg`
runtime dependency into one self-contained executable. The behaviour
above is unchanged by that migration.

## Example

<https://github.com/standardgalactic/keg>
