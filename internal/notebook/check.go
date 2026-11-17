package notebook

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
		if !IsNotebook(notebook) {
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
