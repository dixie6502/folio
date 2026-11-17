# folio — project rules

A notational-velocity switcher over one notebook per folder. `README.md` is
the spec: keep it in sync with the code. Open items: the `todo` skill
(`TODO.md` in folio's own notebook).

## Scope

- `folio` stays a switcher (filter, open, create) plus a few non-interactive
  notebook commands. No link commands: links belong to wikijump.vim.
- No global notebook: a notebook exists only once initialized from a folder.
- `folio` knows nothing of `TODO.md`'s format, frontmatter or editor
  conventions (the template carries those). Git beyond finding a repository's
  top level (and `git mv` for `folio mv`) belongs to `internal/storesync`.

## Public repo

- Never name a client, private host, email or person from the user's work.
  Use placeholders (`<owner>`, `<hostname>`). Public projects and their URLs
  (fzf, ripgrep) are fine.
- `pre-commit` (gitleaks, shellcheck, gofmt, go vet, go test, and the bats
  suite of the migration script) passes before every commit.

## Git

Own repo: commit and push directly to `main` once all checks pass.

## Code

- One Go binary, `cmd/folio` and `internal/`; runtime dependencies `git`, and
  `gitleaks` to push; Go modules `golang.org/x/term` and
  `github.com/mattn/go-runewidth`, pinned.
- Build with `go build -o bin/folio ./cmd/folio` (`bin/` is git-ignored).
- Tests: `go test ./...`, against a temporary `$HOME` (`internal/testutil`),
  with local git repositories, never the real stores or the network.
- Until the migration is done (`migration.md`): `migrate/stores.sh` with its
  bats tests (`bats test/`, bats-core pinned in construct's bootstrap), which
  use the old Bash folio kept as `migrate/legacy-folio`. All go together.
