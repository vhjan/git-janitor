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
This file contains the branch-related git operations: querying branches, the default branch and merge state,
fetching, and deleting (locally and on the remote).
*/

package git

import (
	"regexp"
	"strconv"
	"strings"
)

// Fallback names for the default branch when origin/HEAD is unknown.
const (
	BranchMain   = "main"
	BranchMaster = "master"
)

// Branch is the raw information git reports about a local branch.
type Branch struct {
	Name           string // Short branch name
	UpstreamTrack  string // Output of %(upstream:track), e.g. "[ahead 1, behind 2]" or "[gone]"
	Remote         string // Remote of the configured upstream ("origin"), "." for a local upstream, "" if none
	RemoteRef      string // Full ref of the upstream on that remote (e.g. "refs/heads/feature")
	CommitRelative string // Human-readable age of the last commit (e.g. "2 weeks ago")
	CommitUnix     int64  // Unix timestamp of the last commit
	WorktreePath   string // Path of the worktree this branch is checked out in, "" if none
}

// UpstreamGone reports whether the branch tracks a remote branch that no longer exists.
func (b Branch) UpstreamGone() bool {
	return strings.Contains(b.UpstreamTrack, "[gone]")
}

// HasRemoteUpstream reports whether the branch tracks a branch on a real remote (not a local branch).
func (b Branch) HasRemoteUpstream() bool {
	return b.Remote != "" && b.Remote != "."
}

// RemoteBranch returns the upstream's branch name on the remote ("feature/x"), or "" if there is none.
func (b Branch) RemoteBranch() string {
	return strings.TrimPrefix(b.RemoteRef, "refs/heads/")
}

var (
	aheadRe  = regexp.MustCompile(`ahead (\d+)`)
	behindRe = regexp.MustCompile(`behind (\d+)`)
)

// AheadBehind returns how many commits the branch is ahead of / behind its upstream.
func (b Branch) AheadBehind() (ahead, behind int) {
	if m := aheadRe.FindStringSubmatch(b.UpstreamTrack); m != nil {
		ahead, _ = strconv.Atoi(m[1])
	}
	if m := behindRe.FindStringSubmatch(b.UpstreamTrack); m != nil {
		behind, _ = strconv.Atoi(m[1])
	}
	return ahead, behind
}

// CurrentBranch returns the checked-out branch, or "" when HEAD is detached or the lookup fails.
func (r *Repo) CurrentBranch() string {
	out, err := r.run("branch", "--show-current")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// DefaultBranch identifies the repository's primary branch. It checks origin/HEAD first, then falls back to a
// local "main", and finally assumes "master".
func (r *Repo) DefaultBranch() string {
	if out, err := r.run("rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil {
		ref := strings.TrimSpace(out)
		if parts := strings.Split(ref, "/"); len(parts) == 2 {
			return parts[1]
		}
	}

	if _, err := r.run("show-ref", "--verify", "--quiet", "refs/heads/"+BranchMain); err == nil {
		return BranchMain
	}

	return BranchMaster
}

// RefExists reports whether ref resolves to a commit.
func (r *Repo) RefExists(ref string) bool {
	_, err := r.run("rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// ShortSHA returns the abbreviated commit id of ref, or "" if it cannot be resolved. It is shown after deletions so
// a branch can be restored with `git branch <name> <sha>`.
func (r *Repo) ShortSHA(ref string) string {
	out, err := r.run("rev-parse", "--short", ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// MergedBranches returns the set of local branches already merged into target.
func (r *Repo) MergedBranches(target string) map[string]struct{} {
	merged := make(map[string]struct{})
	out, err := r.run("branch", "--format=%(refname:short)", "--merged", target)
	if err != nil {
		return merged
	}
	for _, name := range splitLines(out) {
		merged[name] = struct{}{}
	}
	return merged
}

// IsSquashMerged reports whether the net change of branch is already contained in target even though git does not
// consider branch merged, which is what a squash merge or a rebase/cherry-pick merge looks like.
//
// Two checks are made, both based on patch ids:
//   - rebase/cherry-pick merges: `git cherry` marks every commit of the branch as already present in target;
//   - squash merges: a throw-away commit holding the branch's tree on top of the merge base (one commit with the
//     whole diff of the branch) is checked the same way. The probe commit is unreferenced and is removed by git's
//     normal garbage collection.
func (r *Repo) IsSquashMerged(branch, target string) bool {
	if out, err := r.run("cherry", target, branch); err == nil {
		lines := splitLines(out)
		allPresent := len(lines) > 0
		for _, l := range lines {
			if !strings.HasPrefix(l, "-") {
				allPresent = false
				break
			}
		}
		if allPresent {
			return true
		}
	}

	baseOut, err := r.run("merge-base", target, branch)
	if err != nil {
		return false
	}
	base := strings.TrimSpace(baseOut)

	treeOut, err := r.run("rev-parse", branch+"^{tree}")
	if err != nil {
		return false
	}
	tree := strings.TrimSpace(treeOut)

	// A branch without any net change cannot have been squash-merged (it would match everything).
	if baseTree, err := r.run("rev-parse", base+"^{tree}"); err != nil || strings.TrimSpace(baseTree) == tree {
		return false
	}

	probe, err := r.run(
		"-c", "user.name=git-janitor", "-c", "user.email=git-janitor@localhost", "-c", "commit.gpgsign=false",
		"commit-tree", tree, "-p", base, "-m", "git-janitor squash probe",
	)
	if err != nil {
		return false
	}

	out, err := r.run("cherry", target, strings.TrimSpace(probe))
	if err != nil {
		return false
	}
	lines := splitLines(out)
	return len(lines) == 1 && strings.HasPrefix(lines[0], "-")
}

// UnpushedCommits counts the commits that exist only on branch: reachable from it, but neither from the default
// branch nor from any remote-tracking branch. Deleting the branch would lose exactly these commits.
func (r *Repo) UnpushedCommits(branch, defaultBranch string) int {
	out, err := r.run("rev-list", "--count", branch, "--not", defaultBranch, "--remotes")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

// Fetch updates remote information. With prune set, remote-tracking branches that no longer exist on the remote
// are removed, which makes local branches whose upstream vanished show up as "[gone]". It returns the names of
// the remote-tracking refs that were pruned.
func (r *Repo) Fetch(prune bool) (pruned []string, err error) {
	args := []string{"fetch"}
	if prune {
		args = append(args, "--prune")
	}
	_, stderr, err := r.runIO(args...)
	if err != nil {
		return nil, err
	}
	return parsePruned(stderr), nil
}

// parsePruned extracts pruned refs from `git fetch --prune` progress output, which contains lines like
// " - [deleted]         (none)     -> origin/feature".
func parsePruned(stderr string) []string {
	var pruned []string
	for _, line := range splitLines(stderr) {
		if !strings.Contains(line, "[deleted]") {
			continue
		}
		if _, ref, ok := strings.Cut(line, "->"); ok {
			pruned = append(pruned, strings.TrimSpace(ref))
		}
	}
	return pruned
}

// branchFormat is the for-each-ref format parsed by parseBranchLine. It extracts everything in a single call.
const (
	branchSeparator = "|||"
	branchFormat    = "%(refname:short)" + branchSeparator +
		"%(upstream:track)" + branchSeparator +
		"%(upstream:remotename)" + branchSeparator +
		"%(upstream:remoteref)" + branchSeparator +
		"%(committerdate:relative)" + branchSeparator +
		"%(committerdate:unix)" + branchSeparator +
		"%(worktreepath)"
	branchFieldCount = 7
)

// LocalBranches lists all local branches (refs/heads).
func (r *Repo) LocalBranches() ([]Branch, error) {
	out, err := r.run("for-each-ref", "--format="+branchFormat, "refs/heads/")
	if err != nil {
		return nil, err
	}

	var branches []Branch
	for _, line := range splitLines(out) {
		if b, ok := parseBranchLine(line); ok {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

// parseBranchLine parses one line produced with branchFormat.
func parseBranchLine(line string) (Branch, bool) {
	parts := strings.Split(line, branchSeparator)
	if len(parts) != branchFieldCount {
		return Branch{}, false
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return Branch{}, false
	}

	b := Branch{
		Name:           name,
		UpstreamTrack:  parts[1],
		Remote:         strings.TrimSpace(parts[2]),
		RemoteRef:      strings.TrimSpace(parts[3]),
		CommitRelative: parts[4],
		WorktreePath:   strings.TrimSpace(parts[6]),
	}
	if u, err := strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64); err == nil {
		b.CommitUnix = u
	}
	return b, true
}

// DeleteBranch removes a local branch. With force it uses -D (delete even if git does not consider it merged
// into HEAD or its upstream), otherwise -d.
func (r *Repo) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run("branch", flag, name)
	return err
}

// DeleteRemoteBranch deletes a branch on a remote (`git push <remote> --delete <branch>`).
func (r *Repo) DeleteRemoteBranch(remote, branch string) error {
	_, err := r.run("push", remote, "--delete", branch)
	return err
}

// Upstream returns the remote-tracking branch that local branch follows ("origin/main"), or "" if it has none.
func (r *Repo) Upstream(branch string) string {
	out, err := r.run("rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// CommitsBehind counts the commits that upstream has and branch lacks (branch..upstream).
func (r *Repo) CommitsBehind(branch, upstream string) int {
	out, err := r.run("rev-list", "--count", branch+".."+upstream)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}
