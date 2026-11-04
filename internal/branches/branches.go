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
Package branches contains the UI-independent branch logic: classifying branches (protected, merged, gone, stale),
deleting them, and the merge/rebase policy on top of the git layer. The TUI, and later any non-interactive command,
are thin layers on top of this package.
*/
package branches

import (
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/vhjan/git-janitor/internal/git"
)

// AlwaysProtected lists branch names that can never be deleted, in addition to the current and default branch.
var AlwaysProtected = []string{"dev"}

// maxParallel bounds how many git processes are used at once for the per-branch checks.
const maxParallel = 8

// Branch is a local branch together with the status flags the tool cares about.
type Branch struct {
	Name            string
	Protected       bool   // Cannot be deleted (current, default, built-in, user-protected or checked out elsewhere)
	ProtectedReason string // Why it is protected, empty when it is not
	Merged          bool   // Git considers it merged into the default branch
	Squashed        bool   // Not merged according to git, but its changes are already in the default branch
	Gone            bool   // Its upstream branch no longer exists on the remote
	Stale           bool   // No commits within the stale threshold
	Ahead           int    // Commits ahead of its upstream
	Behind          int    // Commits behind its upstream
	Unpushed        int    // Commits that exist only on this branch (not on the default branch nor on any remote)
	Remote          string // Remote of its upstream ("origin"), "" when it has none
	RemoteBranch    string // Name of the upstream branch on that remote
	Worktree        string // Path of another worktree that has this branch checked out
	Age             string // Human-readable age of the last commit
	LastCommitUnix  int64  // Unix timestamp of the last commit (for sorting)
}

// FullyMerged reports whether the branch's work is already in the default branch, by true merge or by squash.
func (b Branch) FullyMerged() bool { return b.Merged || b.Squashed }

// AtRisk reports whether deleting the branch would lose commits that exist nowhere else.
func (b Branch) AtRisk() bool { return !b.Protected && !b.FullyMerged() && b.Unpushed > 0 }

// RemoteDeletable reports whether the branch has a live upstream on a remote that could be deleted too. A remote
// that is ahead of the local branch (Behind > 0) holds commits we do not have, so it is never offered.
func (b Branch) RemoteDeletable() bool {
	return b.Remote != "" && b.RemoteBranch != "" && !b.Gone && b.Behind == 0
}

// Options controls how branches are classified.
type Options struct {
	ProtectPatterns []string  // Extra protected names/patterns (shell-style wildcards)
	StaleDays       float64   // Days without commits before a branch counts as stale
	Now             time.Time // Reference time for staleness; zero means time.Now()
}

// Snapshot is the state of the repository's local branches at one point in time.
type Snapshot struct {
	Branches []Branch
	Current  string // Checked-out branch, "" when HEAD is detached
	Default  string // The repository's default branch

	// DefaultUpstream is the remote-tracking branch the default branch follows ("origin/main"), "" if none. When
	// DefaultBehind > 0 the local default branch lacks commits the remote has, so merges done on the remote (e.g.
	// squash-merged PRs) are not visible to the "merged" checks, which compare against the local default branch.
	DefaultUpstream string
	DefaultBehind   int
}

// Load reads all local branches from the repository and classifies them.
// It does not touch the network; call repo.Fetch first if remote state matters.
func Load(repo *git.Repo, opts Options) (Snapshot, error) {
	snap := Snapshot{
		Current: repo.CurrentBranch(),
		Default: repo.DefaultBranch(),
	}
	if up := repo.Upstream(snap.Default); up != "" && repo.RefExists(up) {
		snap.DefaultUpstream = up
		snap.DefaultBehind = repo.CommitsBehind(snap.Default, up)
	}
	merged := repo.MergedBranches(snap.Default)

	raw, err := repo.LocalBranches()
	if err != nil {
		return snap, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := make([]Branch, len(raw))
	for i, b := range raw {
		_, isMerged := merged[b.Name]
		ahead, behind := b.AheadBehind()

		result[i] = Branch{
			Name:            b.Name,
			Merged:          isMerged,
			Gone:            b.UpstreamGone(),
			Stale:           IsStale(b.CommitUnix, now, opts.StaleDays),
			Ahead:           ahead,
			Behind:          behind,
			Age:             b.CommitRelative,
			LastCommitUnix:  b.CommitUnix,
			ProtectedReason: ProtectionReason(b.Name, snap.Current, snap.Default, opts.ProtectPatterns),
		}
		if b.HasRemoteUpstream() {
			result[i].Remote = b.Remote
			result[i].RemoteBranch = b.RemoteBranch()
		}
		if b.WorktreePath != "" && b.Name != snap.Current {
			result[i].Worktree = b.WorktreePath
			if result[i].ProtectedReason == "" {
				result[i].ProtectedReason = "checked out in worktree " + b.WorktreePath
			}
		}
		result[i].Protected = result[i].ProtectedReason != ""
	}

	// The remaining checks spawn several git processes per branch, so run them in parallel. Protected and merged
	// branches are skipped: they can't be deleted or have nothing to lose.
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)
	for i := range result {
		if result[i].Protected || result[i].Merged {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(b *Branch) {
			defer wg.Done()
			defer func() { <-sem }()

			if repo.IsSquashMerged(b.Name, snap.Default) {
				b.Squashed = true
				return
			}
			b.Unpushed = repo.UnpushedCommits(b.Name, snap.Default)
		}(&result[i])
	}
	wg.Wait()

	snap.Branches = result
	return snap, nil
}

// ProtectionReason returns why a branch must never be deleted, or "" if it may be: the current branch, the
// default branch, a built-in protected name, or a match with a user-provided pattern (e.g. "release-*").
func ProtectionReason(name, current, defaultBranch string, patterns []string) string {
	switch name {
	case current:
		return "current branch"
	case defaultBranch:
		return "default branch"
	}
	if slices.Contains(AlwaysProtected, name) {
		return "built-in protected branch"
	}
	for _, p := range patterns {
		if matched, err := filepath.Match(p, name); err == nil && matched {
			return "matches protect pattern " + p
		}
	}
	return ""
}

// IsProtected reports whether a branch must never be deleted. See ProtectionReason.
func IsProtected(name, current, defaultBranch string, patterns []string) bool {
	return ProtectionReason(name, current, defaultBranch, patterns) != ""
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
