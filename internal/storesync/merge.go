package storesync

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dixie6502/folio/internal/gitcmd"
)

// Conflict files: <name><ConflictTag><host>-<UTC time><extension>.
const (
	ConflictTag   = ".conflict-"
	conflictStamp = "20060102T150405Z"
)

// merge tells what integrating the upstream did.
type merge struct {
	merged    bool     // the store moved to include the upstream's commits
	conflicts []string // conflict files written
	retry     bool     // a file changed meanwhile: nothing moved, retry later
}

// integrate brings the upstream's commits into store. The merge is computed
// with merge-tree, so no file is touched until the result is ready; then
// reset --keep moves to it, which refuses when a file it would change was
// edited meanwhile (retry: the next sync tries again).
func (s Syncer) integrate(store, gitDir string) (merge, error) {
	var result merge
	ahead, behind, err := divergence(store)
	if err != nil || behind == 0 {
		return result, err
	}
	target := upstream
	if ahead > 0 {
		if target, result.conflicts, err = s.mergeCommit(store, gitDir); err != nil {
			return result, err
		}
	}
	if _, err := gitcmd.Run(store, "reset", "-q", "--keep", target); err != nil {
		// A file edited since the commit step makes reset refuse: retry.
		// With nothing edited, the failure is real.
		if edited, _ := changes(store); edited != "" {
			return merge{retry: true}, nil
		}
		return result, err
	}
	result.merged = true
	return result, nil
}

// divergence returns how many commits HEAD has that the upstream lacks, and
// the reverse.
func divergence(store string) (ahead, behind int, err error) {
	out, err := gitcmd.Run(store, "rev-list", "--left-right", "--count", "HEAD..."+upstream)
	if err != nil {
		return 0, 0, err
	}
	left, right, _ := strings.Cut(out, "\t")
	if ahead, err = strconv.Atoi(left); err != nil {
		return 0, 0, err
	}
	behind, err = strconv.Atoi(right)
	return ahead, behind, err
}

// mergeCommit creates the merge commit of HEAD and the upstream and returns
// it with the conflict files it wrote. A conflicting note keeps the
// upstream's version, the local one is kept beside it as a conflict file; a
// note deleted on one side and changed on the other keeps the change; a file
// against a folder keeps merge-tree's answer (the file renamed <name>~<side>).
func (s Syncer) mergeCommit(store, gitDir string) (string, []string, error) {
	out, err := gitcmd.Run(store, "merge-tree", "--write-tree", "--name-only", "-z", "--no-messages", "HEAD", upstream)
	if err != nil && gitcmd.ExitCode(err) != 1 { // 1 means conflicts
		return "", nil, err
	}
	fields := strings.Split(strings.Trim(out, "\x00"), "\x00")
	tree, conflicted := fields[0], unique(fields[1:])

	var conflictFiles []string
	if len(conflicted) > 0 {
		index := filepath.Join(gitDir, "folio-merge-index")
		defer func() { _ = os.Remove(index) }()
		env := []string{"GIT_INDEX_FILE=" + index}
		if _, err := gitcmd.RunEnv(store, env, "read-tree", tree); err != nil {
			return "", nil, err
		}
		for _, path := range conflicted {
			conflictFile, err := s.resolve(store, env, path)
			if err != nil {
				return "", nil, err
			}
			if conflictFile != "" {
				conflictFiles = append(conflictFiles, conflictFile)
			}
		}
		if tree, err = gitcmd.RunEnv(store, env, "write-tree"); err != nil {
			return "", nil, err
		}
	}
	message := s.Host + " · merge · " + s.Now().UTC().Format(time.RFC3339)
	commit, err := gitcmd.Run(store, "commit-tree", tree, "-p", "HEAD", "-p", upstream, "-m", message)
	return commit, conflictFiles, err
}

// resolve settles conflicted path in the index env points at, and returns
// the conflict file it wrote, if any.
func (s Syncer) resolve(store string, env []string, path string) (string, error) {
	local, err := treeEntry(store, "HEAD", path)
	if err != nil {
		return "", err
	}
	remote, err := treeEntry(store, upstream, path)
	if err != nil {
		return "", err
	}
	if isTree(local) || isTree(remote) {
		return "", nil // a file against a folder: merge-tree keeps both, the file renamed
	}
	switch {
	case local != "" && remote != "":
		conflict := s.conflictName(path)
		if err := setEntry(store, env, remote, path); err != nil {
			return "", err
		}
		return conflict, setEntry(store, env, local, conflict)
	case local != "":
		return "", setEntry(store, env, local, path)
	case remote != "":
		return "", setEntry(store, env, remote, path)
	default: // a path merge-tree made, e.g. a file renamed out of a folder's way
		return "", nil
	}
}

// conflictName names the file keeping this machine's side of path.
func (s Syncer) conflictName(path string) string {
	extension := filepath.Ext(path)
	return fmt.Sprintf("%s%s%s-%s%s", strings.TrimSuffix(path, extension), ConflictTag, s.Host,
		s.Now().UTC().Format(conflictStamp), extension)
}

// treeEntry returns path's "mode,oid" in revision, or "" when it has none.
func treeEntry(store, revision, path string) (string, error) {
	out, err := gitcmd.Run(store, "ls-tree", "-z", revision, "--", path)
	if err != nil || out == "" {
		return "", err
	}
	mode, rest, _ := strings.Cut(out, " ")
	_, rest, _ = strings.Cut(rest, " ") // the object type
	oid, _, _ := strings.Cut(rest, "\t")
	return mode + "," + oid, nil
}

// isTree reports whether a tree entry ("mode,oid") is a folder.
func isTree(entry string) bool { return strings.HasPrefix(entry, "040000,") }

// setEntry puts entry ("mode,oid") at path in the index env points at.
func setEntry(store string, env []string, entry, path string) error {
	_, err := gitcmd.RunEnv(store, env, "update-index", "--add", "--cacheinfo", entry+","+path)
	return err
}

// unique returns items without repeats or empty ones, in order.
func unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
