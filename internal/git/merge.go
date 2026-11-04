/*
MIT License

Copyright (c) 2026 Jan Van Herck (https://github.com/vhjan)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

/*
This file implements merging and rebasing, including a read-only pre-flight check that predicts conflicts without
touching the working tree. Whenever a merge or rebase stops because of conflicts it is aborted again, so a failed
operation never leaves the repository in a half-merged state.
*/

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MergeMode selects how a merge is performed.
type MergeMode int

const (
	MergeDefault MergeMode = iota // Fast-forward when possible, otherwise create a merge commit
	MergeFFOnly                   // Only fast-forward; fail if a merge commit would be needed
	MergeNoFF                     // Always create a merge commit
	MergeSquash                   // Combine all changes into a single new commit (no merge commit)
)

// flags returns the extra `git merge` arguments for the mode.
func (m MergeMode) flags() []string {
	switch m {
	case MergeFFOnly:
		return []string{"--ff-only"}
	case MergeNoFF:
		return []string{"--no-ff"}
	case MergeSquash:
		return []string{"--squash"}
	default:
		return nil
	}
}

// Label is a short human-readable description of the mode.
func (m MergeMode) Label() string {
	switch m {
	case MergeFFOnly:
		return "Fast-forward only"
	case MergeNoFF:
		return "Always create a merge commit (--no-ff)"
	case MergeSquash:
		return "Squash into a single commit (--squash)"
	default:
		return "Merge (fast-forward when possible)"
	}
}

// MergeCommand returns the command line that Merge runs, for display (dry run, logs).
func MergeCommand(ref string, mode MergeMode) string {
	parts := append([]string{"git", "merge", "--no-edit"}, mode.flags()...)
	return strings.Join(append(parts, ref), " ")
}

// RebaseCommand returns the command line that Rebase runs.
func RebaseCommand(onto string) string {
	return "git rebase " + onto
}

// ConflictError is returned when a merge or rebase hit conflicts. The operation has already been aborted.
type ConflictError struct {
	Op    string   // "merge" or "rebase"
	Files []string // Files that conflicted
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s aborted because of conflicts in: %s", e.Op, strings.Join(e.Files, ", "))
}

// MergeCheck is the outcome of a read-only pre-flight for merging theirs into ours.
type MergeCheck struct {
	Incoming    int      // Commits in theirs that ours does not have
	Local       int      // Commits in ours that theirs does not have
	UpToDate    bool     // theirs adds nothing: ours already contains it
	FastForward bool     // ours is a direct ancestor of theirs, so no merge commit is needed
	Checked     bool     // Whether conflicts could be predicted (needs git 2.38+ for `merge-tree --write-tree`)
	Conflicts   []string // Files that would conflict, when Checked
}

// CheckMerge inspects what merging theirs into ours would do, without modifying anything.
func (r *Repo) CheckMerge(ours, theirs string) (MergeCheck, error) {
	var c MergeCheck

	out, err := r.run("rev-list", "--left-right", "--count", ours+"..."+theirs)
	if err != nil {
		return c, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return c, fmt.Errorf("unexpected rev-list output %q", strings.TrimSpace(out))
	}
	c.Local, _ = strconv.Atoi(fields[0])
	c.Incoming, _ = strconv.Atoi(fields[1])
	c.UpToDate = c.Incoming == 0
	c.FastForward = c.Local == 0 && c.Incoming > 0

	// Without diverging history there is nothing that could conflict.
	if c.UpToDate || c.FastForward {
		c.Checked = true
		return c, nil
	}

	treeOut, _, err := r.runIO("merge-tree", "--write-tree", "--name-only", "--no-messages", ours, theirs)
	if err == nil {
		// Exit status 0: the merge would be clean.
		c.Checked = true
		return c, nil
	}
	switch exitCode(err) {
	case 1:
		// Conflicts: line 1 is the tree id, the following lines are the conflicted paths.
		c.Checked = true
		lines := splitLines(treeOut)
		if len(lines) > 1 {
			c.Conflicts = lines[1:]
		}
	case 129:
		// Usage error: this git predates `merge-tree --write-tree` (2.38). Conflicts just can't be predicted.
	default:
		// Anything else is a real failure (unrelated histories, a missing object, ...): report it rather than
		// blaming the git version.
		return c, err
	}
	return c, nil
}

// IsClean reports whether there are no uncommitted changes to tracked files. Untracked files are ignored because
// they do not get in the way of a merge or rebase.
func (r *Repo) IsClean() (bool, error) {
	out, err := r.run("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// Merge merges ref into the current branch. On conflicts the merge is aborted and a *ConflictError is returned.
func (r *Repo) Merge(ref string, mode MergeMode) error {
	args := append([]string{"merge", "--no-edit"}, mode.flags()...)
	if _, err := r.run(append(args, ref)...); err != nil {
		return r.abortOnConflict("merge", mode == MergeSquash, err)
	}

	// `merge --squash` stages the result but does not commit it.
	if mode == MergeSquash {
		if _, err := r.run("commit", "--no-edit"); err != nil {
			return err
		}
	}
	return nil
}

// Rebase rebases the current branch onto onto. On conflicts the rebase is aborted and a *ConflictError is returned.
func (r *Repo) Rebase(onto string) error {
	if _, err := r.run("rebase", onto); err != nil {
		return r.abortOnConflict("rebase", false, err)
	}
	return nil
}

// abortOnConflict inspects the repository after a failed merge/rebase. If it stopped on conflicts (or a rebase is
// left in progress) the operation is rolled back and a *ConflictError is returned; otherwise err is returned as is.
func (r *Repo) abortOnConflict(op string, squash bool, err error) error {
	files := r.conflictedFiles()
	if len(files) == 0 && !(op == "rebase" && r.rebaseInProgress()) {
		return err
	}

	var abortErr error
	switch {
	case op == "rebase":
		_, abortErr = r.run("rebase", "--abort")
	case squash:
		// A squash merge leaves no MERGE_HEAD, so --abort does not apply.
		_, abortErr = r.run("reset", "--merge")
	default:
		_, abortErr = r.run("merge", "--abort")
	}
	if abortErr != nil {
		// e.g. an index.lock held by an IDE: the repository is still mid-operation, so say so instead of
		// claiming that nothing changed.
		return fmt.Errorf("%s hit conflicts in %s, and aborting it failed (%s); run `%s` yourself",
			op, strings.Join(files, ", "), Reason(abortErr), abortHint(op, squash))
	}
	return &ConflictError{Op: op, Files: files}
}

// conflictedFiles lists paths with unresolved merge conflicts.
func (r *Repo) conflictedFiles() []string {
	out, err := r.run("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}
	return splitLines(out)
}

// rebaseInProgress reports whether git left a rebase unfinished.
func (r *Repo) rebaseInProgress() bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		out, err := r.run("rev-parse", "--git-path", name)
		if err != nil {
			continue
		}
		p := strings.TrimSpace(out)
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.Dir, p)
		}
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// abortHint is the command a user should run to roll back an operation that we failed to abort ourselves.
func abortHint(op string, squash bool) string {
	switch {
	case op == "rebase":
		return "git rebase --abort"
	case squash:
		return "git reset --merge" // a squash merge leaves no MERGE_HEAD, so --abort does not apply
	default:
		return "git merge --abort"
	}
}
