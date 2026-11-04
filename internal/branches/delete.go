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
This file implements bulk branch deletion (local, and optionally the matching remote branch) with dry-run support.
*/

package branches

import (
	"fmt"

	"github.com/vhjan/git-janitor/internal/git"
)

// DeleteOptions controls a bulk deletion.
type DeleteOptions struct {
	DryRun       bool // Report what would happen without deleting anything
	DeleteRemote bool // Also delete the upstream branch on the remote, where there is one
}

// Deleted records one removed branch.
type Deleted struct {
	Name          string
	SHA           string // Abbreviated tip commit, so the branch can be restored with `git branch <name> <sha>`
	RemoteDeleted bool   // The upstream branch on the remote was deleted as well
}

// DeleteResult reports the outcome of a bulk deletion.
type DeleteResult struct {
	Deleted []Deleted // Branches deleted (or that would be deleted in a dry run)
	Errors  []string  // Human-readable failures
}

// Delete removes the given branches. Branches are deleted with -D because git's own "fully merged" check only looks
// at HEAD and the upstream, which would wrongly refuse branches that were merged into the default branch. Deciding
// what is safe to delete is the caller's job (see Branch.AtRisk).
//
// With opts.DeleteRemote the upstream branch is deleted after the local one succeeded.
func Delete(repo *git.Repo, targets []Branch, opts DeleteOptions) DeleteResult {
	var res DeleteResult
	for _, b := range targets {
		wantRemote := opts.DeleteRemote && b.RemoteDeletable()
		d := Deleted{Name: b.Name, SHA: repo.ShortSHA(b.Name)}

		if opts.DryRun {
			d.RemoteDeleted = wantRemote
			res.Deleted = append(res.Deleted, d)
			continue
		}

		if err := repo.DeleteBranch(b.Name, true); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to delete %s: %s", b.Name, git.Reason(err)))
			continue
		}

		if wantRemote {
			if err := repo.DeleteRemoteBranch(b.Remote, b.RemoteBranch); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("Deleted %s locally, but not %s/%s: %s",
					b.Name, b.Remote, b.RemoteBranch, git.Reason(err)))
			} else {
				d.RemoteDeleted = true
			}
		}
		res.Deleted = append(res.Deleted, d)
	}
	return res
}
