package notebook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dixie6502/folio/internal/gitcmd"
)

// Move moves folder oldPath to newPath together with its store notebook, and
// returns what it did. When the folder was already moved by hand (oldPath
// gone, newPath present), it moves only the notebook. A local notes/ travels
// inside its folder. Nothing moves unless both moves can be done.
func (e Env) Move(oldPath, newPath string) ([]string, error) {
	oldPath, err := absolutePath(oldPath)
	if err != nil {
		return nil, err
	}
	newPath, err = absolutePath(newPath)
	if err != nil {
		return nil, err
	}
	moveFolder := exists(oldPath)
	switch {
	case moveFolder && exists(newPath):
		return nil, fmt.Errorf("%s already exists", newPath)
	case !moveFolder && !exists(newPath):
		return nil, fmt.Errorf("neither %s nor %s exists", oldPath, newPath)
	}

	oldNotebook, ok := e.storeNotebook(oldPath)
	moveNotebook := ok && IsNotebook(oldNotebook)
	var newNotebook string
	if moveNotebook {
		if newNotebook, ok = e.storeNotebook(newPath); !ok {
			return nil, fmt.Errorf("no %s/ in %s for the notebook %s", StoreDir, filepath.Dir(newPath), oldNotebook)
		}
		if exists(newNotebook) {
			return nil, fmt.Errorf("%s already exists", newNotebook)
		}
	}
	if !moveFolder && !moveNotebook {
		return nil, fmt.Errorf("%s has no notebook to move", oldPath)
	}

	var done []string
	if moveFolder {
		if err := move(oldPath, newPath); err != nil {
			return done, err
		}
		done = append(done, fmt.Sprintf("moved %s to %s", oldPath, newPath))
	}
	if moveNotebook {
		if err := move(oldNotebook, newNotebook); err != nil {
			return done, err
		}
		done = append(done, fmt.Sprintf("moved %s to %s", oldNotebook, newNotebook))
	}
	return done, nil
}

// move moves src to dst: with git mv when src is tracked by the repository
// holding both, so git records a rename; else by renaming, or by copying then
// deleting when the rename crosses file systems.
func move(src, dst string) error {
	if !isDir(filepath.Dir(dst)) {
		return fmt.Errorf("%s does not exist", filepath.Dir(dst))
	}
	if isTracked(src) && sameRepo(src, dst) {
		_, err := gitcmd.Run(filepath.Dir(src), "mv", "--", filepath.Base(src), dst)
		return err
	}
	err := os.Rename(src, dst)
	if errors.Is(err, syscall.EXDEV) {
		if err := copyTree(src, dst); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}
	return err
}

// isTracked reports whether git tracks path or anything under it.
func isTracked(path string) bool {
	_, err := gitcmd.Run(filepath.Dir(path), "ls-files", "--error-unmatch", "--", filepath.Base(path))
	return err == nil
}

// sameRepo reports whether src and dst's parent lie in one repository's work
// tree.
func sameRepo(src, dst string) bool {
	srcTop, err := gitcmd.Run(filepath.Dir(src), "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	dstTop, err := gitcmd.Run(filepath.Dir(dst), "rev-parse", "--show-toplevel")
	return err == nil && srcTop == dstTop
}
