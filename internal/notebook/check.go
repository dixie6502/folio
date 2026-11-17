package notebook

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// conflictTag marks the files sync leaves beside a conflicting note.
const conflictTag = ".conflict-"

// Check reports the problems of every store: orphan, nested and stray
// notebooks, conflict files, and stores that cannot sync; also misplaced
// store markers. It changes nothing.
func (e Env) Check() []string {
	stores, misplaced := e.Stores()
	var problems []string
	for _, marker := range misplaced {
		problems = append(problems, fmt.Sprintf("%s: store marker outside a %s/ folder", marker, StoreDir))
	}
	for _, store := range stores {
		problems = append(problems, checkNotebooks(store)...)
		problems = append(problems, checkSync(store)...)
	}
	return problems
}

// checkNotebooks reports the problems of store's top-level folders.
func checkNotebooks(store string) []string {
	entries, err := os.ReadDir(store)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", store, err)}
	}
	var problems []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		notebook := filepath.Join(store, entry.Name())
		if !isNotebook(notebook) {
			problems = append(problems, fmt.Sprintf("%s: stray folder, not a notebook", notebook))
			continue
		}
		if folder := folderOf(notebook); !exists(folder) {
			problems = append(problems, fmt.Sprintf("%s: orphan, %s is gone: folio mv %s <new folder>, or folio forget %s",
				notebook, folder, folder, folder))
		}
		problems = append(problems, checkInside(notebook)...)
	}
	return problems
}

// checkInside reports notebooks nested in notebook and conflict files in it.
func checkInside(notebook string) []string {
	var problems []string
	_ = filepath.WalkDir(notebook, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", path, err))
		case entry.IsDir() && path != notebook && isNotebook(path):
			problems = append(problems, fmt.Sprintf("%s: notebook inside the notebook %s", path, notebook))
		case !entry.IsDir() && strings.Contains(entry.Name(), conflictTag):
			problems = append(problems, fmt.Sprintf("%s: sync conflict, merge it into the note and delete it", path))
		}
		return nil
	})
	return problems
}

// checkSync reports commits waiting to be pushed from store, or why it cannot
// sync. One git call on the normal path; more only to explain a failure.
func checkSync(store string) []string {
	count, err := git(store, "rev-list", "--count", "@{upstream}..HEAD")
	if err == nil {
		if n, _ := strconv.Atoi(count); n > 0 {
			return []string{fmt.Sprintf("%s: %d unpushed commit(s)", store, n)}
		}
		return nil
	}
	if _, err := git(store, "rev-parse", "--git-dir"); err != nil {
		return []string{fmt.Sprintf("%s: not a git repository, not synced", store)}
	}
	if remotes, _ := git(store, "remote"); remotes == "" {
		return []string{fmt.Sprintf("%s: no remote, not synced", store)}
	}
	return []string{fmt.Sprintf("%s: no upstream branch, not synced", store)}
}
