# note

A notational-velocity switcher over a folder of plain-markdown notes.
No notebooks: notes live under `$NOTE_HOME` (subfolders allowed), and
structure comes mainly from `#tags` you define inside the notes (e.g. `#moc`
for a map of content). Edit with vim + the wikijump.vim plugin.

## Usage

```
note [-c] [<query>]
```

Filter as you type; **Enter** opens the highlighted match in `$EDITOR`,
**Ctrl-N** creates a note named by your query (`.md` is added
automatically; `work/idea` creates `work/idea.md`, making the folder). Near-matches stay visible, so a typo never silently forks
the namespace. Creating reuses an existing note whose path differs only in
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

### Dates

`note` stores no dates. The filesystem records when a note was created and
last changed:

```bash
stat -c 'created %w | updated %y' "$NOTE_HOME/foo.md"
```

Vim keeps the creation time only with `set backupcopy=yes` (with the default
`auto` it usually writes a new file on each save, resetting it). Copying,
restoring or syncing notes usually resets both dates.

## Install

Symlink `note` into your `$PATH`. For wikijump.vim, create its root marker
once: `touch "${NOTE_HOME:-$HOME/notes}/.wikijump"`.

Requires [`fzf`](https://github.com/junegunn/fzf) and
[`ripgrep`](https://github.com/BurntSushi/ripgrep).

## Environment

- `NOTE_HOME` — notes folder (default: `~/notes`). Only `*.md` files are
  read, in subfolders too; files and folders starting with `_` or `.` are
  skipped (as are files your `.gitignore` excludes).
- `EDITOR` — editor for opening/creating notes (default: `vim`; may
  include flags, e.g. `code -w`).

## Plan

Migrate `note` to a Go binary — motivated only by shedding the `fzf`/`rg`
runtime dependency into one self-contained executable. The behaviour
above is unchanged by that migration.

## Example

<https://github.com/standardgalactic/keg>
