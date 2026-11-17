package notebook

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Forget deletes the notebook of folder, which must no longer exist, after
// confirm's yes; a nil confirm (no one to ask) keeps it.
func (e Env) Forget(folder string, confirm Confirm) (string, error) {
	folder, err := absolutePath(folder)
	if err != nil {
		return "", err
	}
	if exists(folder) {
		return "", fmt.Errorf("%s exists: only an orphan's notebook is forgotten", folder)
	}
	notebook, ok := e.storeNotebook(folder)
	if !ok || !IsNotebook(notebook) {
		return "", fmt.Errorf("%s has no notebook", folder)
	}
	question := fmt.Sprintf("delete %s and its %d note(s)?", notebook, countNotes(notebook))
	if confirm == nil {
		return "", fmt.Errorf("kept %s: folio forget asks for confirmation, so it needs a terminal", notebook)
	}
	if !confirm(question) {
		return "", fmt.Errorf("kept %s", notebook)
	}
	if err := os.RemoveAll(notebook); err != nil {
		return "", err
	}
	return "forgot " + notebook, nil
}

// countNotes returns the number of *.md files under dir.
func countNotes(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			count++
		}
		return nil
	})
	return count
}
