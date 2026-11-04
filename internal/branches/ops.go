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
This file implements the policy around merging and rebasing the current branch: preconditions, which ref to update
from, and friendly errors. The git mechanics live in internal/git.
*/

package branches

import (
	"errors"

	"github.com/vhjan/git-janitor/internal/git"
)

// Errors returned by Merge and Rebase for conditions the user can fix.
var (
	ErrDirtyTree = errors.New("your working tree has uncommitted changes to tracked files; commit or stash them first")
	ErrDetached  = errors.New("HEAD is detached; check out a branch first")
	ErrUpToDate  = errors.New("already up to date, nothing to do")
)

// UpdateTarget returns the ref the current branch should be updated from: the remote-tracking branch the default
// branch is configured to follow (so remotes other than "origin" work), else origin's copy of it, else the local
// default branch. The remote copy is preferred because it is the freshest after a fetch.
func UpdateTarget(repo *git.Repo, defaultBranch string) string {
	if up := repo.Upstream(defaultBranch); up != "" && repo.RefExists(up) {
		return up
	}
	if remote := "origin/" + defaultBranch; repo.RefExists(remote) {
		return remote
	}
	return defaultBranch
}

// preflight verifies that an operation on the current branch can start and returns the branch name.
func preflight(repo *git.Repo) (string, error) {
	current := repo.CurrentBranch()
	if current == "" {
		return "", ErrDetached
	}
	clean, err := repo.IsClean()
	if err != nil {
		return "", err
	}
	if !clean {
		return "", ErrDirtyTree
	}
	return current, nil
}

// Merge merges ref into the current branch. A merge that hits conflicts is aborted (*git.ConflictError).
func Merge(repo *git.Repo, ref string, mode git.MergeMode) error {
	current, err := preflight(repo)
	if err != nil {
		return err
	}
	if check, err := repo.CheckMerge(current, ref); err == nil && check.UpToDate {
		return ErrUpToDate
	}
	return repo.Merge(ref, mode)
}

// Rebase rebases the current branch onto onto. A rebase that hits conflicts is aborted (*git.ConflictError).
func Rebase(repo *git.Repo, onto string) error {
	current, err := preflight(repo)
	if err != nil {
		return err
	}
	if check, err := repo.CheckMerge(current, onto); err == nil && check.UpToDate {
		return ErrUpToDate
	}
	return repo.Rebase(onto)
}
