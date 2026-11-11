# Migration: note → folio, one notebook per folder

Spec, agreed. Extends and simplifies `note` into `folio`, so it can hold every project's notes
and open items, outside the project repos, for use both in a terminal and by
Claude (mneme's `memo` plugin).

The tool, its repo and its variables are renamed `folio` (`FOLIO_*`), so it is
not confused with the `todo` and `note` skills that use it.

## Decided

- `folio` stays a switcher: filter, open, create. No link commands; links are
  `wikijump.vim` in the editor and `wkref` on the command line. Beside it, a few
  non-interactive commands manage notebooks: `init`, `--dir`, `check`, `map`.
- No built-in global notebook. A notebook is tied to a folder and exists only
  once initialized there; outside one, `folio` refuses.
- A todo list is an ordinary note (`TODO.md`). Its format belongs to the `todo`
  skill; `folio` knows nothing about it. Likewise any frontmatter (a `created:`
  date) is a skill's convention, not `folio`'s.
- `folio` knows no editor convention either: what a new notebook starts with
  comes from a template folder.
- `folio` is unaware of git beyond reading a repository's `origin`. Versioning
  and sync stay outside (mneme's hooks).
- Bash for now; a Go port comes when dropping `fzf`/`rg` is wanted (see
  README › Plan). Tests with `bats-core`.

## Spec

### Store and notebooks

- The store is one folder, `$FOLIO_HOME` (default `~/memo`), holding
  one notebook per initialized folder, and the template:

  ```
  <store>/
    .template/             copied into each new notebook
      .wikijump            e.g. wikijump.vim's root marker
    <owner>/<repo>/        a git repository, identified by its origin
      .source              where the notebook was initialized
      .wikijump
      TODO.md
      auth-flow.md
      research/caching.md  subfolders allowed, as today
    scratch/<topic>/       any other folder: its path relative to ~
  ```

- A **notebook** is a folder of the store holding `.source`. Nothing else in the
  store is a notebook; `check` reports what lies outside them.
- **Notebook id** of a folder:
  - inside a git repository with an `origin` remote: the path of origin's URL,
    without host or `.git` suffix, lowercased (forges ignore case in
    owner/repo). Every URL form gives the same id:

    ```
    https://github.com/dixie6502/folio.git      → dixie6502/folio
    git@github.com:dixie6502/folio.git          → dixie6502/folio
    ssh://git@<hostname>:2222/dixie6502/folio   → dixie6502/folio
    ```

    So every checkout of a repo opens the same notebook, wherever it lives:
    worktrees share origin, mirrors with the same `<owner>/<repo>` on two hosts
    share a notebook, nested groups keep their parts (`group/sub/repo`), a fork
    has its own (its origin), and inside a submodule the submodule wins;
  - any other folder under `~` (a repository without `origin` included): its
    path relative to `~`, physical, else as reached through a symlink under `~`;
  - outside `~`: none (refused), unless identified by origin;
  - inside the store: none (refused); there, the current notebook is the
    enclosing notebook folder.
- Ids are plain names: each part matches `[A-Za-z0-9._-]+`, never `.` or `..`.
- `.source` records the origin at init time, as `id=value` lines with paths
  from `~`, so the store moves between machines unchanged:

  ```
  id=dixie6502/folio
  path=~/GitHub/dixie6502/folio
  origin=git@github.com:dixie6502/folio.git
  ```

  `origin` is empty for a folder identified by its path, and drops any credentials
  (`https://user:token@host/…` → `https://host/…`), since the store is synced.
  `path` is empty for a repository outside `~`; `check` then has no folder to
  verify.

### Finding the current notebook

From the current folder: the origin id, if inside a repository with one; then
the path id of each folder walking up to `~`. The first id with a notebook in
the store wins. None: `no notebook here: folio init`, exit 1.

### Commands

- `folio init`: creates the notebook of the current folder under its id
  (origin id in a repository, else the current folder's path id), copies the
  template's contents into it (`$FOLIO_TEMPLATE`, default `<store>/.template`;
  missing or empty: nothing copied), writes `.source`, and prints the path. It
  builds the notebook in a hidden staging folder and moves it into place, so a
  failure leaves nothing behind. Already a notebook: prints the path, exit 0,
  changes nothing.
- `folio [-c] [<query>]`: the switcher, as today, over the current notebook.
- `folio --all [<query>]`: the switcher over every notebook, each note shown as
  `<id>/<name>`. Opens only: creating from it is refused. Notebooks under `_*`
  folders are left out, like the notes there.
- `--` ends the switcher's flags: `folio -c -- -draft` searches for `-draft`.
- `folio --dir`: prints the current notebook's path, exit 1 if none. For scripts
  and Claude skills, which cannot drive the switcher.
- `folio check`: the store's health. Changes nothing; exit 1 when it reports
  anything. For each notebook:
  - **orphan**: the folder recorded in `.source` no longer exists (moved or
    deleted; from its new place, `folio map` re-attaches the notebook);
  - **id changed**: the folder exists but its id now differs (origin added or
    changed, repo renamed or transferred): `folio map <old-id>` from it;
  - **bad record**: `.source` unreadable or its `id` not the notebook's folder.

  And outside notebooks: **stray** files or folders in the store (not `.template`,
  not under a notebook).
- `folio map <old-id>`: run from a folder, moves the notebook `<old-id>` to the
  current folder's id and rewrites its `.source`. Refuses when `<old-id>` is
  not a notebook, or the current id already has one. Empty parent folders left
  by the move are removed.

### Unchanged

`-c`, `#tags` display and search, `$EDITOR`, `_`/`.` files skipped (so
`.source` and the template never show), case-insensitive reuse on create,
`.gitignore` honoured.

### Tests

`bats-core`, pinned, installed at user level with `mise` (and added to
construct's bootstrap). Tests run against a temporary `$HOME` and store, with
real git repositories whose origins are set locally (no network): id from each
URL form, the walk-up, refusals, `init` idempotence and template copy, `check`
for each finding, `map` and its refusals, `--dir`. The switcher itself (fzf) is
tested through its listing function, not interactively.

## Effects on mneme

- The `memo` store (`~/memo`) becomes `folio`'s store, in place. Its layout flattens: today's
  `<scope>/TODO.md`, `<scope>/note/` and `<scope>/todo/` become one notebook per
  id, `TODO.md` and item details being ordinary notes in it, with a `.source`
  written for each. A migration script does it once.
- The `todo` and `note` skills find the notebook with `folio --dir` instead of
  `memo.sh dir`; `memo.sh` keeps only the sync.
- `debrief` reads every notebook in the store and lists `folio check` findings
  under "Needs you".
- `global.md` needs no rule about where open items or notes live.
