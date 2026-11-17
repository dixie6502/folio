package notebook

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Confirm asks the user a yes/no question; nil means no one can answer.
type Confirm func(question string) bool

// Init creates the notebook of folder dir and returns it; an existing one is
// returned as is. With a store in the reference folder's owner folder, the
// notebook goes there; otherwise, or when local is set, it is a local notes/
// in the reference folder. Without local, a local notebook needs confirm's yes.
func (e Env) Init(dir string, local bool, confirm Confirm) (string, error) {
	here := e.locate(dir)
	switch {
	case here.store != "":
		return "", fmt.Errorf("inside the store %s, not a project folder", here.store)
	case here.ref == e.Home:
		return "", errors.New("~ itself has no notebook")
	}
	if !local {
		if here.notebook != "" {
			return create(here.notebook, e.template(filepath.Dir(here.notebook)))
		}
		missing := fmt.Sprintf("no %s/ in %s", StoreDir, filepath.Dir(here.ref))
		if confirm == nil {
			return "", fmt.Errorf("%s: folio init --local creates a local %s/ instead", missing, LocalDir)
		}
		question := fmt.Sprintf("%s: create local %s/ here instead?", missing, LocalDir)
		if here.inRepo {
			question += fmt.Sprintf(" (%s/ will sit inside the repository)", LocalDir)
		}
		if !confirm(question) {
			return "", errors.New("no notebook created")
		}
	}
	return create(filepath.Join(here.ref, LocalDir), e.template(""))
}

// template returns the template for a new notebook in store (empty for a
// local notebook): the store's own, else the global one, else "" for none.
func (e Env) template(store string) string {
	if store != "" && isDir(filepath.Join(store, TemplateDir)) {
		return filepath.Join(store, TemplateDir)
	}
	if isDir(e.Template) {
		return e.Template
	}
	return ""
}

// create makes notebook from template and returns it. It is built in a hidden
// staging folder, then moved into place, so a failure never leaves half a
// notebook behind.
func create(notebook, template string) (string, error) {
	if isNotebook(notebook) {
		return notebook, nil
	}
	if exists(notebook) {
		return "", fmt.Errorf("%s exists and is not a notebook", notebook)
	}
	staging, err := os.MkdirTemp(filepath.Dir(notebook), ".folio-init-")
	if err != nil {
		return "", err
	}
	if err := fill(staging, template); err != nil {
		_ = os.RemoveAll(staging)
		return "", fmt.Errorf("cannot create %s: %w", notebook, err)
	}
	if err := os.Rename(staging, notebook); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return notebook, nil
}

// fill copies template (if any) into the empty folder dir and adds the
// notebook marker.
func fill(dir, template string) error {
	if template != "" {
		if err := copyTree(template, dir); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, NotebookMarker), nil, 0o644)
}

// copyTree copies folder src into dst, which may exist, never overwriting a
// file and keeping permissions. Symlinks are copied as symlinks.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return ignoreExisting(os.Symlink(link, target))
		default:
			return ignoreExisting(copyFile(path, target, info.Mode().Perm()))
		}
	})
}

// copyFile streams file src to dst, created with perm; fails with
// fs.ErrExist when dst exists.
func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// ignoreExisting turns an error for an already existing target into success.
func ignoreExisting(err error) error {
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	return err
}
