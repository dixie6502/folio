# folio — project rules

A notational-velocity switcher over one notebook per folder. `README.md` is
the spec: keep it in sync with the code. Open items: the `todo` skill
(`TODO.md` in folio's own notebook).

## Scope

- `folio` stays a switcher (filter, open, create) plus a few non-interactive
  notebook commands. No link commands: links belong to wikijump.vim.
- No global notebook: a notebook exists only once initialized from a folder.
- `folio` knows nothing of `TODO.md`'s format, frontmatter, editor
  conventions (the template carries those) or git beyond reading `origin`.

## Public repo

- Never name a client, private host, email or person from the user's work.
  Use placeholders (`<owner>`, `<hostname>`). Public projects and their URLs
  (fzf, ripgrep) are fine.
- `pre-commit` (gitleaks, shellcheck, the bats suite) passes before every commit.

## Git

Own repo: commit and push directly to `main` once all checks pass.

## Code

- One Bash script, `folio`; runtime dependencies `fzf`, `rg`, `git`, optional `gawk`.
- Tests: `bats test/` (bats-core, pinned in construct's bootstrap). They run
  against a temporary `$HOME` and store, with local git repositories, never the
  real store or the network.
