# folio

A notational-velocity switcher over plain-markdown notes, one notebook per
folder. Notebooks live beside your projects, never inside them: a clone's
notebook sits in its owner's `notebooks/` store, a git repository synced
between machines, and any other folder can keep a local `notes/`. Structure
comes mainly from `#tags` you define inside the notes (e.g. `#moc` for a map of
content). Edit with vim + the wikijump.vim plugin.

## Usage

```
folio [-c] [<query>]       switch among the current notebook's notes
folio --all [<query>]      switch among every store's notes (open only)
folio --filter <query>     print the current notebook's matches, best first
folio --dir                print the current notebook's folder
folio init [--local]       create the current folder's notebook
folio mv <old> <new>       move a folder and its notebook
folio forget <folder>      delete an orphan's notebook and its notes
folio check                report problems; exit 1 if any
folio sync [--daemon]      sync every store with its remote (now, or keep at it)
```

Filter as you type; **Enter** opens the highlighted match in `$EDITOR`,
**Ctrl-N** creates a note named by your query (`.md` is added automatically;
`work/idea` creates `work/idea.md`, making the folder). Near-matches stay
visible, so a typo never silently forks the namespace. Creating reuses an
existing note whose path differs only in case.

Pass `-c`/`--create-on-enter` to let **Enter** also create when nothing matches
(fast capture; Ctrl-N then moves down); off by default. A query that is a
command name or starts with `-` goes after `--` (`folio -- init`,
`folio -c -- -draft`).

Matching is exact, not fuzzy: each word of the query must appear in a note's
name or tags as a contiguous substring, ignoring case unless the query has an
upper-case letter. Best first: words matching at the start of a name part, then
earlier, then shorter names.

Keys: Up/Ctrl-K/Ctrl-P and Down/Ctrl-J move; Backspace, Ctrl-W (word) and Ctrl-U
(all) edit the query; pasted text joins it on one line; Esc, Ctrl-C or Ctrl-G
cancel. The picker draws inline (up to 40% of the terminal) on `/dev/tty`, and
the editor opens there too, whatever folio's stdin and stdout.

### Notebooks

```
~/.config/folio/template/        global template, copied into new notebooks
~/GitHub/<owner>/
  <repo>/                        a project clone: no notes inside
  notebooks/                     the owner's store: a git clone with a remote
    .folio                       store marker
    .template/                   optional, overrides the global template
    <repo>/                      the notebook of ~/GitHub/<owner>/<repo>
      .notebook                  notebook marker
      TODO.md
      <note>.md
~/scratch/<topic>/
  notes/                         the local notebook of ~/scratch/<topic>
    .notebook
    <note>.md
```

- A **store** is a `notebooks/` folder holding `.folio`, in an owner folder
  (`~/<host>/<owner>/`). You set it up, usually by cloning the owner's notes
  repository: `git clone --filter=blob:none --sparse <url>
  ~/GitHub/<owner>/notebooks`. The marker is committed, so it comes with the
  clone. folio never creates a store.
- A **notebook** is a folder holding `.notebook`: one per project in a store,
  named after the project's folder, or a **local notebook**, a `notes/` folder
  inside the folder it serves, never synced.
- The **current notebook**: take the git top level (for a worktree, its main
  checkout, so a worktree shares its repository's notebook), else the current
  folder. Its notebook is the one in its parent's store, else the nearest
  `notes/` up to `~` (never `~` itself); inside a store's notebook, that
  notebook. Elsewhere folio refuses: there is no default notebook.

`folio init` creates the current folder's notebook in its parent's store and
copies the template into it. With no store there, it asks whether to create a
local `notes/` instead (warning when that would sit inside a repository);
`folio init --local` does it without asking. Without a terminal it never asks:
it fails and names `--local`, so a calling tool never answers for you.

The template is the store's `.template/` if it has one, else
`~/.config/folio/template/` (`$XDG_CONFIG_HOME` honoured); it is copied without
overwriting. For wikijump.vim, put its root marker in the template once:
`mkdir -p ~/.config/folio/template && touch ~/.config/folio/template/.wikijump`.

Only `*.md` files are notes; files and folders starting with `_` or `.` are
skipped, and a new note's name may not use them. A note or folder that cannot be
read is skipped, never failing the listing. `--all` names notes
`<owner>/<notebook>/<note>` (with the host first when an owner has stores under
two hosts).

### Moving and forgetting

A notebook whose folder no longer exists on this machine is an **orphan**: it
stays on disk and `folio check` reports it; nothing is hidden or deleted
automatically.

- `folio mv <old> <new>` moves a folder and its notebook together: `git mv`
  where git tracks them (so history follows), else a plain move; across stores,
  copy then delete. When you already moved the folder by hand, it moves only
  the notebook.
- `folio forget <folder>` deletes an orphan's notebook after a `y` on a prompt
  that counts every file it deletes. It refuses a notebook whose folder exists,
  and refuses to run without a terminal.

### Sync

`folio sync` syncs every store at `~/*/*/notebooks` that has a remote; local
notebooks are never synced. `folio sync --daemon`, run by the `folio-sync`
systemd user unit (`contrib/folio-sync.service`), polls the stores every 10 s,
syncs one once its changes have rested for 60 s, and every store every 3 minutes
or after a resume from suspend. A manual sync and the daemon share a per-store
lock; every git command times out after 2 minutes.

Each sync of a store:

1. grows a sparse checkout to the notebooks whose folders exist on this machine
   (never removing one);
2. commits every change as `<host> · <UTC time>`;
3. fetches, and merges with `git merge-tree`, touching no file until the result
   is ready; `git reset --keep` then moves to it, and retries on the next sync
   if you edited a file meanwhile;
4. on a conflict, keeps the remote version in place and yours beside it as
   `<name>.conflict-<host>-<UTC time>.md`; a deleted file against a modified one
   keeps the modified file; a file against a folder keeps both, the file renamed
   `<name>~<side>`. Sync never stops on a conflict;
5. scans the commits to push with `gitleaks`: a finding, or `gitleaks` missing,
   keeps them local;
6. pushes. History is never rewritten.

Each store's last success and last error go to `~/.cache/folio/status.json`
(`$XDG_CACHE_HOME` honoured).

### Health check

`folio check` changes nothing, and exits 1 when it reports any of: orphan
notebooks (with the `folio mv` or `folio forget` to run), stray folders in a
store, nested notebooks, store markers outside a `notebooks/` folder, conflict
files, and stores that do not sync (no git, no remote, no upstream yet, unpushed
commits, never synced, last synced over an hour ago, or a last sync that failed,
with its error).

### Tags

Every `#word` tag in a note is shown next to its name and is searchable: type
`#moc` to list only notes tagged `#moc`. Matching is by substring, so `#moc`
also matches `#mocha`. Markdown headings (`# Title`) and URL fragments
(`foo#bar`) are not tags.

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

```
go build -o ~/.local/bin/folio ./cmd/folio
cp contrib/folio-sync.service ~/.config/systemd/user/
systemctl --user daemon-reload && systemctl --user enable --now folio-sync
```

Requires `git`, and `gitleaks` to push. The service's `PATH` must reach both,
and pushing from it needs credentials that work without a terminal (an SSH key
without a passphrase, or a credential helper).

## Environment

- `EDITOR`: the editor for notes, split on spaces so it may hold flags
  (`code -w`); default `vim`.
- `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`: where the global template and the sync
  status live (default `~/.config`, `~/.cache`).

## Develop

`go test ./...` runs the tests, against a temporary `$HOME` and local git
repositories, never the real stores or the network. `pre-commit` runs them with
`gofmt`, `go vet`, gitleaks, shellcheck and `bats test/` (the migration
script's tests). Build into `bin/` (git-ignored).

## Migrating from the central store

Until every machine has moved its notes from the old `~/memo` store, see
`migration.md` and `migrate/stores.sh`; both go once that is done.

## Example

<https://github.com/standardgalactic/keg>
