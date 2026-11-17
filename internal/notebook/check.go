package notebook

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Check reports the problems of every store's notebooks: orphan, nested and
// stray notebooks; also misplaced store markers. The storesync package checks
// how stores sync. It changes nothing.
func (e Env) Check() []string {
	stores, misplaced := e.Stores()
	var problems []string
	for _, marker := range misplaced {
		problems = append(problems, fmt.Sprintf("%s: store marker outside a %s/ folder", marker, StoreDir))
	}
	for _, store := range stores {
		problems = append(problems, checkNotebooks(store)...)
	}
	return problems
}

// checkNotebooks reports the problems of store's visible folders.
func checkNotebooks(store string) []string {
	if _, err := os.ReadDir(store); err != nil {
		return []string{fmt.Sprintf("%s: %v", store, err)}
	}
	var problems []string
	for _, dir := range visibleDirs(store) {
		if !IsNotebook(dir) {
			problems = append(problems, fmt.Sprintf("%s: stray folder, not a notebook", dir))
			continue
		}
		if folder := folderOf(dir); !exists(folder) {
			problems = append(problems, fmt.Sprintf("%s: orphan, %s is gone: folio mv %s <new folder>, or folio forget %s",
				dir, folder, folder, folder))
		}
		problems = append(problems, checkInside(dir)...)
	}
	return problems
}

// checkInside reports notebooks nested in notebook.
func checkInside(notebook string) []string {
	var problems []string
	_ = filepath.WalkDir(notebook, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", path, err))
		case entry.IsDir() && path != notebook && IsNotebook(path):
			problems = append(problems, fmt.Sprintf("%s: notebook inside the notebook %s", path, notebook))
		}
		return nil
	})
	return problems
}
