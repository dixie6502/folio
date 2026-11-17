# folio — project rules

A notational-velocity switcher over one notebook per folder. `README.md` is
the spec: keep it in sync with the code. Open items: the `todo` skill
(`TODO.md` in folio's own notebook).

## Scope

- `folio` stays a switcher (filter, open, create) plus a few non-interactive
  notebook commands. No link commands: links belong to wikijump.vim.
- No global notebook: a notebook exists only once initialized from a folder.
- `folio` knows nothing of `TODO.md`'s format, frontmatter or editor
  conventions (the template carries those). The Bash script reads git's
  `origin` only; in the Go rewrite, git beyond finding a repository's top level
  (and `git mv` for `folio mv`) belongs to `internal/storesync`.

## Public repo

- Never name a client, private host, email or person from the user's work.
  Use placeholders (`<owner>`, `<hostname>`). Public projects and their URLs
  (fzf, ripgrep) are fine.
- `pre-commit` (gitleaks, shellcheck, the bats suite, gofmt, go vet, go test)
  passes before every commit.

## Git

Own repo: commit and push directly to `main` once all checks pass.

## Code

- In migration (`migration.md`): the Bash script `folio` (runtime dependencies
  `fzf`, `rg`, `git`, optional `gawk`) stays the installed tool until the Go
  rewrite (`cmd/folio`, `internal/`; runtime dependencies `git`, `gitleaks` to
  push; Go modules `golang.org/x/term`, `github.com/mattn/go-runewidth`)
  replaces it. `README.md` describes the Bash script until then.
- Build the Go binary with `go build -o bin/folio ./cmd/folio`: a plain
  `go build` would overwrite the Bash script at the repository root.
- Tests: `bats test/` (bats-core, pinned in construct's bootstrap) for the Bash
  script, `go test ./...` for Go. Both run against a temporary `$HOME`, with
  local git repositories, never the real store or the network.
