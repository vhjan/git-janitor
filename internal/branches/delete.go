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
This file implements bulk branch deletion with dry-run support.
*/

package branches

import (
	"fmt"

	"github.com/vhjan/git-janitor/internal/git"
)

// DeleteResult reports the outcome of a bulk deletion.
type DeleteResult struct {
	Deleted []string // Branches deleted (or that would be deleted in a dry run)
	Errors  []string // Human-readable failures
}

// Delete force-deletes the given branches. With dryRun set nothing is removed and every branch is reported as if
// it had been deleted.
func Delete(repo *git.Repo, names []string, dryRun bool) DeleteResult {
	var res DeleteResult
	for _, name := range names {
		if dryRun {
			res.Deleted = append(res.Deleted, name)
			continue
		}
		if err := repo.DeleteBranch(name, true); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("Failed to delete %s", name))
			continue
		}
		res.Deleted = append(res.Deleted, name)
	}
	return res
}
