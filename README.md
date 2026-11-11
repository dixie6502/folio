# folio

A notational-velocity switcher over plain-markdown notes, one notebook per
folder. Notebooks live outside your projects, in one store (`$FOLIO_HOME`,
default `~/memo`); each is tied to the folder it was initialized from.
Structure comes mainly from `#tags` you define inside the notes (e.g. `#moc`
for a map of content). Edit with vim + the wikijump.vim plugin.

## Usage

```
folio init                 # once per folder: create its notebook
folio [-c] [<query>]       # switch among the current notebook's notes
folio --all [<query>]      # switch among every notebook's notes (open only)
folio --dir                # print the current notebook's folder
folio check                # report notebooks gone astray
folio map <old-key>        # re-attach a notebook to the current folder
```

Filter as you type; **Enter** opens the highlighted match in `$EDITOR`,
**Ctrl-N** creates a note named by your query (`.md` is added
automatically; `work/idea` creates `work/idea.md`, making the folder).
Near-matches stay visible, so a typo never silently forks the namespace.
Creating reuses an existing note whose path differs only in case.

Pass `-c`/`--create-on-enter` to let **Enter** also create when nothing
matches (fast capture); off by default. Matching is exact (each word a
contiguous substring), not fuzzy. A query that is a command name or starts
with `-` goes after `--` (`folio -- init`, `folio -c -- -draft`).

### Notebooks

A notebook's **key** names its folder in the store:

- inside a git repository with an `origin`: origin's path, lowercased since
  forges ignore case there, whatever the URL form
  (`https://<hostname>/<owner>/<repo>.git` and
  `git@<hostname>:<owner>/<repo>.git` both give `<owner>/<repo>`). Every
  checkout of a repo, wherever it lives, opens the same notebook;
- any other folder under `~` (also when reached through a symlink under `~`):
  its path from `~` (`scratch/<topic>`).

`folio` uses the current folder's notebook: the origin key's, else the nearest
initialized folder walking up to `~`; inside a notebook's own folder in the
store, that notebook. Outside one it refuses; there is no default notebook.
`folio init` creates the notebook, copies the template into it, and records
where it came from in `.source` (credentials in the origin URL are dropped; a
repository outside `~` records no path):

```
key=<owner>/<repo>
path=~/GitHub/<owner>/<repo>
origin=git@<hostname>:<owner>/<repo>.git
```

When a key changes (repo renamed, new origin, folder moved), `folio check`
reports the notebook as re-keyed or orphan, and `folio map <old-key>`, run
from the folder, moves it to the folder's current key.

### Tags

Every `#word` tag in a note is shown next to its name and is searchable:
type `#moc` to list only notes tagged `#moc`. Matching is by substring, so
`#moc` also matches `#mocha`. Markdown headings (`# Title`) and URL
fragments (`foo#bar`) are not tags.

### Dates

`folio` stores no dates. The filesystem records when a note was created and
last changed:

```bash
stat -c 'created %w | updated %y' "$(folio --dir)/foo.md"
```

Vim keeps the creation time only with `set backupcopy=yes` (with the default
`auto` it usually writes a new file on each save, resetting it). Copying,
restoring or syncing notes usually resets both dates.

## Install

Symlink `folio` into your `$PATH`. For wikijump.vim, put its root marker in
the template once, so every new notebook gets one:
`mkdir -p ~/memo/.template && touch ~/memo/.template/.wikijump`.

Requires [`fzf`](https://github.com/junegunn/fzf),
[`ripgrep`](https://github.com/BurntSushi/ripgrep) and `git`. Uses `gawk`
when installed so tags line up after non-ASCII note names; any other `awk`
works but may misalign them.

`folio` never runs git beyond reading a repository's origin: version or sync
the store yourself (it is a plain folder).

## Environment

- `FOLIO_HOME` — the store (default: `~/memo`). Only `*.md` files are read,
  in subfolders too; files and folders starting with `_` or `.` are skipped
  (as are files your `.gitignore` excludes).
- `FOLIO_TEMPLATE` — folder copied into each new notebook, never overwriting
  (default: `$FOLIO_HOME/.template`; missing: the notebook starts empty).
- `EDITOR` — editor for opening/creating notes (default: `vim`; may
  include flags, e.g. `code -w`).

## Develop

`bats test/` runs the tests (bats-core), against a temporary `$HOME` and
local git repositories. `pre-commit` runs them with gitleaks and shellcheck.

## Plan

Migrate `folio` to a Go binary — motivated only by shedding the `fzf`/`rg`
runtime dependency into one self-contained executable. The behaviour
above is unchanged by that migration.

## Example

<https://github.com/standardgalactic/keg>
