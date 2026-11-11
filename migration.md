# Migration: one notebook per folder

Draft spec, under discussion. Extends and simplifies `note` so it can hold every
project's notes and open items, outside the project repos, for use both in a
terminal and by Claude (mneme's `memo` plugin).

The tool is renamed, so it is not confused with the `todo` and `note` skills
that use it (name open; candidates: `folio`, `sheaf`, `quire`, `jot`). Below,
`note` stands for the new name, and `NOTE_*` variables follow it.

## Decided

- `note` stays a switcher: filter, open, create. No link commands; links are
  `wikijump.vim` in the editor and `wkref` on the command line. Beside it, a few
  non-interactive commands manage notebooks: `init`, `--dir`, `check`, `map`.
- No built-in global notebook. A notebook is tied to a folder and exists only
  once initialized there; outside one, `note` refuses.
- A todo list is an ordinary note (`TODO.md`). Its format belongs to the `todo`
  skill; `note` knows nothing about it. Likewise any frontmatter (a `created:`
  date) is a skill's convention, not `note`'s.
- `note` knows no editor convention either: what a new notebook starts with
  comes from a template folder.
- `note` is unaware of git beyond reading a repository's `origin`. Versioning
  and sync stay outside (mneme's hooks).
- Bash for now; a Go port comes when dropping `fzf`/`rg` is wanted (see
  README › Plan). Tests with `bats-core`.

## Spec

### Store and notebooks

- The store is one folder, `$NOTE_HOME` (default: see Open questions), holding
  one notebook per initialized folder, and the template:

  ```
  <store>/
    .template/             copied into each new notebook
      .wikijump            e.g. wikijump.vim's root marker
    <owner>/<repo>/        a git repository, keyed by its origin
      .source              where the notebook was initialized
      .wikijump
      TODO.md
      auth-flow.md
      research/caching.md  subfolders allowed, as today
    scratch/<topic>/       any other folder: its path relative to ~
  ```

- A **notebook** is a folder of the store holding `.source`. Nothing else in the
  store is a notebook; `check` reports what lies outside them.
- **Notebook key** of a folder:
  - inside a git repository with an `origin` remote: the path of origin's URL,
    without host or `.git` suffix. Every URL form gives the same key:

    ```
    https://github.com/dixie6502/note.git      → dixie6502/note
    git@github.com:dixie6502/note.git          → dixie6502/note
    ssh://git@<hostname>:2222/dixie6502/note   → dixie6502/note
    ```

    So every checkout of a repo opens the same notebook, wherever it lives:
    worktrees share origin, mirrors with the same `<owner>/<repo>` on two hosts
    share a notebook, nested groups keep their parts (`group/sub/repo`), a fork
    has its own (its origin), and inside a submodule the submodule wins;
  - any other folder under `~` (a repository without `origin` included): its
    path relative to `~`;
  - outside `~`: none (refused).
- Keys are plain names: each part matches `[A-Za-z0-9._-]+`, never `.` or `..`.
- `.source` records the origin at init time, as `key=value` lines with paths
  from `~`, so the store moves between machines unchanged:

  ```
  key=dixie6502/note
  path=~/GitHub/dixie6502/note
  origin=git@github.com:dixie6502/note.git
  ```

  `origin` is empty for a folder keyed by its path.

### Finding the current notebook

From the current folder: the origin key, if inside a repository with one; then
the path key of each folder walking up to `~`. The first key with a notebook in
the store wins. None: `no notebook here: note init`, exit 1.

### Commands

- `note init`: creates the notebook of the current folder under its key
  (origin key in a repository, else the current folder's path key), copies the
  template's contents into it (`$NOTE_TEMPLATE`, default `<store>/.template`;
  missing or empty: nothing copied), writes `.source`, and prints the path.
  Already a notebook: prints the path, exit 0, changes nothing.
- `note [-c] [<query>]`: the switcher, as today, over the current notebook.
- `note --all [<query>]`: the switcher over every notebook, each note shown as
  `<key>/<name>`. Opens only: creating from it is refused.
- `note --dir`: prints the current notebook's path, exit 1 if none. For scripts
  and Claude skills, which cannot drive the switcher.
- `note check`: the store's health. Changes nothing; exit 1 when it reports
  anything. For each notebook:
  - **orphan**: the folder recorded in `.source` no longer exists (moved or
    deleted; from its new place, `note map` re-attaches the notebook);
  - **re-keyed**: the folder exists but its key now differs (origin added or
    changed, repo renamed or transferred): `note map <old-key>` from it;
  - **bad record**: `.source` unreadable or its `key` not the notebook's folder.

  And outside notebooks: **stray** files or folders in the store (not `.template`,
  not under a notebook).
- `note map <old-key>`: run from a folder, moves the notebook `<old-key>` to the
  current folder's key and rewrites its `.source`. Refuses when `<old-key>` is
  not a notebook, or the current key already has one. Empty parent folders left
  by the move are removed.

### Unchanged

`-c`, `#tags` display and search, `$EDITOR`, `_`/`.` files skipped (so
`.source` and the template never show), case-insensitive reuse on create,
`.gitignore` honoured.

### Tests

`bats-core`, pinned, installed at user level with `mise` (and added to
construct's bootstrap). Tests run against a temporary `$HOME` and store, with
real git repositories whose origins are set locally (no network): key from each
URL form, the walk-up, refusals, `init` idempotence and template copy, `check`
for each finding, `map` and its refusals, `--dir`. The switcher itself (fzf) is
tested through its listing function, not interactively.

## Effects on mneme

- The `memo` store becomes `note`'s store. Its layout flattens: today's
  `<scope>/TODO.md`, `<scope>/note/` and `<scope>/todo/` become one notebook per
  key, `TODO.md` and item details being ordinary notes in it, with a `.source`
  written for each. A migration script does it once.
- The `todo` and `note` skills find the notebook with `note --dir` instead of
  `memo.sh dir`; `memo.sh` keeps only the sync.
- `debrief` reads every notebook in the store and lists `note check` findings
  under "Needs you".
- `global.md` needs no rule about where open items or notes live.

## Open questions

1. The new name (candidates above), which also names the store variable.
2. The store's default folder: `~/<name>`, or keep `~/memo`?
