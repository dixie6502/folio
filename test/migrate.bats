#!/usr/bin/env bats
# migrate/stores.sh, against a temporary $HOME and an old store made by the
# Bash folio itself (so its records are real), whose remote is a local bare
# repository.

setup() {
  MIGRATE="$BATS_TEST_DIRNAME/../migrate/stores.sh"
  FOLIO="$BATS_TEST_DIRNAME/../folio"
  export HOME="$BATS_TEST_TMPDIR/home"
  # A git hook exports these, and git obeys them before -C: never touch the
  # repository the suite runs from.
  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR FOLIO_HOME FOLIO_TEMPLATE
  export GIT_CONFIG_GLOBAL="$HOME/.gitconfig" GIT_CONFIG_NOSYSTEM=1
  export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid
  export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
  OLD="$HOME/memo"
  OUT="$BATS_TEST_TMPDIR/out"
  mkdir -p "$HOME"
}

# Creates folder $1 (a repository with origin $2, if given) and its notebook in
# the old store with the Bash folio, with a note in it; prints the notebook.
init_notebook() {
  mkdir -p "$1"
  if [[ -n ${2:-} ]]; then
    git -C "$1" init -q
    git -C "$1" remote add origin "$2"
  fi
  local notebook
  notebook=$(cd "$1" && "$FOLIO" init)
  printf 'notes of %s\n' "${notebook#"$OLD"/}" >"$notebook/TODO.md"
}

# Makes the old store a repository synced with a bare remote, all committed.
publish_old() {
  git init -q --bare -b main "$HOME/memo.git"
  git -C "$OLD" init -q -b main
  git -C "$OLD" remote add origin "$HOME/memo.git"
  commit_old "first"
  git -C "$OLD" push -q -u origin main
}

# Commits everything in the old store, and pushes once it tracks a remote.
commit_old() {
  git -C "$OLD" add --all
  git -C "$OLD" commit -q -m "$1"
  git -C "$OLD" rev-parse -q --verify '@{upstream}' >/dev/null && git -C "$OLD" push -q
  return 0
}

# An old store with two owners' repositories and two scratch folders, one of
# them since deleted; owner/repo's note changed in a second commit.
seed() {
  init_notebook "$HOME/GitHub/owner/repo" https://example.org/owner/repo.git
  init_notebook "$HOME/GitHub/owner/Other" git@example.org:owner/other.git
  init_notebook "$HOME/GitHub/client/app" https://example.org/client/app.git
  init_notebook "$HOME/scratch/topic"
  init_notebook "$HOME/scratch/gone"
  rm -rf "$HOME/scratch/gone"
  publish_old
  printf 'more\n' >>"$OLD/owner/repo/TODO.md"
  commit_old "second"
}

@test "stores: a dry run prints the plan and changes nothing" {
  seed
  run "$MIGRATE" stores "$OUT"
  [ "$status" -eq 0 ]
  [[ $output == *"store $OUT/GitHub/owner/notebooks"* ]]
  [[ $output == *"owner/other -> Other"* ]]
  [[ $output == *"store $OUT/GitHub/client/notebooks"* ]]
  [[ $output == *"dry run"* ]]
  [ ! -e "$OUT" ]
}

@test "stores: builds one store per owner in staging, history kept, markers swapped" {
  seed
  run "$MIGRATE" stores "$OUT" --apply
  [ "$status" -eq 0 ]
  store="$OUT/GitHub/owner/notebooks"
  [ -f "$store/.folio" ]
  [ -f "$store/repo/.notebook" ]
  [ ! -e "$store/repo/.source" ]
  [ "$(cat "$store/Other/TODO.md")" = "notes of owner/other" ]
  [ ! -e "$store/app" ]
  # The note's history came along: both commits that touched it.
  [ "$(git -C "$store" log --format=%s -- repo/TODO.md | tr '\n' ' ')" = "second first " ]
  # Nothing of another owner or of scratch in this store's history.
  [ -z "$(git -C "$store" log --all --format=%H -- client scratch app)" ]
  [ -z "$(git -C "$store" remote)" ]
  [ -f "$OUT/GitHub/client/notebooks/app/.notebook" ]
  [[ $output == *"git clone --filter=blob:none --sparse"* ]]
  # Nothing placed in the home folder, and the old store untouched.
  [ ! -e "$HOME/GitHub/owner/notebooks" ]
  [ -f "$OLD/owner/repo/.source" ]
}

@test "stores: refuses a staging folder that is not empty" {
  seed
  mkdir -p "$OUT/something"
  run "$MIGRATE" stores "$OUT"
  [ "$status" -eq 1 ]
  [[ $output == *"is not empty"* ]]
}

@test "stores: refuses an old store with changes not pushed" {
  seed
  printf 'edit\n' >>"$OLD/owner/repo/TODO.md"
  run "$MIGRATE" stores "$OUT"
  [ "$status" -eq 1 ]
  [[ $output == *"uncommitted changes"* ]]
  git -C "$OLD" commit -q -am "local"
  run "$MIGRATE" stores "$OUT"
  [ "$status" -eq 1 ]
  [[ $output == *"unpushed commits"* ]]
}

@test "stores: lists a repository outside ~/<host>/<owner>/<repo> as not migrated" {
  init_notebook "$HOME/GitHub/owner/deep/sub" https://example.org/owner/deep.git
  publish_old
  run "$MIGRATE" stores "$OUT"
  [ "$status" -eq 0 ]
  [[ $output == *"not migrated"* ]]
  [[ $output == *"owner/deep: ~/GitHub/owner/deep/sub is not ~/<host>/<owner>/<repo>"* ]]
}

@test "local: copies this machine's folders' notebooks into notes/" {
  seed
  run "$MIGRATE" local
  [ "$status" -eq 0 ]
  [[ $output == *"local $HOME/scratch/topic/notes <- scratch/topic"* ]]
  [[ $output == *"scratch/gone: $HOME/scratch/gone is not on this machine"* ]]
  [ ! -e "$HOME/scratch/topic/notes" ]

  run "$MIGRATE" local --apply
  [ "$status" -eq 0 ]
  [ -f "$HOME/scratch/topic/notes/.notebook" ]
  [ ! -e "$HOME/scratch/topic/notes/.source" ]
  [ "$(cat "$HOME/scratch/topic/notes/TODO.md")" = "notes of scratch/topic" ]
  [ -z "$(find "$HOME/scratch/topic" -name '.notes-migrate.*')" ]
}

@test "local: leaves a folder that became a repository, and an existing notes/" {
  seed
  mkdir -p "$HOME/scratch/topic/notes" "$HOME/scratch/gone"
  git -C "$HOME/scratch/gone" init -q
  run "$MIGRATE" local --apply
  [ "$status" -eq 0 ]
  [[ $output == *"scratch/topic: $HOME/scratch/topic/notes exists"* ]]
  [[ $output == *"scratch/gone: $HOME/scratch/gone became a repository"* ]]
  [ ! -e "$HOME/scratch/gone/notes" ]
}

@test "an unknown mode or option is refused" {
  run "$MIGRATE" everything
  [ "$status" -eq 1 ]
  run "$MIGRATE" local --force
  [ "$status" -eq 1 ]
  run "$MIGRATE" stores
  [ "$status" -eq 1 ]
}

@test "stores: refuses an old store that tracks no upstream" {
  seed
  git -C "$OLD" branch -q --unset-upstream
  run "$MIGRATE" stores "$OUT"
  [ "$status" -eq 1 ]
  [[ $output == *"tracks no upstream branch"* ]]
}

@test "stores: lists repositories it cannot place, and builds the rest" {
  init_notebook "$HOME/GitHub/owner/repo" https://example.org/owner/repo.git
  init_notebook "$BATS_TEST_TMPDIR/outside" https://example.org/owner/outside.git
  init_notebook "$HOME/scratch/topic/tool" https://example.org/owner/tool.git
  # A repository renamed upstream: old and new ids recorded one path.
  init_notebook "$HOME/GitHub/owner/bar" https://example.org/owner/foo.git
  git -C "$HOME/GitHub/owner/bar" remote set-url origin https://example.org/owner/bar.git
  (cd "$HOME/GitHub/owner/bar" && "$FOLIO" init >/dev/null)
  # A notebook inside another (written by hand: folio init refuses it).
  mkdir -p "$OLD/owner/repo/inner"
  printf 'id=owner/repo/inner\npath=~/GitHub/owner/repo/inner\norigin=x\n' >"$OLD/owner/repo/inner/.source"
  publish_old

  run "$MIGRATE" stores "$OUT" --apply
  [ "$status" -eq 0 ]
  [[ $output == *"owner/outside: a repository outside ~ (no path recorded)"* ]]
  [[ $output == *"owner/tool: ~/scratch/topic/tool is not ~/<host>/<owner>/<repo>"* ]]
  [[ $output == *"owner/foo: shares bar in $OUT/GitHub/owner/notebooks with another notebook"* ]]
  [[ $output == *"owner/bar: shares bar"* ]]
  [[ $output == *"owner/repo: nested with the notebook owner/repo/inner"* ]]
  [[ $output == *"owner/repo/inner: nested with the notebook owner/repo"* ]]
  # Nothing placed for any of them; the store exists only if a notebook fits.
  [ ! -e "$OUT/GitHub/owner/notebooks/bar" ]
  [ ! -e "$OUT/GitHub/owner/notebooks/repo" ]
  [ ! -e "$OUT/scratch" ]
}

@test "local: a migrated notebook follows the umask, not mktemp's privacy" {
  init_notebook "$HOME/scratch/topic"
  umask 022
  run "$MIGRATE" local --apply
  [ "$status" -eq 0 ]
  [ "$(stat -c %a "$HOME/scratch/topic/notes")" = 755 ]
}
