// Package notebook finds, creates, moves and checks folio notebooks.
//
// A store is a folder named notebooks/ holding a .folio marker, in an owner
// folder ~/<host>/<owner>/; it holds one notebook per project of that owner,
// named after the project's folder. A local notebook is a notes/ folder inside
// the folder it serves. Either kind of notebook holds a .notebook marker.
package notebook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dixie6502/folio/internal/gitcmd"
)

// Names of the folders and markers that make up the layout.
const (
	StoreDir       = "notebooks"
	StoreMarker    = ".folio"
	NotebookMarker = ".notebook"
	LocalDir       = "notes"
	TemplateDir    = ".template"
)

// storeDepth is how many folders below ~ a store sits: ~/<host>/<owner>/notebooks.
const storeDepth = 3

// ErrNoNotebook means the folder has no notebook.
var ErrNoNotebook = errors.New("no notebook here: folio init")

// Env holds the paths folio works from, symlinks resolved.
type Env struct {
	Home     string // the user's home folder
	Template string // the global template folder (may not exist)
}

// NewEnv reads the environment: $HOME and $XDG_CONFIG_HOME.
func NewEnv() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, err
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	return Env{Home: physical(home), Template: filepath.Join(config, "folio", "template")}, nil
}

// location is where a folder stands relative to the notebook layout.
type location struct {
	dir      string // the folder, symlinks resolved
	store    string // the store dir lies in, if any; the fields below are then unset
	ref      string // the folder a notebook belongs to (a repository's main checkout)
	inRepo   bool   // whether ref is a repository
	notebook string // ref's notebook in its owner's store; "" without a store
}

// locate works out where folder dir stands.
func (e Env) locate(dir string) location {
	here := location{dir: physical(dir)}
	if store, ok := e.enclosingStore(here.dir); ok {
		here.store = store
		return here
	}
	here.ref, here.inRepo = referenceFolder(here.dir)
	here.notebook, _ = e.storeNotebook(here.ref)
	return here
}

// Find returns the notebook of folder dir: inside a store, the notebook dir
// lies in; else the reference folder's notebook in its owner's store; else
// the nearest local notes/ from dir up to, not including, the home folder.
func (e Env) Find(dir string) (string, error) {
	here := e.locate(dir)
	if here.store != "" {
		for d := here.dir; d != here.store; d = filepath.Dir(d) {
			if IsNotebook(d) {
				return d, nil
			}
		}
		return "", fmt.Errorf("inside the store %s, but not in a notebook", here.store)
	}
	if here.notebook != "" && IsNotebook(here.notebook) {
		return here.notebook, nil
	}
	for d := here.dir; d != e.Home && d != filepath.Dir(d); d = filepath.Dir(d) {
		if local := filepath.Join(d, LocalDir); IsNotebook(local) {
			return local, nil
		}
	}
	return "", ErrNoNotebook
}

// storeNotebook returns where folder ref's notebook sits in its owner's
// store; ok is false when there is no store there.
func (e Env) storeNotebook(ref string) (notebook string, ok bool) {
	store := filepath.Join(filepath.Dir(ref), StoreDir)
	if !e.isStore(store) {
		return "", false
	}
	return filepath.Join(store, filepath.Base(ref)), true
}

// folderOf returns the folder a store notebook belongs to, the inverse of
// storeNotebook.
func folderOf(notebook string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(notebook)), filepath.Base(notebook))
}

// HasFolder reports whether the folder a store notebook belongs to exists on
// this machine (it is no orphan).
func HasFolder(notebook string) bool { return exists(folderOf(notebook)) }

// isStore reports whether dir is a store: ~/<host>/<owner>/notebooks holding
// the store marker.
func (e Env) isStore(dir string) bool {
	rel, err := filepath.Rel(e.Home, dir)
	return err == nil && filepath.Base(dir) == StoreDir && !strings.HasPrefix(rel, "..") &&
		len(strings.Split(rel, string(filepath.Separator))) == storeDepth &&
		isFile(filepath.Join(dir, StoreMarker))
}

// enclosingStore returns the store dir lies in, if any.
func (e Env) enclosingStore(dir string) (string, bool) {
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if e.isStore(d) {
			return d, true
		}
	}
	return "", false
}

// Stores lists every store, and the store markers misplaced at a store's
// depth in a folder not named notebooks/. Hidden folders are skipped.
func (e Env) Stores() (stores, misplaced []string) {
	for _, host := range visibleDirs(e.Home) {
		for _, owner := range visibleDirs(host) {
			for _, dir := range visibleDirs(owner) {
				switch marker := filepath.Join(dir, StoreMarker); {
				case !isFile(marker):
				case filepath.Base(dir) == StoreDir:
					stores = append(stores, dir)
				default:
					misplaced = append(misplaced, marker)
				}
			}
		}
	}
	return stores, misplaced
}

// visibleDirs lists the folders in dir whose names do not start with a dot.
func visibleDirs(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, filepath.Join(dir, entry.Name()))
		}
	}
	return dirs
}

// referenceFolder returns the folder a notebook belongs to for dir: the main
// checkout of the repository dir lies in (so a worktree shares its notebook),
// else dir itself; inRepo tells which.
func referenceFolder(dir string) (ref string, inRepo bool) {
	out, err := gitcmd.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-toplevel")
	if err != nil {
		return dir, false
	}
	common, top, found := strings.Cut(out, "\n")
	if !found {
		return dir, false
	}
	if filepath.Base(common) == ".git" {
		return physical(filepath.Dir(common)), true
	}
	return physical(top), true // a submodule: its git dir lies in the parent's
}

// IsNotebook reports whether dir holds the notebook marker.
func IsNotebook(dir string) bool { return isFile(filepath.Join(dir, NotebookMarker)) }

// isFile reports whether path is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isDir reports whether path is a folder.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// exists reports whether anything sits at path (a broken symlink counts).
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// absolutePath returns path absolute, symlinks resolved.
func absolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return physical(abs), nil
}

// physical resolves symlinks in path; a path that does not exist keeps its
// missing tail under its resolved existing parent.
func physical(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(physical(parent), filepath.Base(path))
}
