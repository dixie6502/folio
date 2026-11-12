#!/usr/bin/env bats
# folio's notebook commands, against a temporary $HOME and store. Repositories
# get their origin with `git remote add`, which never touches the network.

setup() {
  FOLIO="$BATS_TEST_DIRNAME/../folio"
  export HOME="$BATS_TEST_TMPDIR/home"
  # A git hook exports these, and git obeys them before -C: never touch the
  # repository the suite runs from.
  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR
  unset FOLIO_HOME FOLIO_TEMPLATE
  STORE="$HOME/memo"
  mkdir -p "$HOME"
}

# Creates a git repository at $1 with origin $2 (no origin when empty).
make_repo() {
  mkdir -p "$1"
  git -C "$1" init -q
  [[ -z ${2:-} ]] || git -C "$1" remote add origin "$2"
}

# Creates folder $1 and initializes its notebook.
init_at() {
  mkdir -p "$1"
  folio_in "$1" init >/dev/null
}

# Runs folio from directory $1 with the remaining arguments.
folio_in() {
  local dir=$1
  shift
  (cd "$dir" && "$FOLIO" "$@")
}

# ---------------------------------------------------------------- ids

@test "a repository's id comes from origin, whatever the URL form" {
  local url
  for url in https://github.com/owner/repo.git git@github.com:owner/repo.git \
    ssh://git@example.org:2222/owner/repo https://example.org/owner/repo/; do
    rm -rf "$HOME/r"
    make_repo "$HOME/r" "$url"
    run folio_in "$HOME/r" init
    [ "$status" -eq 0 ]
    [ "$output" = "$STORE/owner/repo" ]
    rm -rf "$STORE"
  done
}

@test "nested groups keep every part of the origin path" {
  make_repo "$HOME/r" git@example.org:group/sub/repo.git
  run folio_in "$HOME/r" init
  [ "$output" = "$STORE/group/sub/repo" ]
}

@test "every checkout of a repo, anywhere, opens the same notebook" {
  make_repo "$HOME/GitHub/owner/repo" git@github.com:owner/repo.git
  make_repo "$HOME/elsewhere/copy" https://github.com/owner/repo
  folio_in "$HOME/GitHub/owner/repo" init
  run folio_in "$HOME/elsewhere/copy" --dir
  [ "$status" -eq 0 ]
  [ "$output" = "$STORE/owner/repo" ]
}

@test "a subfolder of a repository uses the repository's notebook" {
  make_repo "$HOME/r" git@github.com:owner/repo.git
  mkdir -p "$HOME/r/src/deep"
  folio_in "$HOME/r" init
  run folio_in "$HOME/r/src/deep" --dir
  [ "$output" = "$STORE/owner/repo" ]
}

@test "a folder outside any repository is identified by its path from ~" {
  mkdir -p "$HOME/scratch/topic"
  run folio_in "$HOME/scratch/topic" init
  [ "$status" -eq 0 ]
  [ "$output" = "$STORE/scratch/topic" ]
}

@test "a repository without origin is identified by its path" {
  make_repo "$HOME/scratch/local"
  run folio_in "$HOME/scratch/local" init
  [ "$output" = "$STORE/scratch/local" ]
}

@test "folders outside ~, and ~ itself, are refused" {
  run folio_in / init
  [ "$status" -eq 1 ]
  [[ $output == *"outside"* ]]
  run folio_in "$HOME" init
  [ "$status" -eq 1 ]
}

@test "an origin path with unsafe parts is refused" {
  make_repo "$HOME/r" "https://example.org/owner/../repo"
  run folio_in "$HOME/r" init
  [ "$status" -eq 1 ]
  [[ $output == *"unsafe"* ]]
  [ ! -e "$STORE" ]
}

@test "a repository outside ~ records no path, and check accepts it" {
  make_repo "$BATS_TEST_TMPDIR/outside" git@github.com:owner/repo.git
  run folio_in "$BATS_TEST_TMPDIR/outside" init
  [ "$status" -eq 0 ]
  grep -qx "path=" "$STORE/owner/repo/.source"
  run "$FOLIO" check
  [ "$status" -eq 0 ]
}

@test "origin ids are lowercased: forges ignore case in owner/repo" {
  make_repo "$HOME/a" git@github.com:Owner/Repo.git
  make_repo "$HOME/b" https://github.com/owner/repo
  folio_in "$HOME/a" init
  run folio_in "$HOME/b" --dir
  [ "$output" = "$STORE/owner/repo" ]
}

@test "credentials in an origin URL never reach .source" {
  make_repo "$HOME/r" "https://me:secret-token@example.org/owner/repo.git"
  folio_in "$HOME/r" init
  grep -qx "origin=https://example.org/owner/repo.git" "$STORE/owner/repo/.source"
  run grep -q secret-token "$STORE/owner/repo/.source"
  [ "$status" -eq 1 ]
}

@test "a folder reached through a symlink under ~ keeps its path from ~" {
  mkdir -p "$BATS_TEST_TMPDIR/data/proj"
  ln -s "$BATS_TEST_TMPDIR/data" "$HOME/link"
  run folio_in "$HOME/link/proj" init
  [ "$status" -eq 0 ]
  [ "$output" = "$STORE/link/proj" ]
  grep -qx "path=~/link/proj" "$STORE/link/proj/.source"
}

@test "GIT_DIR from a calling hook does not change the folder's id" {
  make_repo "$HOME/r" git@github.com:owner/repo.git
  make_repo "$HOME/other" git@github.com:owner/other.git
  run env GIT_DIR="$HOME/other/.git" bash -c "cd '$HOME/r' && '$FOLIO' init"
  [ "$output" = "$STORE/owner/repo" ]
}

# ---------------------------------------------------------------- finding

@test "inside a notebook's own folder, that notebook is current; init refuses" {
  init_at "$HOME/scratch/t"
  mkdir -p "$STORE/scratch/t/sub"
  run folio_in "$STORE/scratch/t/sub" --dir
  [ "$status" -eq 0 ]
  [ "$output" = "$STORE/scratch/t" ]
  run folio_in "$STORE/scratch/t" init
  [ "$status" -eq 1 ]
  [[ $output == *"inside the store"* ]]
}

@test "-- ends the switcher's flags, so any query can be typed" {
  run bash -c "source '$FOLIO' && parse_switch_args 1 -c -- -draft && echo \"\$SWITCH_MODE \$SWITCH_QUERY\""
  [ "$output" = "enter -draft" ]
  run bash -c "source '$FOLIO' && parse_switch_args 1 -- init && echo \"\$SWITCH_MODE \$SWITCH_QUERY\""
  [ "$output" = "ctrl-n init" ]
}

@test "--dir refuses where no notebook exists" {
  mkdir -p "$HOME/scratch/none"
  run folio_in "$HOME/scratch/none" --dir
  [ "$status" -eq 1 ]
  [[ $output == *"no notebook here"* ]]
}

@test "the nearest initialized folder walking up wins" {
  mkdir -p "$HOME/scratch/topic/a/b"
  folio_in "$HOME/scratch" init
  run folio_in "$HOME/scratch/topic/a/b" --dir
  [ "$output" = "$STORE/scratch" ]
}

@test "a repository's origin id is tried before the folders above it" {
  init_at "$HOME/scratch"
  make_repo "$HOME/scratch/r" git@github.com:owner/repo.git
  run folio_in "$HOME/scratch/r" --dir
  [ "$output" = "$STORE/scratch" ]
  folio_in "$HOME/scratch/r" init
  run folio_in "$HOME/scratch/r" --dir
  [ "$output" = "$STORE/owner/repo" ]
}

@test "the bare switcher refuses where no notebook exists" {
  mkdir -p "$HOME/x"
  run folio_in "$HOME/x"
  [ "$status" -eq 1 ]
  [[ $output == *"no notebook here"* ]]
}

# ---------------------------------------------------------------- init

@test "init writes .source with paths from ~" {
  make_repo "$HOME/GitHub/owner/repo" git@github.com:owner/repo.git
  folio_in "$HOME/GitHub/owner/repo" init
  run cat "$STORE/owner/repo/.source"
  [ "${lines[0]}" = "id=owner/repo" ]
  [ "${lines[1]}" = "path=~/GitHub/owner/repo" ]
  [ "${lines[2]}" = "origin=git@github.com:owner/repo.git" ]
}

@test "a .source record passes gitleaks: the store is pushed through it" {
  command -v gitleaks >/dev/null || skip "gitleaks not installed"
  make_repo "$HOME/GitHub/owner/repo" https://github.com/owner/repo.git
  folio_in "$HOME/GitHub/owner/repo" init
  run gitleaks dir "$STORE" --no-banner --redact
  [ "$status" -eq 0 ]
}

@test "init copies the template without overwriting, and is idempotent" {
  mkdir -p "$STORE/.template" "$HOME/scratch/t"
  touch "$STORE/.template/.wikijump"
  echo template >"$STORE/.template/README.md"
  folio_in "$HOME/scratch/t" init
  [ -f "$STORE/scratch/t/.wikijump" ]
  echo mine >"$STORE/scratch/t/README.md"
  run folio_in "$HOME/scratch/t" init
  [ "$status" -eq 0 ]
  [ "$(cat "$STORE/scratch/t/README.md")" = mine ]
}

@test "init honours FOLIO_HOME and FOLIO_TEMPLATE" {
  export FOLIO_HOME="$HOME/other" FOLIO_TEMPLATE="$HOME/tpl"
  mkdir -p "$FOLIO_TEMPLATE" "$HOME/scratch/t"
  touch "$FOLIO_TEMPLATE/.marker"
  run folio_in "$HOME/scratch/t" init
  [ "$output" = "$HOME/other/scratch/t" ]
  [ -f "$HOME/other/scratch/t/.marker" ]
}

@test "init without a template creates an empty notebook" {
  init_at "$HOME/scratch/t"
  [ "$(ls -A "$STORE/scratch/t")" = .source ]
}

@test "init refuses inside another notebook or over a non-notebook folder" {
  mkdir -p "$HOME/scratch/a/b" "$HOME/scratch/c" "$STORE/scratch/c"
  folio_in "$HOME/scratch/a" init
  run folio_in "$HOME/scratch/a/b" init
  [ "$status" -eq 1 ]
  [[ $output == *"inside the notebook scratch/a"* ]]
  run folio_in "$HOME/scratch/c" init
  [ "$status" -eq 1 ]
  [[ $output == *"already has a notebook (or a folder)"* ]]
}

# ---------------------------------------------------------------- listing

@test "the listing of all notebooks names each note by its id" {
  mkdir -p "$HOME/scratch/a" "$HOME/scratch/b"
  folio_in "$HOME/scratch/a" init
  folio_in "$HOME/scratch/b" init
  printf '#moc\n' >"$STORE/scratch/a/one.md"
  printf 'x\n' >"$STORE/scratch/b/two.md"
  printf 'stray\n' >"$STORE/loose.md"
  run bash -c "source '$FOLIO' && mapfile -t nbs < <(list_notebooks) && notes_with_tags '$STORE' \"\${nbs[@]}\""
  [ "$status" -eq 0 ]
  [[ $output == *"scratch/a/one"*"#moc"* ]]
  [[ $output == *"scratch/b/two"* ]]
  [[ $output != *"loose"* ]]
}

@test "--all searches nested notebooks once and skips _* folders" {
  init_at "$HOME/scratch"
  mkdir -p "$STORE/scratch/inner" "$STORE/_archive/old"
  printf 'id=scratch/inner\npath=~/x\norigin=\n' >"$STORE/scratch/inner/.source"
  printf 'id=_archive/old\npath=~/y\norigin=\n' >"$STORE/_archive/old/.source"
  run bash -c "source '$FOLIO' && list_search_roots"
  [ "$output" = "$STORE/scratch" ]
}

@test "init leaves nothing behind when it fails, and works once fixed" {
  mkdir -p "$STORE/.template" "$HOME/scratch/t"
  echo secret >"$STORE/.template/unreadable.md"
  chmod 000 "$STORE/.template/unreadable.md"
  run folio_in "$HOME/scratch/t" init
  [ "$status" -eq 1 ]
  [ ! -e "$STORE/scratch" ]
  [ -z "$(find "$STORE" -maxdepth 1 -name '.init.*')" ]
  chmod 600 "$STORE/.template/unreadable.md"
  run folio_in "$HOME/scratch/t" init
  [ "$status" -eq 0 ]
}

@test "--all refuses -c: it never creates" {
  init_at "$HOME/scratch/a"
  run folio_in "$HOME" --all -c
  [ "$status" -eq 1 ]
  [[ $output == *"never creates"* ]]
}

# ---------------------------------------------------------------- check

@test "check passes on a healthy store and ignores hidden entries" {
  make_repo "$HOME/r" git@github.com:owner/repo.git
  folio_in "$HOME/r" init
  mkdir -p "$STORE/.git" "$STORE/.template"
  touch "$STORE/.gitignore"
  run "$FOLIO" check
  [ "$status" -eq 0 ]
}

@test "check reports an orphan when the folder is gone" {
  init_at "$HOME/scratch/gone"
  rm -rf "$HOME/scratch/gone"
  run "$FOLIO" check
  [ "$status" -eq 1 ]
  [[ $output == *"orphan scratch/gone"* ]]
}

@test "check reports a changed id when origin changes" {
  make_repo "$HOME/r" git@github.com:owner/old.git
  folio_in "$HOME/r" init
  git -C "$HOME/r" remote set-url origin git@github.com:owner/new.git
  run "$FOLIO" check
  [ "$status" -eq 1 ]
  [[ $output == *"id changed owner/old"*"owner/new"* ]]
}

@test "check reports bad records and strays" {
  mkdir -p "$HOME/scratch/a" "$STORE/stray-dir"
  folio_in "$HOME/scratch/a" init
  sed -i 's/^id=.*/id=wrong/' "$STORE/scratch/a/.source"
  touch "$STORE/loose.md"
  run "$FOLIO" check
  [ "$status" -eq 1 ]
  [[ $output == *"bad record scratch/a"* ]]
  [[ $output == *"stray loose.md"* ]]
  [[ $output == *"stray stray-dir"* ]]
}

# ---------------------------------------------------------------- map

@test "map moves a notebook to the folder's new id" {
  make_repo "$HOME/r" git@github.com:owner/old.git
  folio_in "$HOME/r" init
  echo note >"$STORE/owner/old/n.md"
  git -C "$HOME/r" remote set-url origin git@github.com:owner/new.git
  run folio_in "$HOME/r" map owner/old
  [ "$status" -eq 0 ]
  [ "$(cat "$STORE/owner/new/n.md")" = note ]
  [ ! -e "$STORE/owner/old" ]
  grep -qx "id=owner/new" "$STORE/owner/new/.source"
  run "$FOLIO" check
  [ "$status" -eq 0 ]
}

@test "map re-attaches an orphan from the folder's new place" {
  init_at "$HOME/scratch/before"
  mv "$HOME/scratch/before" "$HOME/scratch/after"
  run folio_in "$HOME/scratch/after" map scratch/before
  [ "$status" -eq 0 ]
  [ ! -e "$STORE/scratch/before" ]
  grep -qx "path=~/scratch/after" "$STORE/scratch/after/.source"
}

@test "map removes empty parents left behind" {
  make_repo "$HOME/r" git@github.com:solo/repo.git
  folio_in "$HOME/r" init
  git -C "$HOME/r" remote set-url origin git@github.com:other/repo.git
  folio_in "$HOME/r" map solo/repo
  [ ! -e "$STORE/solo" ]
}

@test "map refuses an unknown old id or an existing target" {
  mkdir -p "$HOME/scratch/a" "$HOME/scratch/b"
  folio_in "$HOME/scratch/a" init
  folio_in "$HOME/scratch/b" init
  run folio_in "$HOME/scratch/b" map scratch/none
  [ "$status" -eq 1 ]
  [[ $output == *"not a notebook"* ]]
  run folio_in "$HOME/scratch/b" map scratch/a
  [ "$status" -eq 1 ]
  [[ $output == *"already has a notebook"* ]]
}

@test "map refuses an unsafe old id" {
  mkdir -p "$HOME/scratch/a"
  run folio_in "$HOME/scratch/a" map ../etc
  [ "$status" -eq 1 ]
  [[ $output == *"unsafe"* ]]
}

# ---------------------------------------------------------------- forget

@test "forget deletes an orphan and its empty parents after a yes" {
  init_at "$HOME/scratch/gone"
  echo note >"$STORE/scratch/gone/n.md"
  rm -rf "$HOME/scratch/gone"
  run bash -c "echo y | '$FOLIO' forget scratch/gone"
  [ "$status" -eq 0 ]
  [[ $output == *"1 note(s)"* && $output == *"forgot scratch/gone"* ]]
  [ ! -e "$STORE/scratch" ]
  [ -d "$STORE" ]
}

@test "forget keeps the notebook without a yes" {
  init_at "$HOME/scratch/gone"
  rm -rf "$HOME/scratch/gone"
  local answer
  for answer in n "" yesplease; do
    run bash -c "printf '%s\n' '$answer' | '$FOLIO' forget scratch/gone"
    [ "$status" -eq 1 ]
    [[ $output == *"kept scratch/gone"* ]]
  done
  run "$FOLIO" forget scratch/gone </dev/null
  [ "$status" -eq 1 ]
  [ -f "$STORE/scratch/gone/.source" ]
}

@test "forget refuses a notebook whose folder exists or changed id" {
  init_at "$HOME/scratch/live"
  make_repo "$HOME/r" git@github.com:owner/old.git
  folio_in "$HOME/r" init
  git -C "$HOME/r" remote set-url origin git@github.com:owner/new.git
  local id
  for id in scratch/live owner/old; do
    run bash -c "echo y | '$FOLIO' forget $id"
    [ "$status" -eq 1 ]
    [[ $output == *"not an orphan"* ]]
    [ -f "$STORE/$id/.source" ]
  done
}

@test "forget refuses a notebook initialized outside ~ or badly recorded" {
  make_repo "$BATS_TEST_TMPDIR/outside" git@github.com:owner/out.git
  folio_in "$BATS_TEST_TMPDIR/outside" init
  rm -rf "$BATS_TEST_TMPDIR/outside"
  init_at "$HOME/scratch/bad"
  rm -rf "$HOME/scratch/bad"
  sed -i 's/^id=.*/id=wrong/' "$STORE/scratch/bad/.source"
  local id
  for id in owner/out scratch/bad; do
    run bash -c "echo y | '$FOLIO' forget $id"
    [ "$status" -eq 1 ]
    [ -f "$STORE/$id/.source" ]
  done
}

@test "forget refuses non-notebooks and unsafe ids" {
  mkdir -p "$STORE/stray-dir"
  run bash -c "echo y | '$FOLIO' forget stray-dir"
  [ "$status" -eq 1 ]
  [[ $output == *"not a notebook"* ]]
  [ -d "$STORE/stray-dir" ]
  run bash -c "echo y | '$FOLIO' forget ../etc"
  [ "$status" -eq 1 ]
  [[ $output == *"unsafe"* ]]
  run "$FOLIO" forget
  [ "$status" -eq 1 ]
  [[ $output == *"usage"* ]]
}

@test "forget refuses an orphan holding another notebook" {
  init_at "$HOME/scratch/outer"
  rm -rf "$HOME/scratch/outer"
  # init refuses nesting; a hand move into the store can still nest one.
  mkdir -p "$STORE/scratch/outer/inner"
  printf 'id=scratch/outer/inner\npath=~/scratch/outer/inner\norigin=\n' \
    >"$STORE/scratch/outer/inner/.source"
  run bash -c "echo y | '$FOLIO' forget scratch/outer"
  [ "$status" -eq 1 ]
  [[ $output == *"holds the notebook"* ]]
  [ -f "$STORE/scratch/outer/inner/.source" ]
}

# ---------------------------------------------------------------- misc

@test "help lists the commands" {
  run "$FOLIO" help
  [ "$status" -eq 0 ]
  [[ $output == *"folio init"* && $output == *"folio map"* && $output == *"folio forget"* ]]
}
