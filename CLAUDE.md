# folio — project rules

A notational-velocity switcher over one notebook per folder. Spec of the
current migration: `migration.md`; keep `README.md` in sync with the code.

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
