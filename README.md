# nb

A notebook manager over a tree of plain-markdown files. `note` owns
cross-notebook navigation and creation; inside a notebook you edit with
vim + the wikijump.vim plugin.

## Commands

Two notational-velocity switchers (filter as you type; **Enter** opens the
highlighted match, **Ctrl-N** creates a new one named by your query) plus
shell integration. Pass `-c`/`--create-on-enter` to let **Enter** also
create when nothing matches (fast capture); off by default. Matching is
exact (each word a contiguous substring), not fuzzy.

| Command | What it does |
|---|---|
| `note [<query>]` | **Note switcher.** Filter notes (shown without the `.md` suffix), open the pick in vim, or create the typed name — `.md` is added automatically. Near-matches stay visible, so a typo never silently forks the namespace. |
| `note book [<query>]` | **Notebook switcher.** Filter notebooks, select or create one. Prints the notebook path (compose with `nb` to cd into it). |
| `note init <shell>` | Print shell integration (`nb` + completion) for `eval`; `<shell>` is `bash` or `zsh`. |
| `note help` | Usage. |

Bare `note` (no query) opens the switcher over every note — the primary
"jump to anything" action.

### Note switcher scope

- `-n, --notebook <nb>` — restrict to notebook `<nb>`.
- Inside a notebook (cwd under `$NOTE_HOME`) — restrict to the notebook
  named by the first path component.
- Otherwise — search all of `$NOTE_HOME`. Creating a *new* note then
  requires `-n` or a notebook as cwd.

## Install

Symlink `note` into your `$PATH`, then add the shell integration to your
rc file:

```bash
eval "$(note init bash)"      # or: note init zsh
```

This defines `nb`, the cd-wrapper around `note book` (a command cannot
change the parent shell's directory, so the cd must run in a function):

```bash
nb [<query>]    # pick or create a notebook, then cd into it
```

Requires [`fzf`](https://github.com/junegunn/fzf) and
[`ripgrep`](https://github.com/BurntSushi/ripgrep).

## Environment

- `NOTE_HOME` — notes root (default: `~/notes`).
- `EDITOR` — editor for opening/creating notes (default: `vim`; may
  include flags, e.g. `code -w`).

## Configuration

Edit the constants near the top of `note`:

- `NEW_NOTEBOOK_FILES` — files scaffolded into a new notebook (default:
  `.wikijump`, the wikijump.vim root marker).
- `NEW_NOTEBOOK_CMD` — command run inside a freshly created notebook
  (default: open `README.md` in `$EDITOR`); empty array disables it.

## Plan

Migrate `note` to a Go binary — motivated only by shedding the `fzf`/`rg`
runtime dependency into one self-contained executable. The command set and
UX above are unchanged by that migration.

## Example

<https://github.com/standardgalactic/keg>
