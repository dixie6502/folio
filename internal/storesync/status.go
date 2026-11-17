package storesync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// staleAfter is how old a last successful sync may be before check reports
// it: the daemon fetches every few minutes.
const staleAfter = time.Hour

// entry records a store's last sync.
type entry struct {
	LastSuccess time.Time `json:"last_success,omitzero"`
	LastError   string    `json:"last_error,omitempty"`
	ErrorAt     time.Time `json:"error_at,omitzero"`
}

// status maps each store to its last sync.
type status map[string]entry

// loadStatus reads the status file; a missing one is an empty status.
func loadStatus(path string) (status, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return status{}, nil
	}
	if err != nil {
		return nil, err
	}
	loaded := status{}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return loaded, nil
}

// record writes store's sync result to the status file, under a lock, as a
// whole new file so a reader never sees half of it.
func (s Syncer) record(store string, syncErr error) error {
	if err := os.MkdirAll(filepath.Dir(s.StatusFile), 0o700); err != nil {
		return err
	}
	unlock, err := lock(s.StatusFile+".lock", lockWait)
	if err != nil {
		return err
	}
	defer unlock()
	all, err := loadStatus(s.StatusFile)
	if err != nil {
		return err
	}
	last, now := all[store], s.Now().UTC()
	if syncErr == nil {
		last.LastSuccess = now
	} else {
		last.LastError, last.ErrorAt = syncErr.Error(), now
	}
	all[store] = last
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	temporary := s.StatusFile + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, s.StatusFile)
}

// Check reports, for each store, why it cannot sync, commits waiting to be
// pushed, conflict files left by a merge, and what the status file says:
// never synced, synced too long ago, or a last sync that failed.
func (s Syncer) Check(stores []string) ([]string, error) {
	all, err := loadStatus(s.StatusFile)
	if err != nil {
		return nil, err
	}
	var problems []string
	report := func(store, format string, args ...any) {
		problems = append(problems, store+": "+fmt.Sprintf(format, args...))
	}
	for _, store := range stores {
		problems = append(problems, conflictFiles(store)...)
		st := probe(store)
		if st.skip != "" {
			report(store, "%s, not synced", st.skip)
			continue
		}
		if !st.hasUpstream {
			report(store, "no upstream branch yet: folio sync pushes the store and sets it")
		} else if n, err := countCommits(store, st.unpushed()); err != nil {
			report(store, "%v", err)
		} else if n > 0 {
			report(store, "%d unpushed commit(s)", n)
		}
		last := all[store]
		switch {
		case last.LastSuccess.IsZero():
			report(store, "never synced: run folio sync, or enable the folio-sync service")
		case s.Now().Sub(last.LastSuccess) > staleAfter:
			report(store, "last synced %s, over an hour ago: is the folio-sync service running?",
				last.LastSuccess.Local().Format(time.DateTime))
		}
		if last.LastError != "" && last.ErrorAt.After(last.LastSuccess) {
			report(store, "last sync failed at %s: %s", last.ErrorAt.Local().Format(time.DateTime), last.LastError)
		}
	}
	return problems, nil
}

// conflictFiles reports the conflict files a merge left in store.
func conflictFiles(store string) []string {
	var problems []string
	_ = filepath.WalkDir(store, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case entry.IsDir() && entry.Name() == ".git":
			return filepath.SkipDir
		case !entry.IsDir() && strings.Contains(entry.Name(), ConflictTag):
			problems = append(problems, path+": sync conflict, merge it into the note and delete it")
		}
		return nil
	})
	return problems
}

// lock takes an exclusive lock on the file at path, waiting up to wait, and
// returns the function releasing it.
func lock(path string, wait time.Duration) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			_ = file.Close()
			return nil, fmt.Errorf("%s: held by another sync", path)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
