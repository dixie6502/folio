package storesync

import (
	"context"
	"time"
)

// Daemon timing: how often stores are polled, how long a store must stay
// unchanged before its changes are committed, and how often remotes are
// fetched (and the list of stores refreshed).
const (
	pollEvery   = 10 * time.Second
	quietPeriod = 60 * time.Second
	fetchEvery  = 3 * time.Minute
)

// watch is what the daemon remembers of a store between polls.
type watch struct {
	status    string    // git status --porcelain at the last poll
	changedAt time.Time // when status last changed
}

// Daemon syncs the stores listed by stores until ctx is done: a store whose
// changes have rested for the quiet period, and every store when a fetch is
// due or the machine has resumed from suspend. It reports each sync that did
// something through logf.
func (s Syncer) Daemon(ctx context.Context, stores func() []string, logf func(format string, args ...any)) {
	watches := map[string]watch{}
	var list []string
	var lastFetch time.Time // zero: the first poll syncs every store
	lastPoll := s.Now()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		now := s.Now()
		fetchDue := now.Sub(lastFetch) >= fetchEvery || hasResumed(lastPoll, now)
		if fetchDue {
			list, lastFetch = stores(), now
		}
		for _, store := range list {
			status, _ := changes(store)
			if previous := watches[store]; previous.status != status {
				watches[store] = watch{status: status, changedAt: now}
			}
			if !fetchDue && !isQuiet(watches[store], now) {
				continue
			}
			outcome, err := s.Store(store)
			if err != nil {
				logf("%s: %v", store, err)
			} else if outcome.Changed() {
				logf("%s: %s", store, outcome)
			}
			delete(watches, store) // the sync changed the status: watch afresh
		}
		lastPoll = s.Now() // after the syncs, so a slow one does not look like a resume
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// isQuiet reports whether a store has changes that have rested for the quiet
// period.
func isQuiet(w watch, now time.Time) bool {
	return w.status != "" && now.Sub(w.changedAt) >= quietPeriod
}

// hasResumed reports whether the wall clock jumped between two polls, as it
// does across a suspend (the monotonic clock stops then, so it is ignored).
func hasResumed(lastPoll, now time.Time) bool {
	return now.Round(0).Sub(lastPoll.Round(0)) > 3*pollEvery
}
