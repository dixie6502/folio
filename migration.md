# Migration: stores beside the clones, sync built in, Go

Draft spec, decided unless listed under Open questions. `README.md` describes
the current Bash `folio` and stays in sync with the code; this file describes
the target. Once implemented, its content moves into `README.md` and this file
is deleted.

Goal: notes easy to reach from the folder you work in, never inside a project
repository (kept out of an LLM's project context, no second version history in
the project), synced between machines, with one store per owner so a machine
gets only the owners it needs.

## Decided

- `folio` stays a switcher (filter, open, create) plus a few non-interactive
  notebook commands. No link commands: links belong to wikijump.vim.
- Rewritten in Go, as one binary. Its own picker replaces `fzf`; reading notes
  and tags replaces `rg` and `gawk`. Runtime dependencies: `git`, plus
  `gitleaks` to push. Go modules: `golang.org/x/term` (raw terminal) and
  `github.com/mattn/go-runewidth` (display width of note names), pinned.
- Sync is part of the binary (`folio sync`), so the store rules live in one
  place. Only the sync code runs git beyond reading a repository's top level.
- No ids, no origin parsing, no `.source` record: a notebook is found by its
  path.
- `folio` knows nothing of `TODO.md`'s format, frontmatter or editor
  conventions; a template carries those.
- The command-line contract used by other tools (`folio --dir`, `folio init`)
  keeps its shape.

## Spec

### Stores and notebooks

- A **store** is a folder named `notebooks/` holding a `.folio` marker,
  placed by the user in an owner folder (`~/<host>/<owner>/`, the parent of
  the projects it serves), usually by cloning the owner's notes repository:
  `git clone <url> ~/GitHub/<owner>/notebooks`. The marker is committed, so it
  comes with the clone; it tells a store from a project that happens to be
  called `notebooks`. `folio` never creates a store.
- A **notebook** is a folder holding a `.notebook` marker, either inside a
  store or as a **local notebook**: a `notes/` folder inside the folder it
  serves, never synced.
- Layout:

  ```
  ~/.config/folio/template/        global template, copied into new notebooks
  ~/GitHub/<owner>/
    <repo>/                        a project clone: no notes inside
    notebooks/                     store, a git clone with a remote
      .folio                       store marker
      .template/                   optional, overrides the global template
      <repo>/                      notebook of ~/GitHub/<owner>/<repo>
        .notebook
        TODO.md
        todo/
        <note>.md
  ~/scratch/<topic>/
    notes/                         local notebook of ~/scratch/<topic>
      .notebook
      <note>.md
  ```

### Current notebook

The folder of reference is the git top level when inside a repository (for a
worktree, the main checkout, from `git rev-parse --git-common-dir`), else the
current folder. Its notebook is, in order:

1. `<parent>/notebooks/<name>/`, when `<parent>` (the reference folder's
   parent) holds a store and that notebook exists;
2. else the nearest `notes/` holding `.notebook`, in the current folder or a
   parent, up to `~`;
3. inside a store's notebook: that notebook.

Otherwise `folio` refuses: there is no default notebook.

### `folio init`

- Parent holds a store: creates `<parent>/notebooks/<name>/`, its marker and
  the template; prints the notebook.
- No store in the parent: warns and asks
  `no notebooks/ in <parent>: create local notes/ here instead? [y/N]`,
  adding `notes/ will sit inside the repository` when the folder is a
  repository.
- `folio init --local` creates the local notebook without asking.
- Without a terminal, `folio init` without `--local` fails with a message
  naming `--local`, so a calling tool never answers on the user's behalf.

### Template

The nearest of the store's `.template/` and `~/.config/folio/template/`
(`$XDG_CONFIG_HOME` honoured) is copied into each new notebook, never
overwriting. Missing: the notebook starts empty.

### Switcher

Unchanged from `README.md`: filter as you type, exact matching (each word a
contiguous substring), `#tags` shown and searchable, **Enter** opens,
**Ctrl-N** creates, `-c` makes Enter create on no match, `.md` implicit,
`folder/name` creates in a subfolder, a case-only variant of an existing path
reuses it. `--all` filters the notes of every notebook in every store found.
A non-interactive `folio --filter <query>` prints the ranked matches (for
scripts and tests).

### Moving and forgetting

- **Orphan**: a notebook whose folder no longer exists on this machine. It
  stays on disk and is reported; nothing is hidden or deleted automatically.
- `folio mv <old> <new>` moves a folder and its notebook together: `git mv`
  for a folder tracked inside a repository, else `mv`; `git mv` for a notebook
  moving within its store; copy then delete across stores. When the folder
  was already moved by hand, it moves only the notebook.
- `folio forget <folder>` deletes an orphan's notebook and every note in it,
  after a `y` on the prompt. It refuses a notebook whose folder exists.

### Sync

- `folio sync` syncs every store found at `~/*/*/notebooks/.folio` that has a
  remote; local notebooks are never synced. `folio sync --daemon` runs the
  loop below and is started by a systemd user unit, `folio-sync`.
  `folio sync --now` asks the daemon for an immediate cycle.
- Each cycle, per store, under a lock:
  1. **Checkout**: the store is a sparse clone; a notebook is added to the
     sparse set when its folder exists on this machine. The set only grows.
  2. **Commit** every change after 60 s without one, as
     `<host> · <UTC time>`.
  3. **Fetch**, then compute the merge with
     `git merge-tree --write-tree HEAD @{upstream}`, without touching files.
  4. **Update**: create the merge commit, then `git reset --keep` to it, which
     refuses when a file changed meanwhile; the cycle then retries later.
  5. **Conflict**: the remote version stays in place, the local one is kept
     beside it as `<name>.conflict-<host>-<UTC time>.md`; a deleted file
     against a modified one keeps the modified file. Sync never stops on a
     conflict.
  6. **Scan** unpushed commits with `gitleaks`; a finding, or `gitleaks`
     missing, blocks the push and leaves the commits local.
  7. **Push**.
- Timing: `git status` every 10 s, fetch every 3 min, and a full cycle at once
  when the clock jumps (resume from suspend). Polling, no file watcher.
- History is never rewritten.
- Status in `~/.cache/folio/status.json` (`$XDG_CACHE_HOME` honoured), per
  store: last successful sync, unpushed commits, conflict files, orphans, last
  error. Read by mneme's `debrief`.

### Health check

`folio check` reports, from disk and the sync status, and exits 1 if any:

- orphan notebooks (folder gone), with the `folio mv` or `folio forget` to run;
- conflict files left by sync;
- stores without a remote, with unpushed commits, or whose last sync failed
  (with the error), and a daemon whose last sync is older than an hour;
- a store marker in a folder not named `notebooks/`;
- a folder inside a store that is neither a notebook nor in one, and a
  notebook inside another notebook.

It changes nothing.

### Commands

```
folio [-c] [<query>]       switch among the current notebook's notes
folio --all [<query>]      switch among every store's notes (open only)
folio --filter <query>     print the matches, ranked
folio --dir                print the current notebook's folder
folio init [--local]       create the current folder's notebook
folio mv <old> <new>       move a folder and its notebook
folio forget <folder>      delete an orphan notebook
folio check                report problems; exit 1 if any
folio sync [--now|--daemon]
```

## Migration

1. Write the Go skeleton: notebook lookup, `--dir`, `init`, `mv`, `forget`,
   `check`, with tests (Go unit tests; the bats suite as acceptance tests).
2. Add `folio sync` and the systemd unit.
3. Migrate the existing store with a one-off script: each notebook moves from
   the current store to `<parent>/notebooks/<name>/` (stores) or
   `<folder>/notes/` (other folders), using the folder its `.source` records;
   history is split per store with `git filter-repo`; the old store
   repository is archived.
4. Replace the `fzf` picker with the built-in one; drop `fzf`, `rg`, `gawk`.
5. Update mneme (remove `memo.sh` and its hooks, `debrief` reads the sync
   status) and construct (Go, the binary, the systemd unit, the template).

## Open questions

None at present.
