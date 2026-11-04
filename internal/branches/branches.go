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
Package branches contains the UI-independent branch logic: classifying branches (protected, merged, gone, stale)
and deleting them. The TUI, and later any non-interactive command, are thin layers on top of this package.
*/
package branches

import (
	"path/filepath"
	"slices"
	"time"

	"github.com/vhjan/git-janitor/internal/git"
)

// AlwaysProtected lists branch names that can never be deleted, in addition to the current and default branch.
var AlwaysProtected = []string{"dev"}

// Branch is a local branch together with the status flags the tool cares about.
type Branch struct {
	Name           string
	Protected      bool   // Cannot be deleted (current, default, built-in or user-protected)
	Merged         bool   // Already merged into the default branch
	Gone           bool   // Its upstream branch no longer exists on the remote
	Stale          bool   // No commits within the stale threshold
	Age            string // Human-readable age of the last commit
	LastCommitUnix int64  // Unix timestamp of the last commit (for sorting)
}

// Options controls how branches are classified.
type Options struct {
	ProtectPatterns []string  // Extra protected names/patterns (shell-style wildcards)
	StaleDays       float64   // Days without commits before a branch counts as stale
	Now             time.Time // Reference time for staleness; zero means time.Now()
}

// Load reads all local branches from the repository and classifies them.
// It does not touch the network; call repo.Fetch first if remote state matters.
func Load(repo *git.Repo, opts Options) ([]Branch, error) {
	current := repo.CurrentBranch()
	defaultBranch := repo.DefaultBranch()
	merged := repo.MergedBranches(defaultBranch)

	raw, err := repo.LocalBranches()
	if err != nil {
		return nil, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := make([]Branch, 0, len(raw))
	for _, b := range raw {
		_, isMerged := merged[b.Name]
		result = append(result, Branch{
			Name:           b.Name,
			Protected:      IsProtected(b.Name, current, defaultBranch, opts.ProtectPatterns),
			Merged:         isMerged,
			Gone:           b.UpstreamGone(),
			Stale:          IsStale(b.CommitUnix, now, opts.StaleDays),
			Age:            b.CommitRelative,
			LastCommitUnix: b.CommitUnix,
		})
	}
	return result, nil
}

// IsProtected reports whether a branch must never be deleted: the current branch, the default branch, the
// built-in AlwaysProtected names, or anything matching a user-provided pattern (e.g. "release-*").
func IsProtected(name, current, defaultBranch string, patterns []string) bool {
	if name == current || name == defaultBranch {
		return true
	}
	if slices.Contains(AlwaysProtected, name) {
		return true
	}
	for _, p := range patterns {
		if matched, err := filepath.Match(p, name); err == nil && matched {
			return true
		}
	}
	return false
}

// IsStale reports whether a commit timestamp is older than the given number of days. An unknown (zero)
// timestamp is never considered stale.
func IsStale(commitUnix int64, now time.Time, staleDays float64) bool {
	if commitUnix == 0 {
		return false
	}
	threshold := int64(staleDays * 24 * 60 * 60)
	return now.Unix()-commitUnix > threshold
}
