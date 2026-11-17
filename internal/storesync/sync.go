// Package storesync keeps folio's stores in step with their remotes: it
// commits local changes, merges remote ones without touching files being
// edited, keeps both sides of a conflict, scans for secrets and pushes.
package storesync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dixie6502/folio/internal/gitcmd"
	"github.com/dixie6502/folio/internal/notebook"
)

// lockWait is how long a sync waits for another one holding the same store
// (a variable so tests can shorten it).
var lockWait = 30 * time.Second

// upstream names the branch a store's branch tracks.
const upstream = "@{upstream}"

// Syncer syncs stores. Its zero value is not usable: build it with New.
type Syncer struct {
	Host       string           // names this machine in commits and conflict files
	Gitleaks   string           // path to gitleaks; "" blocks every push
	StatusFile string           // where each store's last sync is recorded
	Now        func() time.Time // the clock
}

// New returns a Syncer for this machine.
func New() (Syncer, error) {
	host, err := os.Hostname()
	if err != nil {
		return Syncer{}, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return Syncer{}, err
	}
	gitleaks, _ := exec.LookPath("gitleaks")
	return Syncer{
		Host:       host,
		Gitleaks:   gitleaks,
		StatusFile: filepath.Join(cache, "folio", "status.json"),
		Now:        time.Now,
	}, nil
}

// Outcome tells what one store's sync did.
type Outcome struct {
	Skipped   string   // why the store was not synced, if it was not
	Committed bool     // local changes were committed
	Merged    bool     // remote changes were merged in
	Conflicts []string // conflict files written beside notes
	Retry     bool     // a file changed during the merge: the next sync retries
	Pushed    bool     // commits were pushed
}

// Changed reports whether the sync changed anything, or must retry.
func (o Outcome) Changed() bool {
	return o.Committed || o.Merged || o.Retry || o.Pushed
}

// String summarises the outcome in one line.
func (o Outcome) String() string {
	if o.Skipped != "" {
		return "skipped: " + o.Skipped
	}
	if !o.Changed() {
		return "up to date"
	}
	var parts []string
	if o.Committed {
		parts = append(parts, "committed")
	}
	if o.Merged {
		parts = append(parts, "merged")
	}
	if len(o.Conflicts) > 0 {
		parts = append(parts, "conflicts kept in "+strings.Join(o.Conflicts, ", "))
	}
	if o.Retry {
		parts = append(parts, "a file changed during the merge, retrying later")
	}
	if o.Pushed {
		parts = append(parts, "pushed")
	}
	return strings.Join(parts, ", ")
}

// Store syncs one store and records the result in the status file.
func (s Syncer) Store(store string) (Outcome, error) {
	outcome, err := s.cycle(store)
	if outcome.Skipped == "" && !outcome.Retry {
		if recordErr := s.record(store, err); recordErr != nil && err == nil {
			err = recordErr
		}
	}
	return outcome, err
}

// state is what decides how a store syncs.
type state struct {
	gitDir      string // the store's git folder
	skip        string // why the store cannot sync, if it cannot
	hasUpstream bool   // the branch tracks a remote branch that exists
}

// probe works out store's state: the one place that decides whether a store
// can sync.
func probe(store string) state {
	gitDir, err := gitcmd.Run(store, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return state{skip: "not a git repository"}
	}
	if remotes, _ := gitcmd.Run(store, "remote"); remotes == "" {
		return state{gitDir: gitDir, skip: "no remote"}
	}
	_, err = gitcmd.Run(store, "rev-parse", "--verify", "-q", upstream)
	return state{gitDir: gitDir, hasUpstream: err == nil}
}

// unpushed names the commits not yet on the remote.
func (st state) unpushed() string {
	if st.hasUpstream {
		return upstream + "..HEAD"
	}
	return "HEAD" // never pushed: all of them
}

// cycle runs one sync of store, under its lock: grow the checkout, commit,
// fetch, merge (then grow again, for new remote notebooks), scan, push.
func (s Syncer) cycle(store string) (Outcome, error) {
	var outcome Outcome
	st := probe(store)
	if st.skip != "" {
		outcome.Skipped = st.skip
		return outcome, nil
	}
	unlock, err := lock(filepath.Join(st.gitDir, "folio-sync.lock"), lockWait)
	if err != nil {
		outcome.Skipped = "another sync holds it"
		return outcome, nil
	}
	defer unlock()
	for _, marker := range []string{"MERGE_HEAD", "rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(st.gitDir, marker)); err == nil {
			return outcome, fmt.Errorf("a merge or rebase is in progress in %s: finish it first", store)
		}
	}

	if err := growCheckout(store); err != nil {
		return outcome, err
	}
	if outcome.Committed, err = s.commit(store); err != nil {
		return outcome, err
	}
	if _, err := gitcmd.Run(store, "fetch", "-q"); err != nil {
		return outcome, err
	}
	if st.hasUpstream {
		merge, err := s.integrate(store, st.gitDir)
		outcome.Merged, outcome.Conflicts, outcome.Retry = merge.merged, merge.conflicts, merge.retry
		if err != nil || merge.retry {
			return outcome, err
		}
		if merge.merged {
			if err := growCheckout(store); err != nil {
				return outcome, err
			}
		}
	}

	if !st.hasUpstream && !hasCommit(store) {
		return outcome, nil // an empty store: nothing to push yet
	}
	if n, err := countCommits(store, st.unpushed()); err != nil || n == 0 {
		return outcome, err
	}
	if err := s.scan(store, st.unpushed()); err != nil {
		return outcome, err
	}
	push := []string{"push", "-q"}
	if !st.hasUpstream {
		push = append(push, "-u", "origin", "HEAD")
	}
	if _, err := gitcmd.Run(store, push...); err != nil {
		return outcome, err
	}
	outcome.Pushed = true
	return outcome, nil
}

// changes returns store's git status --porcelain, without taking git's
// optional index lock (a poll must not collide with the user's git).
func changes(store string) (string, error) {
	return gitcmd.Run(store, "--no-optional-locks", "status", "--porcelain")
}

// commit commits every change in store; it reports whether there was any.
func (s Syncer) commit(store string) (bool, error) {
	status, err := changes(store)
	if err != nil || status == "" {
		return false, err
	}
	if _, err := gitcmd.Run(store, "add", "--all"); err != nil {
		return false, err
	}
	_, err = gitcmd.Run(store, "commit", "-q", "-m", s.Host+" · "+s.Now().UTC().Format(time.RFC3339))
	return err == nil, err
}

// hasCommit reports whether store has any commit.
func hasCommit(store string) bool {
	_, err := gitcmd.Run(store, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// countCommits returns the number of commits in revisions.
func countCommits(store, revisions string) (int, error) {
	count, err := gitcmd.Run(store, "rev-list", "--count", revisions)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(count)
}

// scan runs gitleaks over the commits in revisions; a finding, or gitleaks
// missing, blocks the push.
func (s Syncer) scan(store, revisions string) error {
	if s.Gitleaks == "" {
		return errors.New("gitleaks not installed: not pushing")
	}
	cmd := exec.Command(s.Gitleaks, "git", "--log-opts="+revisions, "--redact", "--no-banner", "--log-level", "error", ".")
	cmd.Dir = store
	out, err := cmd.CombinedOutput()
	switch {
	case err == nil:
		return nil
	case gitcmd.ExitCode(err) == 1:
		return errors.New("gitleaks found a secret in unpushed commits: not pushing")
	default:
		return fmt.Errorf("gitleaks: %w: %s", err, strings.TrimSpace(string(out)))
	}
}

// growCheckout adds to a sparse store's checkout every notebook on disk (so a
// new one can be committed) and every notebook whose folder exists on this
// machine, plus the store's template. It never removes one.
func growCheckout(store string) error {
	if sparse, _ := gitcmd.Run(store, "config", "--bool", "core.sparseCheckout"); sparse != "true" {
		return nil
	}
	listed, err := gitcmd.Run(store, "sparse-checkout", "list")
	if err != nil {
		return err
	}
	inCheckout := map[string]bool{}
	for _, dir := range strings.Split(listed, "\n") {
		inCheckout[dir] = true
	}
	var add []string
	want := func(name string) {
		if !inCheckout[name] {
			inCheckout[name] = true
			add = append(add, name)
		}
	}
	entries, _ := os.ReadDir(store)
	for _, entry := range entries {
		if entry.IsDir() && notebook.IsNotebook(filepath.Join(store, entry.Name())) {
			want(entry.Name())
		}
	}
	tracked, _ := gitcmd.Run(store, "ls-tree", "-d", "--name-only", "-z", "HEAD")
	for _, name := range strings.Split(tracked, "\x00") {
		if name == notebook.TemplateDir || (name != "" && notebook.HasFolder(filepath.Join(store, name))) {
			want(name)
		}
	}
	if len(add) == 0 {
		return nil
	}
	_, err = gitcmd.Run(store, append([]string{"sparse-checkout", "add"}, add...)...)
	return err
}
