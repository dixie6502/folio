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

### Template and timestamps (vim)

`note` only opens the editor; templates and timestamps live in vim, so they
also apply to notes you create by following a link. Add this to your vimrc:

- **Template:** a new note under `$NOTE_HOME` starts as a copy of
  `$NOTE_HOME/_template.md` (if present), with `{{title}}` (file name
  without `.md`) and `{{date}}` (`2026-11-06`) filled in. Nothing is
  written until you save.
- **Stamps:** on a save that changes a note (or writes it for the first
  time), an empty `creation:` is filled once and `update:` is refreshed,
  inside front matter starting with `---` on line 1.
- Files and folders starting with `_` or `.` (including the template) are
  left alone. `NOTE_HOME` must be exported in the shell vim starts from.

Example `_template.md`:

```markdown
---
creation:
update:
---

# {{title}}
```

```vim
" Notes under $NOTE_HOME: fill new ones from _template.md, and stamp
" creation:/update: in their front matter on save.

" True for a note under $NOTE_HOME whose path has no _* or .* part.
function! s:IsNote(path) abort
  if empty($NOTE_HOME) | return 0 | endif
  let l:home = fnamemodify($NOTE_HOME, ':p')
  let l:path = fnamemodify(a:path, ':p')
  if stridx(l:path, l:home) != 0 | return 0 | endif
  return l:path[len(l:home):] !~# '\v(^|/)[_.]'
endfunction

" Fill a new note from the template; {{title}} and {{date}} are replaced.
function! s:NoteTemplate() abort
  let l:template = $NOTE_HOME . '/_template.md'
  if !s:IsNote(expand('<afile>')) || !filereadable(l:template) | return | endif
  let l:title = expand('<afile>:t:r')
  let l:date = strftime('%Y-%m-%d')
  let l:lines = []
  for l:line in readfile(l:template)
    let l:line = join(split(l:line, '{{title}}', 1), l:title)
    call add(l:lines, join(split(l:line, '{{date}}', 1), l:date))
  endfor
  call setline(1, l:lines)
  setlocal nomodified
endfunction

" Fill an empty creation: once and refresh update: on a save that changes
" the note (or writes it for the first time).
function! s:NoteStamp() abort
  let l:file = expand('<afile>')
  if !s:IsNote(l:file) || (!&modified && filereadable(l:file)) | return | endif
  if getline(1) !~# '^---\s*$' | return | endif
  let l:end = 0
  for l:lnum in range(2, min([line('$'), 50]))
    if getline(l:lnum) =~# '^---\s*$' | let l:end = l:lnum | break | endif
  endfor
  if l:end == 0 | return | endif
  let l:stamp = strftime('%Y-%m-%dT%H:%M:%S%z')
  for l:lnum in range(2, l:end - 1)
    if getline(l:lnum) =~# '^creation:\s*$'
      call setline(l:lnum, 'creation: ' . l:stamp)
    elseif getline(l:lnum) =~# '^update:'
      call setline(l:lnum, 'update: ' . l:stamp)
    endif
  endfor
endfunction

augroup note
  autocmd!
  autocmd BufNewFile $NOTE_HOME/*.md call s:NoteTemplate()
  autocmd BufWritePre $NOTE_HOME/*.md call s:NoteStamp()
augroup END
```

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
