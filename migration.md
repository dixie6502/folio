# Migration: one notebook per folder

Draft spec, under discussion. Extends and simplifies `note` so it can hold every
project's notes and open items, outside the project repos, for use both in a
terminal and by Claude (mneme's `memo` plugin).

## Decided

- `note` stays a switcher: filter, open, create. No link commands; links are
  `wikijump.vim` in the editor and `wkref` on the command line.
- No built-in global notebook. A notebook is tied to a folder and exists only
  once initialized there.
- A todo list is an ordinary note (`TODO.md`). Its format belongs to the `todo`
  skill; `note` knows nothing about it. Likewise any frontmatter (a `created:`
  date) is a skill's convention, not `note`'s.
- `note` is unaware of git. Versioning and sync stay outside (mneme's hooks).
- Stays in Bash for now; a Go port comes when dropping `fzf`/`rg` is wanted
  (see README › Plan).

## Spec

### Store and notebooks

- The store is one folder (variable and default: see Open questions), holding
  one notebook per initialized folder:

  ```
  <store>/
    <owner>/<repo>/        a git repository, keyed by its origin
      .wikijump            root marker for wikijump.vim
      TODO.md
      auth-flow.md
      research/caching.md  subfolders allowed, as today
    scratch/<topic>/       any other folder: its path relative to ~
  ```

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
- A key changes when its source does (origin added, repo renamed or transferred,
  folder moved): the notebook is then moved by hand in the store.

### Commands

- `note init`: creates the notebook of the current folder (`<store>/<key>/` and
  its `.wikijump`) and prints its path. Already there: prints the path, exit 0.
- `note [-c] [<query>]`: as today, over the current notebook. The current
  notebook is the nearest initialized one walking up from the current folder
  (inside a repository, its origin key is tried first). None: refuses with
  `no notebook here: note init`, exit 1.
- `note --all [<query>]`: the switcher over every notebook, each note shown as
  `<key>/<name>`. Creating from it needs a query starting with an existing key.
- `note --dir`: prints the current notebook's path, exit 1 if none. The one
  non-interactive command: used by scripts and Claude skills, which cannot drive
  the switcher.

### Unchanged

`-c`, `#tags` display and search, `$EDITOR`, `_`/`.` files skipped, case-insensitive
reuse on create, `.gitignore` honoured.

## Effects on mneme

- The `memo` store becomes `note`'s store. Its layout flattens: today's
  `<scope>/TODO.md`, `<scope>/note/` and `<scope>/todo/` become one notebook per
  key, `TODO.md` and item details being ordinary notes in it.
- The `todo` and `note` skills find the notebook with `note --dir` instead of
  `memo.sh dir`; `memo.sh` keeps only the sync.
- `debrief` reads every notebook in the store.
- `global.md` needs no rule about where open items or notes live.

## Open questions

1. Store variable and default: keep `NOTE_HOME` (`~/notes`) and move `~/memo`
   there, or another name?
2. A changed key (see Store and notebooks) needs a manual `mv` in the store.
   Acceptable, or should `note init` offer to move the old notebook?
3. Tests: plain Bash test scripts (no new dependency) or `bats`?
4. `note --all` creation: require the `<key>/` prefix (proposed), or refuse
   creating from `--all`?
