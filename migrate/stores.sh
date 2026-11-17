#!/usr/bin/env bash
# One-off migration from the Bash folio's central store ($FOLIO_HOME, default
# ~/memo) to the layout of migration.md. Never changes the old store. Delete
# this script and test/migrate.bats once every machine is migrated.
#
# Usage: migrate/stores.sh stores <staging folder> [--apply]
#            Once, on any machine: builds one store per owner folder,
#            <staging>/<host>/<owner>/notebooks, from a fresh full clone of
#            the old store's remote, keeping each notebook's history. Push
#            each, then clone it into ~/<host>/<owner>/notebooks.
#        migrate/stores.sh local [--apply]
#            On each machine: copies the notebooks of folders that are not
#            repositories (e.g. ~/scratch/<topic>) and exist here into
#            <folder>/notes. Their history stays in the old store.
#
# Without --apply, prints the plan and changes nothing. Ends with the
# notebooks it does not migrate and why, to review before archiving the old
# store: it never guesses with notes it cannot place.
#
# Environment: MIGRATE_HOSTS, the host folders under ~ that hold clones
# (default: GitHub Forgejo Codeberg GitLab).
# Needs git, and git-filter-repo for `stores`.
set -euo pipefail

readonly OLD_STORE="${FOLIO_HOME:-$HOME/memo}"
read -r -a HOSTS <<<"${MIGRATE_HOSTS:-GitHub Forgejo Codeberg GitLab}"
readonly HOSTS
# A notebook id's part: must match the Bash folio's ID_PART.
readonly NAME='[A-Za-z0-9][A-Za-z0-9._-]*'
# Field separator of the listings: not whitespace, so empty fields survive
# `read` (a tab would collapse them).
readonly SEP=$'\x1f'

LEFT_BEHIND=() # "<id>: <reason>" for each notebook not migrated

say() { printf '%s\n' "$*"; }
die() { printf 'migrate: %s\n' "$*" >&2; exit 1; }

# Records that notebook $1 is not migrated, for reason $2.
leave() { LEFT_BEHIND+=("$1: $2"); }

# Prints the notebooks left behind, if any.
report_left_behind() {
  ((${#LEFT_BEHIND[@]} > 0)) || return 0
  say "not migrated (review before archiving the old store):"
  printf '  %s\n' "${LEFT_BEHIND[@]}"
}

# Prints the folder a recorded path ($1, written ~/...) stands for.
# shellcheck disable=SC2088  # a literal ~: .source paths are written from ~
expand_path() { [[ $1 == "~/"* ]] && printf '%s/%s\n' "$HOME" "${1#\~/}"; }

# True when recorded path $1 is an owner's project:
# ~/<host>/<owner>/<repo>, <host> one of HOSTS.
is_project_path() {
  local host
  [[ $1 =~ ^~/($NAME)/$NAME/$NAME$ ]] || return 1
  for host in "${HOSTS[@]}"; do
    [[ ${BASH_REMATCH[1]} == "$host" ]] && return 0
  done
  return 1
}

# Prints "<id> <path> <origin>" (SEP-separated) for each notebook of store $1,
# the id being the notebook's folder in the store. Must match the Bash
# folio's list_notebooks: hidden and _ folders are skipped. A notebook that
# holds or sits in another one is left behind, with its neighbour: copying
# either would duplicate the inner one's notes.
list_notebooks() {
  local dirs=() dir other key value path origin nested
  mapfile -t dirs < <(find "$1" -mindepth 1 \( -type f -name .source -printf '%h\n' \) -o \
    \( \( -name '.*' -o -name '_*' \) -prune \) | sort)
  for dir in "${dirs[@]}"; do
    nested=""
    for other in "${dirs[@]}"; do
      if [[ $other == "$dir"/* || $dir == "$other"/* ]]; then
        nested="${other#"$1"/}"
      fi
    done
    if [[ -n $nested ]]; then
      leave "${dir#"$1"/}" "nested with the notebook $nested"
      continue
    fi
    path="" origin=""
    while IFS='=' read -r key value; do
      case $key in
        path) path=$value ;;
        origin) origin=$value ;;
      esac
    done <"$dir/.source"
    printf '%s%s%s%s%s\n' "${dir#"$1"/}" "$SEP" "$path" "$SEP" "$origin"
  done
}

# Fails unless the old store holds nothing its remote lacks: the migration
# reads the remote.
require_old_store_pushed() {
  git -C "$OLD_STORE" rev-parse --git-dir >/dev/null 2>&1 || die "$OLD_STORE is not a git repository"
  [[ -z $(git -C "$OLD_STORE" status --porcelain) ]] || die "$OLD_STORE has uncommitted changes: sync it first"
  git -C "$OLD_STORE" remote get-url origin >/dev/null 2>&1 || return 0 # no remote: it is the source
  git -C "$OLD_STORE" rev-parse -q --verify '@{upstream}' >/dev/null ||
    die "$OLD_STORE tracks no upstream branch: cannot tell whether everything is pushed"
  [[ -z $(git -C "$OLD_STORE" rev-list '@{upstream}..HEAD') ]] || die "$OLD_STORE has unpushed commits: sync it first"
}

# migrate stores: one store per owner folder, history kept.
migrate_stores() {
  local staging="$1" apply="$2" origin id path repo_origin store name
  [[ -n $staging ]] || die "usage: migrate/stores.sh stores <staging folder> [--apply]"
  staging=$(realpath -m -- "$staging")
  [[ ! -e $staging || -z $(ls -A "$staging") ]] || die "$staging is not empty"
  command -v git-filter-repo >/dev/null || die "git-filter-repo not installed"
  require_old_store_pushed
  origin=$(git -C "$OLD_STORE" remote get-url origin 2>/dev/null) || origin="$OLD_STORE"

  # A full clone: the old store may be sparse and miss other machines' notes.
  # A global, not local: the EXIT trap runs after this function returns.
  work=$(mktemp -d)
  trap 'rm -rf -- "$work"' EXIT
  git clone -q "$origin" "$work/full" || die "cannot clone $origin"

  # The plan: one "<store> <name> <id>" line per notebook to move.
  local plan="$work/plan" listing="$work/listing"
  : >"$plan"
  list_notebooks "$work/full" >"$listing" # not $(...): its leave calls must stay in this shell
  while IFS=$SEP read -r id path repo_origin; do
    [[ -n $repo_origin ]] || continue # not a repository: migrated by `local`
    if [[ -z $path ]]; then
      leave "$id" "a repository outside ~ (no path recorded)"
    elif ! is_project_path "$path"; then
      leave "$id" "$path is not ~/<host>/<owner>/<repo> (hosts: ${HOSTS[*]})"
    else
      path=${path#\~/}
      printf '%s%s%s%s%s\n' "$staging/$(dirname "$path")/notebooks" "$SEP" "$(basename "$path")" "$SEP" "$id" >>"$plan"
    fi
  done <"$listing"
  leave_clashes "$plan"

  local stores=()
  mapfile -t stores < <(cut -d "$SEP" -f1 "$plan" | sort -u)
  for store in "${stores[@]}"; do
    say "store $store"
    local args=()
    while IFS=$SEP read -r name id; do
      say "  $id -> $name"
      args+=(--path "$id/" --path-rename "$id/:$name/")
    done < <(awk -F "$SEP" -v OFS="$SEP" -v store="$store" '$1 == store { print $2, $3 }' "$plan")
    [[ $apply == 1 ]] && build_store "$work/full" "$store" "${args[@]}"
  done
  report_left_behind
  if [[ $apply != 1 ]]; then
    say "dry run: rerun with --apply"
    return
  fi
  say "next, for each store: create a private repository, push the store to it:"
  say "  git -C <store> remote add origin <url> && git -C <store> push -u origin HEAD"
  say "then, on every machine, clone it into place and run folio check:"
  say "  git clone --filter=blob:none --sparse <url> ~/<host>/<owner>/notebooks"
}

# Removes from plan $1 the notebooks that would land in the same store
# folder (e.g. a repository renamed upstream: old and new ids, one path), and
# leaves them behind: merging them would mix their notes.
leave_clashes() {
  local plan="$1" clash id
  while IFS= read -r clash; do
    while IFS= read -r id; do
      leave "$id" "shares ${clash#*"$SEP"} in ${clash%%"$SEP"*} with another notebook"
    done < <(awk -F "$SEP" -v key="$clash" '$1 FS $2 == key { print $3 }' "$plan")
  done < <(cut -d "$SEP" -f1,2 "$plan" | sort | uniq -d)
  awk -F "$SEP" 'NR == FNR { count[$1 FS $2]++; next } count[$1 FS $2] == 1' "$plan" "$plan" >"$plan.kept"
  mv "$plan.kept" "$plan"
}

# Builds store $2 from the full clone $1, keeping only the paths named in the
# filter-repo arguments that follow, then turns each notebook's .source into
# a .notebook marker and adds the store marker.
build_store() {
  local full="$1" store="$2" notebook
  shift 2
  mkdir -p "$(dirname "$store")"
  git clone -q "$full" "$store"
  git -C "$store" filter-repo --quiet --force "$@"
  for notebook in "$store"/*/; do
    rm -- "$notebook/.source"
    : >"$notebook/.notebook"
  done
  : >"$store/.folio"
  git -C "$store" add --all
  git -C "$store" commit -q -m "Split this owner's notebooks from the central store"
}

# migrate local: copy this machine's non-repository notebooks into their
# folders.
migrate_local() {
  local apply="$1" id path repo_origin folder target staging listing
  [[ -d $OLD_STORE ]] || die "$OLD_STORE does not exist"
  listing="$(mktemp)"
  list_notebooks "$OLD_STORE" >"$listing" # not $(...): its leave calls must stay in this shell
  while IFS=$SEP read -r id path repo_origin; do
    [[ -z $repo_origin ]] || continue # a repository: migrated by `stores`
    folder=$(expand_path "$path") || { leave "$id" "recorded path '$path' is not under ~"; continue; }
    target="$folder/notes"
    if [[ ! -d $folder ]]; then
      leave "$id" "$folder is not on this machine"
    elif git -C "$folder" rev-parse --git-dir >/dev/null 2>&1; then
      leave "$id" "$folder became a repository: move its notes to its owner's store by hand"
    elif [[ -e $target ]]; then
      leave "$id" "$target exists"
    else
      say "local $target <- $id"
      [[ $apply == 1 ]] || continue
      staging=$(mktemp -d "$folder/.notes-migrate.XXXXXX")
      chmod "$(printf '%o' $((0777 & ~0$(umask))))" "$staging" # mktemp makes it private
      cp -R "$OLD_STORE/$id/." "$staging/"
      rm -f -- "$staging/.source"
      : >"$staging/.notebook"
      mv "$staging" "$target"
    fi
  done <"$listing"
  rm -f -- "$listing"
  report_left_behind
  [[ $apply == 1 ]] || say "dry run: rerun with --apply"
}

main() {
  local mode="${1:-}" staging="" apply=0
  shift || true
  if [[ $mode == stores ]]; then
    staging="${1:-}"
    shift || true
  fi
  case "${1:-}" in
    "") ;;
    --apply) apply=1 ;;
    *) die "unknown option: $1" ;;
  esac
  case "$mode" in
    stores) migrate_stores "$staging" "$apply" ;;
    local) migrate_local "$apply" ;;
    *) die "usage: migrate/stores.sh stores <staging folder> [--apply] | local [--apply]" ;;
  esac
}

main "$@"
