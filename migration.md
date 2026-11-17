# Migration: from the central store to the owners' stores

The Bash folio kept every notebook in one store, `~/memo` (`$FOLIO_HOME`),
synced by mneme's `memo` hooks. The Go folio (see `README.md`) keeps them in a
`notebooks/` store per owner, or a local `notes/`, and syncs them itself. This
file, `migrate/` and `test/migrate.bats` go once every machine is migrated.

`migrate/stores.sh` is a dry run unless given `--apply`, never changes the old
store, and ends with every notebook it leaves behind and why (to review before
archiving the old store). `migrate/legacy-folio` is the Bash folio, kept only
for the script's tests.

## Steps

1. **On every machine, stop the old sync.** Let `~/memo` sync one last time (a
   Claude session start or end, or commit and push it by hand), then update
   mneme's plugins (`claude plugin marketplace update mneme`), whose `memo` no
   longer syncs. From here on, no note may be written to `~/memo`.
2. **On every machine, install the Go folio** with construct's bootstrap: the
   binary, the `folio-sync` service and the global template. Until step 4,
   `/todo` and `/note` find no notebook.
3. **Once, on any machine, build the stores:**
   `migrate/stores.sh stores <staging folder>`, then again with `--apply`.
   From a full clone of the old store's remote it builds
   `<staging>/<host>/<owner>/notebooks` per owner, each notebook's history kept
   (`git filter-repo`), `.source` replaced by `.notebook`. For each store,
   create the owner's private repository (`<owner>/notebooks`), then
   `git -C <store> remote add origin <url> && git -C <store> push -u origin HEAD`.
4. **On every machine, put the notes in place:**
   - clone each store this machine needs:
     `git clone --filter=blob:none --sparse <url> ~/<host>/<owner>/notebooks`;
   - `migrate/stores.sh local`, then with `--apply`: copies the notebooks of
     this machine's folders that are not repositories into `<folder>/notes`;
   - `folio sync`, then `folio check`.
5. **Review** the notebooks the script lists as not migrated, then archive the
   old store's repository and delete `~/memo`.
6. **Delete** this file, `migrate/` and `test/migrate.bats`.
